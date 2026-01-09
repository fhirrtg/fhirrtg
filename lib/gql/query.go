package gql

import (
	"fmt"
	"strings"
)

// GraphQL AST Node Types
type Node interface {
	String() string
}

type Fragment struct {
	Name   string
	Type   string
	Fields []Field
}

func (f Fragment) String() string {
	var fieldStrs []string
	for _, field := range f.Fields {
		fieldStrs = append(fieldStrs, field.String())
	}
	return fmt.Sprintf("fragment %s on %s { %s }", f.Name, f.Type, strings.Join(fieldStrs, " "))
}

// Variable represents a GraphQL variable (e.g., $id: ID!)
type Variable struct {
	Name string
	Type string
}

func (v Variable) String() string {
	return fmt.Sprintf("$%s: %s", v.Name, v.Type)
}

type ArgumentValue struct {
	Value        string
	SubArguments map[string]ArgumentValue
}
type Arguments map[string]ArgumentValue

func (a ArgumentValue) String() string {
	if len(a.SubArguments) > 0 {
		var subArgs []string
		for key, value := range a.SubArguments {
			subArgs = append(subArgs, fmt.Sprintf("%s: %s", key, value))
		}
		return fmt.Sprintf("{ %s }", strings.Join(subArgs, ", "))
	}
	if a.Value == "" {
		return "{}"
	}
	return fmt.Sprintf("%q", a.Value)
}

// Field represents a GraphQL field (e.g., "id", "name")
type Field struct {
	Name       string
	SubFields  []Field
	Alias      string
	Arguments  Arguments
	Type       string
	Kind       string
	Connection bool
	Fragments  []Fragment
}

func (f Field) String() string {
	if f.Kind == "LIST" || f.Connection {
		return f.connectionString()
	}
	return f.regularString()
}

func (f Field) regularString() string {
	var args []string
	for key, value := range f.Arguments {
		args = append(args, fmt.Sprintf("%s: %s", key, value.String()))
	}

	fieldStr := f.Name
	if f.Alias != "" {
		fieldStr = f.Alias + ": " + fieldStr
	}

	if len(args) > 0 {
		fieldStr += "(" + strings.Join(args, ", ") + ")"
	}

	if len(f.SubFields)+len(f.Fragments) > 0 {
		fieldStr += " { "
		elementStrings := []string{}

		for _, frag := range f.Fragments {
			elementStrings = append(elementStrings, "..."+frag.Name)
		}

		for _, subField := range f.SubFields {
			elementStrings = append(elementStrings, subField.String())
		}

		fieldStr += strings.Join(elementStrings, " ")
		fieldStr += " }"
	}

	return fieldStr
}

func (f Field) connectionString() string {
	fields := []Field{
		{Name: "pageInfo", SubFields: []Field{
			{Name: "hasNextPage"},
			{Name: "hasPreviousPage"},
			{Name: "startCursor"},
			{Name: "endCursor"},
		}},
		{Name: "edges", SubFields: []Field{
			{Name: "cursor"},
			{Name: "node", SubFields: f.SubFields, Fragments: f.Fragments},
		}},
	}

	connectionField := Field{
		Name:      f.Name + "Connection",
		Alias:     f.Alias,
		Arguments: f.Arguments,
		SubFields: fields,
	}

	return connectionField.regularString()
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

	args := ""
	if len(varDefs) > 0 {
		args = "(" + strings.Join(varDefs, ", ") + ")"
	}

	queryStr := fmt.Sprintf("%s %s%s { %s }", q.Operation, q.Name, args, strings.Join(fieldStrs, " "))
	return queryStr
}

type PossibleType struct {
	Name string
	Kind string
}

type SchemaType struct {
	Name          string
	Kind          string
	PossibleTypes []PossibleType
	Fields        []Field
}

// func Test() {
// 	// Construct an AST representation of the query
// 	query := Query{
// 		Operation: "query",
// 		Name:      "GetUser",
// 		Variables: []Variable{
// 			{Name: "id", Type: "ID!"},
// 			{Name: "name", Type: "String!"},
// 		},
// 		Fields: []Field{
// 			{
// 				Name: "user",
// 				Arguments: map[string]string{
// 					"id": "$id",
// 				},
// 				SubFields: []Field{
// 					{Name: "id"},
// 					{Name: "name"},
// 					{Name: "email"},
// 				},
// 			},
// 			{
// 				Name: "patient",
// 				Arguments: map[string]string{
// 					"id": "$id",
// 				},
// 				SubFields: []Field{
// 					{Name: "id"},
// 					{Name: "name"},
// 					{Name: "mrn"},
// 				},
// 			},
// 		},
// 	}

// 	// Render query to a string
// 	fmt.Println(query.String())
// }
