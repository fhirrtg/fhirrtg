package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// toGraphQLInput converts a JSON-decoded value to a GraphQL input value
// literal, using schemaDict to look up each input-object field's type so
// ENUM-typed string values render as bare identifiers (with FHIR-style
// hyphens normalized to underscores when needed).
func toGraphQLInput(v interface{}, typeName string) string {
	switch val := v.(type) {
	case map[string]interface{}:
		return inputObjectToGraphQL(val, typeName)
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = toGraphQLInput(item, typeName)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return inputScalarToGraphQL(v, typeName)
	}
}

func inputObjectToGraphQL(m map[string]interface{}, typeName string) string {
	schema, hasSchema := schemaDict[typeName]
	parts := make([]string, 0, len(m))
	for k, item := range m {
		childType := ""
		if hasSchema {
			found := false
			for _, f := range schema.InputFields {
				if f.Name == k {
					childType = f.Type
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, toGraphQLInput(item, childType)))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func inputScalarToGraphQL(v interface{}, typeName string) string {
	if s, ok := v.(string); ok && typeName != "" {
		if schema, ok := schemaDict[typeName]; ok && schema.Kind == "ENUM" {
			return resolveEnumValue(s, schema.EnumValues)
		}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// resolveEnumValue picks the introspected enum-value name that matches the
// given FHIR code, trying common normalizations (raw, hyphens-as-underscores,
// uppercased). Falls back to hyphens-as-underscores when no match is found.
func resolveEnumValue(input string, enumValues []string) string {
	candidates := []string{
		input,
		strings.ReplaceAll(input, "-", "_"),
		strings.ToUpper(strings.ReplaceAll(input, "-", "_")),
	}
	for _, c := range candidates {
		for _, ev := range enumValues {
			if ev == c {
				return ev
			}
		}
	}
	return strings.ReplaceAll(input, "-", "_")
}
