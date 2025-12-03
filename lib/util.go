package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

func GqlRequest(gql string, profile string) []byte {
	query := fmt.Sprintf(`{"query": %q}`, gql)

	url := fmt.Sprintf("%s/$graphql?_profile=%s", upstream, profile)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(query)))

	if err != nil {
		panic(err)
	}

	// Set Headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", GQL_ACCEPT_HEADER)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode >= 400 {
		fmt.Println("Error response from server:", resp.Status)
		fmt.Println(string(body))
		return body
	}

	return body
}

// getEnv retrieves the value of the environment variable named by the key
// If the variable is not present, returns the fallback value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
