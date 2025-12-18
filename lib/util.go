package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

func GqlRequest(gql string, profile string) ([]byte, error) {
	query := fmt.Sprintf(`{"query": %q}`, gql)

	url := fmt.Sprintf("%s/$graphql?_profile=%s", upstream, profile)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(query)))

	if err != nil {
		slog.Error("Error creating request:", "error", err)
		return nil, err
	}

	// Copy additional headers from request
	for name, values := range req.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Set Headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", GQL_ACCEPT_HEADER)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error sending request:", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading response body:", "error", err)
		return nil, err
	}

	if resp.StatusCode >= 400 {
		fmt.Println("Error response from server:", resp.Status)
		fmt.Println(string(body))
		return body, fmt.Errorf("error response from server: %s", resp.Status)
	}

	return body, nil
}

func ProxyRequest(w http.ResponseWriter, req *http.Request) {
	url := fmt.Sprintf("%s%s", upstream, req.URL.Path)

	proxyReq, err := http.NewRequest(req.Method, url, req.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for name, values := range req.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to send proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// getEnv retrieves the value of the environment variable named by the key
// If the variable is not present, returns the fallback value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
