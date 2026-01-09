package main

import (
	"fmt"
	"strings"

	"github.com/fhirrtg/fhirrtg/gql"
)

type IncludeParam struct {
	ResourceName  string
	FieldName     string
	TargetType    string
	PossibleTypes []string
}

func KebabToLowerCamel(s string) string {
	if !strings.Contains(s, "-") {
		return s
	}

	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			parts[i] = strings.ToLower(p)
		} else {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, "")
}

func parseIncludeParam(includeParam string) (IncludeParam, error) {
	parts := strings.Split(includeParam, ":")

	if len(parts) != 2 && len(parts) != 3 {
		return IncludeParam{}, fmt.Errorf("invalid _include|_revinclude parameter: %s, %v", includeParam, parts)
	}

	include := IncludeParam{
		ResourceName: parts[0],
		FieldName:    KebabToLowerCamel(parts[1]),
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

	log.Debug(fmt.Sprintf("Include Resource: %v\n", unionType.PossibleTypes))

	return include, nil
}

func findField(fields []gql.Field, fieldName string) gql.Field {
	for _, field := range fields {
		if field.Name == fieldName {
			return field
		}
	}

	return gql.Field{}
}
