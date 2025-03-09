package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
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

type IncludeParam struct {
	ResourceName  string
	FieldName     string
	TargetType    string
	PossibleTypes []string
}

func parseIncludeParam(includeParam string) (IncludeParam, error) {
	parts := strings.Split(includeParam, ":")

	if len(parts) != 2 && len(parts) != 3 {
		return IncludeParam{}, fmt.Errorf("invalid _include parameter: %s, %v", includeParam, parts)
	}

	include := IncludeParam{
		ResourceName: parts[0],
		FieldName:    parts[1],
	}
	if len(parts) == 3 {
		include.TargetType = parts[2]
	}

	inclResource := schemaDict[include.ResourceName]
	inclField := findField(inclResource.Fields, include.FieldName)

	referenceType := schemaDict[inclField.Type]
	refResourceType := findField(referenceType.Fields, "resource")
	unionType := schemaDict[refResourceType.Type]

	for _, possibleType := range unionType.PossibleTypes {
		include.PossibleTypes = append(include.PossibleTypes, possibleType.Name)
	}

	fmt.Printf("Include Resource: %v\n", unionType.PossibleTypes)

	return include, nil
}

func FullResourceRequest(resourceType string, includes []IncludeParam, fragments map[string]gql.Fragment) gql.Query {
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

	query := gql.Query{
		Operation: "query",
		Name:      "Get" + resourceType,
		Fields: []gql.Field{
			{
				Name:      resourceType,
				Fragments: []gql.Fragment{fragments[resourceType]},
				SubFields: subFields,
			},
		},
	}

	return query
}

func fhirSearch(w http.ResponseWriter, queryString url.Values, resourceType string) {
	includeParams := queryString["_include"]

	var includes []IncludeParam
	fragment := GenerateFragment(resourceType)
	fragments := map[string]gql.Fragment{resourceType: fragment}

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

	gqlStr := ""
	for _, fragment := range fragments {
		gqlStr += fragment.String() + "\n"
	}

	query := FullResourceRequest(resourceType, includes, fragments)
	gqlStr += query.String()

	// fmt.Println("GQL Query:")
	fmt.Println(gqlStr)

	resp := GqlRequest(gqlStr)
	w.Write(resp)
}

func parseQueryString(w http.ResponseWriter, req *http.Request) {
	// Determine if the request is a POST or GET
	switch req.Method {
	case http.MethodPost:
		fmt.Println("Request Method: POST")
	case http.MethodGet:
		fmt.Println("Request Method: GET")

		pathComponents := strings.Split(req.URL.Path, "/")
		switch len(pathComponents) {
		case 1:
			// Server Root
			fmt.Println("No path components")
		case 2:
			/// Resource Type Search
			fhirSearch(w, req.URL.Query(), pathComponents[1])
		case 3:
			// Resource Type Read
			fmt.Println("Resource Type Read")
			fmt.Println("  Type:", pathComponents[1])
			fmt.Println("  ID:", pathComponents[2])
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
	fmt.Println(`    ________  __________     ____  ____________
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
