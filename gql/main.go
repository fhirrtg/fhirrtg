package gql

import (
	"fmt"
	"strings"
)

// GraphQL AST Node Types
type Node interface {
	String() string
}

// Variable represents a GraphQL variable (e.g., $id: ID!)
type Variable struct {
	Name string
	Type string
}

func (v Variable) String() string {
	return fmt.Sprintf("$%s: %s", v.Name, v.Type)
}

// Field represents a GraphQL field (e.g., "id", "name")
type Field struct {
	Name      string
	SubFields []Field
	Alias     string
	Arguments map[string]string
	Type      string
	Kind      string
}

func (f Field) String() string {
	var args []string
	for key, value := range f.Arguments {
		args = append(args, fmt.Sprintf("%s: %s", key, value))
	}

	fieldStr := f.Name
	if f.Alias != "" {
		fieldStr = f.Alias + ": " + fieldStr
	}

	if len(args) > 0 {
		fieldStr += "(" + strings.Join(args, ", ") + ")"
	}

	if len(f.SubFields) > 0 {
		subFieldStrs := []string{}
		for _, subField := range f.SubFields {
			subFieldStrs = append(subFieldStrs, subField.String())
		}
		fieldStr += " { " + strings.Join(subFieldStrs, " ") + " }"
	}

	return fieldStr
}

// Query represents the full GraphQL query AST
type Query struct {
	Operation string
	Name      string
	Variables []Variable
	Fields    []Field
}

func (q Query) String() string {
	var varDefs []string
	for _, v := range q.Variables {
		varDefs = append(varDefs, v.String())
	}

	var fieldStrs []string
	for _, f := range q.Fields {
		fieldStrs = append(fieldStrs, f.String())
	}

	queryStr := fmt.Sprintf("%s %s(%s) { \n %s \n}", q.Operation, q.Name, strings.Join(varDefs, ", "), strings.Join(fieldStrs, "\n "))
	return queryStr
}

type SchemaType struct {
	Name   string
	Kind   string
	Fields []Field
}

func Test() {
	// Construct an AST representation of the query
	query := Query{
		Operation: "query",
		Name:      "GetUser",
		Variables: []Variable{
			{Name: "id", Type: "ID!"},
			{Name: "name", Type: "String!"},
		},
		Fields: []Field{
			{
				Name: "user",
				Arguments: map[string]string{
					"id": "$id",
				},
				SubFields: []Field{
					{Name: "id"},
					{Name: "name"},
					{Name: "email"},
				},
			},
			{
				Name: "patient",
				Arguments: map[string]string{
					"id": "$id",
				},
				SubFields: []Field{
					{Name: "id"},
					{Name: "name"},
					{Name: "mrn"},
				},
			},
		},
	}

	// Render query to a string
	fmt.Println(query.String())
}
