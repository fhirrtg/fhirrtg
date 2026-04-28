package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fhirrtg/fhirrtg/gql"
)

func fhirRead(w http.ResponseWriter, req *http.Request, resourceType string, id string) {
	ctxLog := LoggerFromRequest(req)

	queryString := req.URL.Query()
	profile := queryString.Get("_profile")
	fragment := GenerateFragment(resourceType)
	fragments := map[string]gql.Fragment{resourceType: fragment}

	query := gql.Query{
		Operation: "query",
		Name:      "Get" + resourceType,
		Fields: []gql.Field{
			{
				Name: resourceType,
				Arguments: gql.Arguments{
					"id": gql.ArgumentValue{Value: id},
				},
				Fragments: []gql.Fragment{fragments[resourceType]},
			},
		},
	}

	gqlStr := ""
	for _, fragment := range fragments {
		gqlStr += fragment.String() + "\n"
	}
	gqlStr += query.String()

	response, err := GqlRequest(gqlStr, profile, req)
	if err != nil || response == nil {
		SendError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil && body == nil {
		ctxLog.Error("Error reading response body:", "error", err)
		SendError(w, err.Error(), response.StatusCode)
		return
	}

	copyHeaders(w.Header(), response.Header)
	SendReadResult(w, body, response.StatusCode)
}

func fhirSearch(w http.ResponseWriter, req *http.Request, resourceType string) {
	ctxLog := LoggerFromRequest(req)
	queryString := req.URL.Query()
	profile := queryString.Get("profile")
	fragment := GenerateFragment(resourceType)
	fragments := map[string]gql.Fragment{resourceType: fragment}

	var includes []IncludeParam
	includeParams := queryString["_include"]
	for _, includeParam := range includeParams {
		include, err := parseIncludeParam(includeParam)
		if err != nil {
			SendError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate fragments for the possible types
		for _, possibleType := range include.PossibleTypes {
			fragments[possibleType] = GenerateFragment(possibleType)
		}
		includes = append(includes, include)
	}

	var revincludes []IncludeParam
	revincludeParams := queryString["_revinclude"]
	for _, revincludeParams := range revincludeParams {
		revinclude, err := parseIncludeParam(revincludeParams)
		if err != nil {
			SendError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate fragment for the revinclude type
		fragments[revinclude.ResourceName] = GenerateFragment(revinclude.ResourceName)
		revincludes = append(includes, revinclude)
	}

	var searchParams = make(gql.Arguments)
	for key, value := range queryString {
		if strings.HasPrefix(key, "_") && key != "_id" {
			continue
		}
		searchParams[key] = gql.ArgumentValue{Value: value[0]}
	}

	gqlStr := ""
	for _, fragment := range fragments {
		gqlStr += fragment.String() + "\n"
	}

	query := FullResourceRequest(resourceType, searchParams, includes, revincludes, fragments)
	gqlStr += query.String()

	response, err := GqlRequest(gqlStr, profile, req)
	if err != nil || response == nil {
		SendError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil && body == nil {
		ctxLog.Error("Error reading response body:", "error", err)
		SendError(w, err.Error(), response.StatusCode)
		return
	}

	copyHeaders(w.Header(), response.Header)
	SendBundle(w, body, response.StatusCode, req)
}

func fhirUpdate(w http.ResponseWriter, req *http.Request, resourceType string, id string) {
	ctxLog := LoggerFromRequest(req)

	body, err := io.ReadAll(req.Body)
	if err != nil {
		ctxLog.Error("Failed to read request body:", "error", err)
		SendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	profile := req.URL.Query().Get("_profile")
	gqlStr, err := generateUpdateMutation(resourceType, id, body)
	if err != nil {
		ctxLog.Error("Failed to generate GraphQL mutation:", "error", err)
		SendError(w, fmt.Sprintf("Invalid input: %s", err.Error()), http.StatusBadRequest)
		return
	}

	resp, err := GqlRequest(gqlStr, profile, req)
	if err != nil {
		ctxLog.Error("GraphQL request failed:", "error", err)
		SendError(w, err.Error(), resp.StatusCode)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		ctxLog.Error("Failed to read response body:", "error", err)
		SendError(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	SendReadResult(w, responseBody, resp.StatusCode)
}

func fhirCreate(w http.ResponseWriter, req *http.Request, resourceType string) {
	ctxLog := LoggerFromRequest(req)

	body, err := io.ReadAll(req.Body)
	if err != nil {
		ctxLog.Error("Failed to read request body:", "error", err)
		SendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	profile := req.URL.Query().Get("_profile")
	gqlStr, err := generateCreateMutation(resourceType, body)
	if err != nil {
		ctxLog.Error("Failed to generate GraphQL mutation:", "error", err)
		SendError(w, fmt.Sprintf("Invalid input: %s", err.Error()), http.StatusBadRequest)
		return
	}

	resp, err := GqlRequest(gqlStr, profile, req)
	if err != nil {
		ctxLog.Error("GraphQL request failed:", "error", err)
		SendError(w, err.Error(), resp.StatusCode)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		ctxLog.Error("Failed to read response body:", "error", err)
		SendError(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	SendReadResult(w, responseBody, resp.StatusCode)
}
