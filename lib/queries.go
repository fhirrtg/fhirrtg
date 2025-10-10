package main

import "github.com/fhirrtg/fhirrtg/gql"

func GenerateFragment(typeName string) gql.Fragment {
	schema := schemaDict[typeName]

	fragment := gql.Fragment{
		Name:   typeName + "Fragment",
		Type:   typeName,
		Fields: buildFieldTree(schema.Fields, 0),
	}

	return fragment
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
