package main

import (
	"encoding/json"
	"time"
)

type FhirBundle struct {
	ResourceType string      `json:"resourceType"`
	Type         string      `json:"type"`
	Total        int         `json:"total"`
	Timestamp    string      `json:"timestamp,omitempty"`
	Entries      []FhirEntry `json:"entry"`
}

type FhirEntry struct {
	FullUrl  string                 `json:"fullUrl"`
	Resource map[string]interface{} `json:"resource"`
}

func PostProcess(body []byte) []byte {
	var jsonData map[string]interface{}
	err := json.Unmarshal(body, &jsonData)
	if err != nil {
		// Return original if we can't unmarshal
		return body
	}

	// Find all "node" keys
	nodes := []interface{}{}

	var findNodes func(data map[string]interface{})
	findNodes = func(data map[string]interface{}) {
		for key, value := range data {
			if key == "node" {
				nodes = append(nodes, value)
			}
			// If key is "resource", delete that key from parent map
			if key == "resource" {
				nodes = append(nodes, value)
				delete(data, key)
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
		Total:        len(nodes),
	}

	// Convert nodes to FhirEntry objects
	for _, node := range nodes {
		nodeMap := node.(map[string]interface{})
		resourceType, _ := nodeMap["resourceType"].(string)
		id, _ := nodeMap["id"].(string)

		fullUrl := ""
		if resourceType != "" && id != "" {
			fullUrl = upstream + "/" + resourceType + "/" + id
		}

		entry := FhirEntry{
			FullUrl:  fullUrl,
			Resource: nodeMap,
		}
		bundle.Entries = append(bundle.Entries, entry)
	}

	// Marshal the bundle to JSON
	body, err = json.Marshal(bundle)
	if err != nil {
		// Return original if we can't marshal
		return body
	}
	return body
}
