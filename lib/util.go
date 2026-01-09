package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

func OperationOutcome(code string, text string, diagnostics *string) []byte {
	issue := map[string]interface{}{
		"severity": "error",
		"code":     code,
		"details": map[string]interface{}{
			"text": text,
		},
	}

	if diagnostics != nil && *diagnostics != "" {
		issue["diagnostics"] = *diagnostics
	}

	outcome := map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue": []map[string]interface{}{
			issue,
		},
	}

	// Remove empty values
	removeEmpties(outcome)

	body, err := json.Marshal(outcome)
	if err != nil {
		// Return original if we can't marshal
		return nil
	}

	return body
}

func GqlRequest(gql string, profile string, origReq *http.Request) ([]byte, int, error) {
	var ctxLog *slog.Logger
	if origReq != nil {
		ctxLog = LoggerFromContext(origReq.Context())
	} else {
		ctxLog = slog.Default()
	}

	query := fmt.Sprintf(`{"query": %q}`, gql)

	url := fmt.Sprintf("%s/$graphql?_profile=%s", upstream, profile)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(query)))

	if err != nil {
		ctxLog.Error("Error creating request:", "error", err)
		return nil, http.StatusServiceUnavailable, err
	}

	if origReq != nil {
		// Copy additional headers from request
		for name, values := range origReq.Header {
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}

		// Add client IP to headers
		clientIP := origReq.RemoteAddr
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	// Set Headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", GQL_ACCEPT_HEADER)
	// Send request
	resp, err := client.Do(req)
	if err != nil {
		ctxLog.Error("Error sending request:", "error", err)
		return nil, http.StatusServiceUnavailable, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctxLog.Error("Error reading response body:", "error", err)
		return nil, http.StatusServiceUnavailable, err
	}

	if resp.StatusCode >= 400 {
		ctxLog.Error("Error response from server:", "status", resp.Status)
		return body, resp.StatusCode, fmt.Errorf("error response from server: %s", resp.Status)
	}

	return body, resp.StatusCode, nil
}

func ProxyRequest(w http.ResponseWriter, origReq *http.Request) {
	var ctxLog *slog.Logger
	if origReq != nil {
		ctxLog = LoggerFromContext(origReq.Context())
	} else {
		ctxLog = slog.Default()
	}

	url := fmt.Sprintf("%s%s", upstream, origReq.URL.Path)

	proxyReq, err := http.NewRequest(origReq.Method, url, origReq.Body)
	if err != nil {
		SendError(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	if origReq != nil {
		// Copy headers
		for name, values := range origReq.Header {
			for _, value := range values {
				proxyReq.Header.Add(name, value)
			}
		}

		// Add client IP to headers
		clientIP := origReq.RemoteAddr
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
		ctxLog.Info("Proxying request")
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		ctxLog.Error("Error sending proxy request upstream:", "error", err)
		SendError(w, "Failed to send proxy request", http.StatusBadGateway)
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
