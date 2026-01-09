package main

import (
	"encoding/json"
	"fmt"
	"io"
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

	// Remove id if it exists
	delete(resource, "id")

	resourceBytes, err := json.Marshal(resource)
	if err != nil {
		slog.Error("Failed to marshal resource body", "error", err)
		return "", err
	}

	returnFragment := GenerateFragment(resourceType)

	primaryField := gql.Field{
		Name: fmt.Sprintf("%sCreate", resourceType),
		Arguments: gql.Arguments{
			"resource": gql.ArgumentValue{Value: string(resourceBytes)},
		},
		Fragments: []gql.Fragment{returnFragment},
		// SubFields: returnFragment.Fields,
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

func FhirCreate(w http.ResponseWriter, req *http.Request, resourceType string) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		SendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	profile := req.URL.Query().Get("_profile")
	gqlStr, err := generateCreateMutation(resourceType, body)
	if err != nil {
		SendError(w, "Failed to generate GraphQL mutation", http.StatusInternalServerError)
		return
	}

	resp, err := GqlRequest(gqlStr, profile, req)
	if err != nil {
		SendError(w, err.Error(), resp.StatusCode)
		return
	}
	// resource := ProcessCreate(resp, req)
	w.Write(body)
	// w.Write(resp) --- IGNORE ---
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
