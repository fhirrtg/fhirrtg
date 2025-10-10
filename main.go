package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/telus/fhirrtg/gql"
)

const (
	VERSION           = "0.1"
	PORT              = 8888
	GQL_ACCEPT_HEADER = "application/graphql-response+json;charset=utf-8, application/json;charset=utf-8"
)

var (
	// Upstream Server URL
	upstream string
)

func GenerateFragment(typeName string) gql.Fragment {
	schema := schemaDict[typeName]

	fragment := gql.Fragment{
		Name:   typeName + "Fragment",
		Type:   typeName,
		Fields: buildFieldTree(schema.Fields, 0),
	}

	return fragment
}

func findField(fields []gql.Field, fieldName string) gql.Field {
	for _, field := range fields {
		if field.Name == fieldName {
			return field
		}
	}

	return gql.Field{}
}

func FullResourceRequest(
	resourceType string,
	searchParams gql.Arguments,
	includes []IncludeParam,
	revincludes []IncludeParam,
	fragments map[string]gql.Fragment,
) gql.Query {

	subFields := []gql.Field{}
	for _, include := range includes {
		includeFrags := []gql.Fragment{}
		for _, possibleType := range include.PossibleTypes {
			includeFrags = append(includeFrags, fragments[possibleType])
		}
		subFields = append(subFields, gql.Field{
			Name: include.FieldName,
			SubFields: []gql.Field{
				{
					Name:      "resource",
					Fragments: includeFrags,
				},
			},
		})
	}

	fields := []gql.Field{}

	var primaryArgs gql.Arguments
	if len(searchParams) > 0 {
		primaryArgs = gql.Arguments{"search": gql.ArgumentValue{SubArguments: searchParams}}
	} else {
		primaryArgs = nil
	}

	primaryField := gql.Field{
		Name:       resourceType,
		Arguments:  primaryArgs,
		Fragments:  []gql.Fragment{fragments[resourceType]},
		SubFields:  subFields,
		Connection: true,
	}

	fields = append(fields, primaryField)

	for _, revinclude := range revincludes {
		revincludeFrag := fragments[revinclude.ResourceName]
		revField := gql.Field{
			Name:       revinclude.ResourceName,
			Arguments:  gql.Arguments{revinclude.FieldName: gql.ArgumentValue{SubArguments: searchParams}},
			Fragments:  []gql.Fragment{revincludeFrag},
			Connection: true,
		}
		fields = append(fields, revField)
	}

	query := gql.Query{
		Operation: "query",
		Name:      "Get" + resourceType,
		Fields:    fields,
	}

	return query
}

func fhirSearch(w http.ResponseWriter, req *http.Request, resourceType string) {
	queryString := req.URL.Query()
	profile := queryString.Get("_profile")
	fragment := GenerateFragment(resourceType)
	fragments := map[string]gql.Fragment{resourceType: fragment}

	var includes []IncludeParam
	includeParams := queryString["_include"]
	for _, includeParam := range includeParams {
		include, err := parseIncludeParam(includeParam)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate fragments for the possible types
		for _, possibleType := range include.PossibleTypes {
			fragments[possibleType] = GenerateFragment(possibleType)
		}
		includes = append(includes, include)
	}

	var revincludes []IncludeParam
	revincludeParams := queryString["_revinclude"]
	for _, revincludeParams := range revincludeParams {
		revinclude, err := parseIncludeParam(revincludeParams)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate fragment for the revinclude type
		fragments[revinclude.ResourceName] = GenerateFragment(revinclude.ResourceName)
		revincludes = append(includes, revinclude)
	}

	var searchParams = make(gql.Arguments)
	for key, value := range queryString {
		if strings.HasPrefix(key, "_") && key != "_id" {
			continue
		}
		searchParams[key] = gql.ArgumentValue{Value: value[0]}
	}

	gqlStr := ""
	for _, fragment := range fragments {
		gqlStr += fragment.String() + "\n"
	}

	query := FullResourceRequest(resourceType, searchParams, includes, revincludes, fragments)
	gqlStr += query.String()

	// fmt.Println("GQL Query:")
	fmt.Println(gqlStr)

	resp := GqlRequest(gqlStr, profile)
	bundle := ProcessBundle(resp, req)
	w.Write(bundle)
}

func fhirRead(w http.ResponseWriter, req *http.Request, resourceType string, id string) {
	queryString := req.URL.Query()
	profile := queryString.Get("_profile")
	fragment := GenerateFragment(resourceType)
	fragments := map[string]gql.Fragment{resourceType: fragment}

	query := gql.Query{
		Operation: "query",
		Name:      "Get" + resourceType,
		Fields: []gql.Field{
			{
				Name: resourceType,
				Arguments: gql.Arguments{
					"id": gql.ArgumentValue{Value: id},
				},
				Fragments: []gql.Fragment{fragments[resourceType]},
			},
		},
	}

	gqlStr := ""
	for _, fragment := range fragments {
		gqlStr += fragment.String() + "\n"
	}
	gqlStr += query.String()

	fmt.Println("GQL Query:")
	fmt.Println(gqlStr)

	resp := GqlRequest(gqlStr, profile)
	resource := ProcessRead(resp, req)
	w.Write(resource)
}

func parseQueryString(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		fmt.Println("Request Method: POST")
		pathComponents := strings.Split(req.URL.Path, "/")

		switch len(pathComponents) {
		case 1:
			// Server Root
			fmt.Println("No path components")
		case 2:
			/// Create Resource
			fmt.Println("Create Resource")
			fmt.Println("  Type: ", pathComponents[1])
			FhirCreate(w, req, pathComponents[1])
		case 3:
			// Update Resource
		default:
			fmt.Println("Bad Request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
		}

		// Handle POST request
	case http.MethodGet:
		fmt.Println("Request Method: GET")

		pathComponents := strings.Split(req.URL.Path, "/")
		switch len(pathComponents) {
		case 1:
			// Server Root
			fmt.Println("No path components")
		case 2:
			/// Resource Type Search
			fhirSearch(w, req, pathComponents[1])
		case 3:
			// Resource Type Read
			fhirRead(w, req, pathComponents[1], pathComponents[2])
		case 4:
			// Compartment Search
			fmt.Println("Compartment Search")
			fmt.Println("  Component: ", pathComponents[1])
			fmt.Println("  ID: ", pathComponents[2])
			fmt.Println("  Type: ", pathComponents[3])
		default:
			fmt.Println("Bad Request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
		}

	default:
		fmt.Println("Request Method: Other")
	}

	fmt.Println("-----------")
	fmt.Println("")
}

func main() {
	fmt.Println(`
    ________  __________     ____  ____________
   / ____/ / / /  _/ __ \   / __ \/_  __/ ____/
  / /_  / /_/ // // /_/ /  / /_/ / / / / / __  
 / __/ / __  // // _, _/  / _, _/ / / / /_/ /  
/_/   /_/ /_/___/_/ |_|  /_/ |_| /_/  \____/   
                                               `)
	fmt.Printf("FHIR RTG Server Version %s\n", VERSION)
	fmt.Printf("Listening on port %d\n", PORT)

	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("No upstream server specified")
		os.Exit(1)
	}
	upstream = args[0]
	fmt.Printf("Upstream server: %s\n", upstream)

	introspect()

	// fmt.Println("-----------\n Reqeust:")
	// patientFragment := GenerateFragment("Patient")
	// encounterFragment := GenerateFragment("Encounter")
	// q := FullResourceRequest(encounterFragment)
	// fmt.Print(patientFragment.String())
	// fmt.Print(encounterFragment.String())
	// fmt.Println(q.String())

	http.HandleFunc("/", parseQueryString)
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}
