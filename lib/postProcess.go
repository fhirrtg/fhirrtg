package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type FhirBundle struct {
	ResourceType string      `json:"resourceType"`
	Type         string      `json:"type"`
	Total        int         `json:"total"`
	Timestamp    string      `json:"timestamp,omitempty"`
	Links        []FhirLink  `json:"link,omitempty"`
	Entries      []FhirEntry `json:"entry"`
}

type FhirEntry struct {
	FullUrl  string                 `json:"fullUrl"`
	Search   *FhirEntrySearch       `json:"search,omitempty"`
	Resource map[string]interface{} `json:"resource"`
}

type FhirEntrySearch struct {
	Mode  string `json:"mode"`
	Score string `json:"score,omitempty"`
}

type FhirLink struct {
	Relation string `json:"relation"`
	Url      string `json:"url"`
}

func createEntry(resource map[string]interface{}, req *http.Request, searchType string) FhirEntry {
	entryMap := resource

	resourceType, _ := entryMap["resourceType"].(string)
	id, _ := entryMap["id"].(string)

	fullUrl := ""
	if resourceType != "" && id != "" {
		fullUrl = fullHost(req) + "/" + resourceType + "/" + id
	}
	entry := FhirEntry{
		Resource: entryMap,
		FullUrl:  fullUrl,
		Search: &FhirEntrySearch{
			Mode: searchType,
		},
	}
	return entry
}

func SendBundle(w http.ResponseWriter, body []byte, req *http.Request) {
	var jsonData map[string]interface{}
	err := json.Unmarshal(body, &jsonData)
	if err != nil {
		// Return original if we can't unmarshal
		w.Write(body)
		return
	}

	// Check if there is an error key and return the original body if it exists
	if errorVal, hasError := jsonData["errors"]; hasError && errorVal != nil {
		SendOperationOutcome(w, jsonData, req)
		return
	}

	// Find all "node" keys
	entries := []FhirEntry{}

	var findNodes func(data map[string]interface{})
	findNodes = func(data map[string]interface{}) {
		for key, value := range data {

			switch key {
			case "node":
				entry := createEntry(value.(map[string]interface{}), req, "match")
				entries = append(entries, entry)

			case "resource":
				entry := createEntry(value.(map[string]interface{}), req, "include")
				entries = append(entries, entry)
			}

			// Recursively search nested maps
			switch v := value.(type) {
			case map[string]interface{}:
				findNodes(v)
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						findNodes(m)
					}
				}
			}
		}
	}

	findNodes(jsonData)

	bundle := FhirBundle{
		ResourceType: "Bundle",
		Type:         "searchset",
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Total:        len(entries),
		Entries:      entries,
	}

	// Create links for the bundle
	bundle.Links = []FhirLink{
		{
			Relation: "self",
			Url:      req.URL.String(),
		},
	}

	// Remove empty values
	removeEmpties(bundle)

	body, err = json.Marshal(bundle)
	if err != nil {
		// Return original if we can't marshal
		w.Write(body)
		return
	}

	w.Write(body)
}

func SendOperationOutcome(w http.ResponseWriter, result map[string]interface{}, req *http.Request) {
	// Stringify the errors
	err_str, err := json.Marshal(result["errors"])
	if err != nil {
		SendError(w, "There was an error processing the request", http.StatusInternalServerError)
		return
	}

	// Safely extract error message with nil checks
	errorText := "Unknown error"
	if errors, ok := result["errors"].([]any); ok && len(errors) > 0 {
		if firstError, ok := errors[0].(map[string]any); ok {
			if msg, ok := firstError["message"].(string); ok {
				errorText = msg
			}
		}
	}

	// Safely extract error code from extensions
	errorCode := "exception"
	if errors, ok := result["errors"].([]any); ok && len(errors) > 0 {
		if firstError, ok := errors[0].(map[string]any); ok {
			if extensions, ok := firstError["extensions"].(map[string]any); ok {
				if code, ok := extensions["code"].(string); ok {
					errorCode = code
				}
			}
		}
	}

	errStr := string(err_str)
	body := OperationOutcome(errorCode, errorText, &errStr)

	// Set appropriate HTTP status code if errorCode is a 3-digit number
	if len(errorCode) == 3 {
		if statusCode, err := strconv.Atoi(errorCode); err == nil && statusCode >= 100 && statusCode <= 599 {
			w.WriteHeader(statusCode)
		}
	}
	w.Write(body)
}

func SendRead(w http.ResponseWriter, body []byte, req *http.Request) {
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		// Return original if we can't unmarshal
		w.Write(body)
		return
	}

	// Check if there is an error key and return the original body if it exists
	if errorVal, hasError := result["errors"]; hasError && errorVal != nil {
		SendOperationOutcome(w, result, req)
		return
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
		w.Write(body)
		return
	}

	// Remove empty values
	removeEmpties(resource)

	// Marshal the resource into JSON and return it
	resourceBody, err := json.Marshal(resource)
	if err != nil {
		// Return original if we can't marshal
		w.Write(body)
		return
	}

	w.Write(resourceBody)
}

func fullHost(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	return scheme + "://" + req.Host
}

func removeEmpties(v interface{}) {
	switch data := v.(type) {
	case map[string]interface{}:
		for key, value := range data {
			if value == nil {
				delete(data, key)
			} else if arr, ok := value.([]interface{}); ok && len(arr) == 0 {
				delete(data, key)
			} else if key == "resource" {
				delete(data, key)
			} else {
				removeEmpties(value)
			}
		}
	case []interface{}:
		for _, item := range data {
			removeEmpties(item)
		}
	case FhirBundle:
		// Process each entry in the bundle
		for i := range data.Entries {
			removeEmpties(data.Entries[i].Resource)
		}
	}
}
