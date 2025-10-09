package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
)

var (
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

func GqlRequest(gql string, profile string) []byte {
	query := fmt.Sprintf(`{"query": %q}`, gql)

	url := fmt.Sprintf("%s?_profile=%s", upstream, profile)

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
