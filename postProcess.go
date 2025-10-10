package main

import (
	"encoding/json"
	"net/http"
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

func PostProcess(body []byte, req *http.Request) []byte {
	var jsonData map[string]interface{}
	err := json.Unmarshal(body, &jsonData)
	if err != nil {
		// Return original if we can't unmarshal
		return body
	}

	// Check if there is an error key and return the original body if it exists
	if errorVal, hasError := jsonData["errors"]; hasError && errorVal != nil {
		return body
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
		return body
	}

	return body
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
