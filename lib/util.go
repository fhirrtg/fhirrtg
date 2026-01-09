package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
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

func GqlRequest(gql string, profile string, origReq *http.Request) (*http.Response, error) {
	ctxLog := LoggerFromRequest(origReq)

	if LOG_LEVEL < 0 {
		fmt.Printf("---------\n%s\n---------\n", gql)
	}

	query := fmt.Sprintf(`{"query": %q}`, gql)

	url := fmt.Sprintf("%s/$graphql?_profile=%s", upstream, profile)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(query)))

	if err != nil {
		ctxLog.Error("Error creating request:", "error", err)
		return nil, err
	}

	if origReq != nil {
		copyHeaders(req.Header, origReq.Header)
		addForwardedFor(req)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", GQL_ACCEPT_HEADER)
	resp, err := client.Do(req)
	if err != nil {
		ctxLog.Error("Error sending request:", "error", err)
		return resp, err
	}

	return resp, nil
}

func ProxyRequest(w http.ResponseWriter, origReq *http.Request) {
	ctxLog := LoggerFromRequest(origReq)

	url := fmt.Sprintf("%s%s", upstream, origReq.URL.Path)

	proxyReq, err := http.NewRequest(origReq.Method, url, origReq.Body)
	if err != nil {
		SendError(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	if origReq != nil {
		ctxLog.Info("Proxying request")
		copyHeaders(proxyReq.Header, origReq.Header)
		addForwardedFor(proxyReq)
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		ctxLog.Error("Error sending proxy request upstream:", "error", err)
		SendError(w, "Failed to send proxy request", http.StatusServiceUnavailable)
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

// Extract client IP from X-Forwarded-For or RemoteAddr
func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		ips := strings.Split(xf, ",")
		return strings.TrimSpace(ips[0]) // first is original client
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func addForwardedFor(req *http.Request) {
	orig := req.Header.Get("X-Forwarded-For")
	ip := req.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if orig == "" {
		req.Header.Set("X-Forwarded-For", ip)
	} else {
		req.Header.Set("X-Forwarded-For", orig+", "+ip)
	}
}

func copyHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}
