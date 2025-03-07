package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	PORT              = 8888
	GQL_ACCEPT_HEADER = "application/graphql-response+json;charset=utf-8, application/json;charset=utf-8"
)

var (
	// Upstream Server URL
	upstream string
)

func introspect() {
	query := /* GraphQL */ `{
		"query": "{
			__schema {
				types {
					name
					kind
					fields {
						name
						type {
							name
							kind
							ofType {
								name
								kind
								ofType {
									name
									kind
									ofType {
										name
										kind
									}
								}
							}
						}
					}
				}
			}
		}"
	}`
	// resp, err := http.Post(upstream, GQL_ACCEPT_HEADER, bytes.NewBuffer([]byte(query)))
	req, err := http.NewRequest("POST", upstream, bytes.NewBuffer([]byte(query)))

	if err != nil {
		panic(err)
	}

	// Set Headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		panic(err)
	}
	defer resp.Body.Close()

	// Print the response
	fmt.Println("Response:")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	buildFieldDict(body)

	// json, err := json.MarshalIndent(result, "", "  ")
	// if err != nil {
	// 	fmt.Println("Error formatting JSON:", err)
	// 	return
	// }
	// fmt.Println(string(json))

}

type SchemaResponse struct {
	Data SchemaData `json:"data"`
}

type SchemaData struct {
	Schema Schema `json:"__schema"`
}

type Schema struct {
	Types []GraphQLType `json:"types"`
}

type GraphQLType struct {
	Name   string         `json:"name"`
	Kind   string         `json:"kind"`
	Fields []GraphQLField `json:"fields"`
}

type GraphQLField struct {
	Name string         `json:"name"`
	Type GraphQLTypeDef `json:"type"`
}

type GraphQLTypeDef struct {
	Name   string          `json:"name"`
	Kind   string          `json:"kind"`
	OfType *GraphQLTypeDef `json:"ofType,omitempty"`
}

func buildFieldDict(response []byte) {
	var schema SchemaResponse
	err := json.Unmarshal([]byte(response), &schema)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Iterate and print types
	for _, typ := range schema.Data.Schema.Types {
		if strings.HasPrefix(typ.Name, "__") {
			continue
		}

		fmt.Printf("Type: %s (Kind: %s)\n", typ.Name, typ.Kind)
		if typ.Fields != nil {
			fmt.Println("  Fields:")
			for _, field := range typ.Fields {
				fmt.Printf("    - Name: %s\n", field.Name)

				fieldType, fieldKind := getFieldType(field.Type)
				fmt.Printf("      FieldType: %v (%s)\n", fieldType, fieldKind)
			}
		}
		fmt.Println()
	}
}

func getFieldType(typeDef GraphQLTypeDef) (string, string) {
	if typeDef.Name != "" {
		return typeDef.Name, typeDef.Kind
	}
	if typeDef.OfType != nil && typeDef.OfType.Name != "" {
		return typeDef.OfType.Name, typeDef.OfType.Kind
	}
	if typeDef.OfType == nil {
		return "", ""
	}
	return getFieldType(*typeDef.OfType)
}

func FhirSearch(w http.ResponseWriter, queryString url.Values, resourceType string, resourceId string) {

	// Print the query string
	fmt.Print("Query String:\n")

	for key, values := range queryString {
		for _, value := range values {
			fmt.Fprintf(w, "%s: %s\n", key, value)
			fmt.Printf("%s: %s\n", key, value)
		}
	}
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
			fmt.Println("Resource Type Search")
			fmt.Println("  Type: ", pathComponents[1])
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
	fmt.Println("FHIR RTG Server Version 0.1")
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

	http.HandleFunc("/", parseQueryString)
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}
