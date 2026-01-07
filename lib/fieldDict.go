package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fhirrtg/fhirrtg/gql"
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
	Types []IntrospectionType `json:"types"`
}

type IntrospectionPossibleType struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type IntrospectionType struct {
	Name          string                      `json:"name"`
	Kind          string                      `json:"kind"`
	PossibleTypes []IntrospectionPossibleType `json:"possibleTypes"`
	Fields        []IntrospectionField        `json:"fields"`
}

type IntrospectionField struct {
	Name string                    `json:"name"`
	Type IntrospectionFieldTypeDef `json:"type"`
}

type IntrospectionFieldTypeDef struct {
	Name   string                     `json:"name"`
	Kind   string                     `json:"kind"`
	OfType *IntrospectionFieldTypeDef `json:"ofType,omitempty"`
}

func introspect() error {
	query := /* GraphQL */ `
		{
			__schema {
				types {
					name
					kind
					possibleTypes {
						name
						kind
					}
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
		}
	`

	body, err := GqlRequest(query, "", nil)
	if err != nil {
		return err
	}
	fd, err := buildFieldDict(body)
	if err != nil {
		return err
	}

	// Print the field dictionary
	debugStr := "Field Dictionary:"

	for key, value := range fd {
		debugStr += fmt.Sprintf("%s [%s]\n", key, value.Kind)
		if value.PossibleTypes != nil {
			names := []string{}
			for _, pt := range value.PossibleTypes {
				names = append(names, pt.Name)
			}
			debugStr += fmt.Sprintf("    ((%s))\n", strings.Join(names, ", "))
		}
		for _, field := range value.Fields {
			debugStr += fmt.Sprintf("   %s (%s|%s)\n", field.Name, field.Type, field.Kind)
		}
	}
	if LOG_LEVEL < 0 {
		fmt.Println(debugStr)
	}

	return nil
}

func buildFieldDict(response []byte) (map[string]gql.SchemaType, error) {
	var introspection IntrospectionResponse
	err := json.Unmarshal([]byte(response), &introspection)
	if err != nil {
		slog.Error("Failed to unmarshal introspection response", "error", err)
		return nil, err
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
			Name:          typ.Name,
			Kind:          typ.Kind,
			PossibleTypes: convertPossibleTypes(typ.PossibleTypes),
			Fields:        fields,
		}
	}

	return schemaDict, nil
}

func getFieldType(typeDef IntrospectionFieldTypeDef) (string, string) {
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

func convertPossibleTypes(possibleTypes []IntrospectionPossibleType) []gql.PossibleType {
	var gqlPossibleTypes []gql.PossibleType
	for _, pt := range possibleTypes {
		gqlPossibleTypes = append(gqlPossibleTypes, gql.PossibleType{
			Name: pt.Name,
			Kind: pt.Kind,
		})
	}
	return gqlPossibleTypes
}

func buildFieldTree(fields []gql.Field, level int) []gql.Field {
	outFields := []gql.Field{}
	for _, field := range fields {
		outField := gql.Field{
			Name: field.Name,
			Type: field.Type,
			Kind: field.Kind,
		}
		if field.Kind == "OBJECT" && level < GQL_DEPTH_LIMIT {
			schema := schemaDict[field.Type]
			outField.SubFields = buildFieldTree(schema.Fields, level+1)
			outFields = append(outFields, outField)
		}
		if field.Kind == "SCALAR" || field.Kind == "ENUM" || field.Kind == "LIST" {
			outFields = append(outFields, outField)
		}
	}
	return outFields
}
