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

func FullResourceRequest(fragment gql.Fragment) gql.Query {
	fmt.Printf("Resource Request: %s\n", fragment.Type)

	// Get the schema for the resource
	// schema := schemaDict[fragment.Type]

	query := gql.Query{
		Operation: "query",
		Name:      "Get" + fragment.Type,
		Fields: []gql.Field{
			{
				Name:     fragment.Type,
				Fragment: fragment,
			},
		},
	}

	return query
}

func fhirSearch(w http.ResponseWriter, queryString url.Values, resourceType string) {
	fragment := GenerateFragment(resourceType)
	query := FullResourceRequest(fragment)

	gqlStr := fragment.String() + "\n" + query.String()

	// fmt.Println("GQL Query:")
	// fmt.Println(gqlStr)

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
