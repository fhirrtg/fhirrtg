package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/telus/fhirrtg/gql"
)

const (
	GQL_DEPTH_LIMIT = 3
)

var (
	schemaDict map[string]gql.SchemaType
)

type IntrospectionResponse struct {
	Data IntrospectionData `json:"data"`
}

type IntrospectionData struct {
	Schema IntrospectionSchema `json:"__schema"`
}

type IntrospectionSchema struct {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fd := buildFieldDict(body)

	// Print the field dictionary
	fmt.Println("Field Dictionary:")
	for key, value := range fd {
		fmt.Printf("%s\n", key)
		for _, field := range value.Fields {
			fmt.Printf("   %s (%s)\n", field.Name, field.Type)
		}
	}

}

func buildFieldDict(response []byte) map[string]gql.SchemaType {
	var introspection IntrospectionResponse
	err := json.Unmarshal([]byte(response), &introspection)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		panic(err)
	}

	schemaDict = make(map[string]gql.SchemaType)

	for _, typ := range introspection.Data.Schema.Types {
		if strings.HasPrefix(typ.Name, "__") {
			continue
		}

		fields := []gql.Field{}
		if typ.Fields != nil {
			for _, field := range typ.Fields {

				fieldType, fieldKind := getFieldType(field.Type)

				fields = append(fields, gql.Field{
					Name: field.Name,
					Type: fieldType,
					Kind: fieldKind,
				})
			}
		}
		schemaDict[typ.Name] = gql.SchemaType{
			Name:   typ.Name,
			Kind:   typ.Kind,
			Fields: fields,
		}
		fmt.Println()
	}

	return schemaDict
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

func recurseFields(fields []gql.Field, level int) []gql.Field {
	outFields := []gql.Field{}
	for _, field := range fields {
		outField := gql.Field{
			Name: field.Name,
			Type: field.Type,
			Kind: field.Kind,
		}
		if field.Kind == "OBJECT" && level < GQL_DEPTH_LIMIT {
			schema := schemaDict[field.Type]
			fmt.Println("Field:", field.Type, schema.Name, schema.Kind)
			outField.SubFields = recurseFields(schema.Fields, level+1)
			outFields = append(outFields, outField)
		}
		if field.Kind != "OBJECT" {
			outFields = append(outFields, outField)
		}
	}
	return outFields
}

func FullResourceRequest(resourceName string) {
	fmt.Printf("Resource Request: %s\n", resourceName)

	// Get the schema for the resource
	schema := schemaDict[resourceName]

	query := gql.Query{
		Operation: "query",
		Name:      "Get" + resourceName,
		Fields: []gql.Field{
			{
				Name:      resourceName,
				SubFields: recurseFields(schema.Fields, 0),
			},
		},
	}

	fmt.Println(query.String())
}
