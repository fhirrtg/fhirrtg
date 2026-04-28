package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fhirrtg/fhirrtg/gql"
)

func generateCreateMutation(resourceType string, body []byte) (string, error) {
	var resource map[string]interface{}
	err := json.Unmarshal(body, &resource)
	if err != nil {
		slog.Error("Failed to unmarshal resource body", "error", err)
		return "", err
	}

	delete(resource, "id")

	returnFragment := GenerateFragment(resourceType)

	primaryField := gql.Field{
		Name: fmt.Sprintf("%sCreate", resourceType),
		Arguments: gql.Arguments{
			"res": gql.ArgumentValue{Value: toGraphQLInput(resource, resourceType+"Input"), Raw: true},
		},
		Fragments: []gql.Fragment{returnFragment},
	}

	gqlStr := returnFragment.String() + "\n"

	mutation := gql.Query{
		Operation: "mutation",
		Name:      fmt.Sprintf("%sCreateMutation", resourceType),
		Fields:    []gql.Field{primaryField},
	}
	gqlStr += mutation.String()
	return gqlStr, nil
}

func generateUpdateMutation(resourceType string, id string, body []byte) (string, error) {
	var resource map[string]interface{}
	err := json.Unmarshal(body, &resource)
	if err != nil {
		slog.Error("Failed to unmarshal resource body", "error", err)
		return "", err
	}

	if bodyID, ok := resource["id"].(string); ok && bodyID != "" && bodyID != id {
		return "", fmt.Errorf("resource id %q in body does not match id %q in URL", bodyID, id)
	}
	delete(resource, "id")

	returnFragment := GenerateFragment(resourceType)

	primaryField := gql.Field{
		Name: fmt.Sprintf("%sUpdate", resourceType),
		Arguments: gql.Arguments{
			"id":  gql.ArgumentValue{Value: id},
			"res": gql.ArgumentValue{Value: toGraphQLInput(resource, resourceType+"Input"), Raw: true},
		},
		Fragments: []gql.Fragment{returnFragment},
	}

	gqlStr := returnFragment.String() + "\n"

	mutation := gql.Query{
		Operation: "mutation",
		Name:      fmt.Sprintf("%sUpdateMutation", resourceType),
		Fields:    []gql.Field{primaryField},
	}
	gqlStr += mutation.String()
	return gqlStr, nil
}

func ProcessCreate(body []byte, req *http.Request) []byte {
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		// Return original if we can't unmarshal
		return body
	}

	// Check if there is an error key and return the original body if it exists
	if errorVal, hasError := result["errors"]; hasError && errorVal != nil {
		return body
	}

	var resource map[string]interface{}

	// Extract the resource from data.[resourceType] structure
	if data, ok := result["data"].(map[string]interface{}); ok {
		// get the first key in data
		for _, v := range data {
			if res, ok := v.(map[string]interface{}); ok {
				resource = res
				break
			}
		}
	}

	if resource == nil {
		// Return original if we couldn't find the resource
		return body
	}

	bundle := map[string]interface{}{
		"resourceType": "Bundle",
		"type":         "transaction-response",
		"entry": []interface{}{
			map[string]interface{}{
				"resource": resource,
				"response": map[string]interface{}{
					"status":   "201 Created",
					"location": fmt.Sprintf("%s/%s", resource["resourceType"], resource["id"]),
				},
			},
		},
		"meta": map[string]interface{}{
			"lastUpdated": time.Now().Format(time.RFC3339),
		},
		"link": []interface{}{
			map[string]interface{}{
				"relation": "self",
				"url":      req.URL.String(),
			},
		},
	}

	// Remove empty values
	removeEmpties(bundle)

	body, err = json.Marshal(bundle)
	if err != nil {
		// Return original if we can't marshal
		return body
	}

	return body
}
