package main

import (
	"fmt"
	"strings"
)

type IncludeParam struct {
	ResourceName  string
	FieldName     string
	TargetType    string
	PossibleTypes []string
}

func parseIncludeParam(includeParam string) (IncludeParam, error) {
	parts := strings.Split(includeParam, ":")

	if len(parts) != 2 && len(parts) != 3 {
		return IncludeParam{}, fmt.Errorf("invalid _include|_revinclude parameter: %s, %v", includeParam, parts)
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

	log.Debug(fmt.Sprintf("Include Resource: %v\n", unionType.PossibleTypes))

	return include, nil
}
