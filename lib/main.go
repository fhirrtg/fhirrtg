package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fhirrtg/fhirrtg/gql"
)

const (
	VERSION                   = "0.1"
	DEFAULT_PORT              = 8888
	DEFAULT_GQL_ACCEPT_HEADER = "application/graphql-response+json;charset=utf-8, application/json;charset=utf-8"
)

var (
	HEALTHCHECK_PATH  = "/health"
	PORT              int
	GQL_ACCEPT_HEADER string
	LOG_LEVEL         slog.Level
)

var (
	client   *http.Client
	log      *slog.Logger
	upstream string
)

func init() {
	logLevelStr := getEnv("RTG_LOG_LEVEL", "info")
	switch strings.ToLower(logLevelStr) {
	case "debug":
		LOG_LEVEL = slog.LevelDebug
	case "info":
		LOG_LEVEL = slog.LevelInfo
	case "warn":
		LOG_LEVEL = slog.LevelWarn
	case "error":
		LOG_LEVEL = slog.LevelError
	default:
		LOG_LEVEL = slog.LevelInfo
		fmt.Printf("Invalid log level: %s, using default: info\n", logLevelStr)
	}
	log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: LOG_LEVEL}))

	portStr := getEnv("RTG_PORT", strconv.Itoa(DEFAULT_PORT))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Printf("Invalid port number: %s, using default: %d\n", portStr, DEFAULT_PORT)
		PORT = DEFAULT_PORT
	} else {
		PORT = port
	}

	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		upstream = getEnv("RTG_UPSTREAM_SERVER", "")
	} else {
		upstream = args[0]
	}
	if upstream == "" {
		fmt.Println("No upstream server specified")
		os.Exit(1)
	}

	GQL_ACCEPT_HEADER = getEnv("RTG_GQL_ACCEPT_HEADER", DEFAULT_GQL_ACCEPT_HEADER)
	HEALTHCHECK_PATH = getEnv("RTG_HEALTHCHECK_PATH", HEALTHCHECK_PATH)

	// HTTP Client Setup
	skipTlsVerify := getEnv("RTG_SKIP_TLS_VERIFY", "false") == "true"
	timeoutStr := getEnv("RTG_GRAPHQL_TIMEOUT", "30")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		fmt.Printf("Invalid timeout value: %s, using default: 30\n", timeoutStr)
		timeout = 30
	}

	client = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipTlsVerify,
			},
		},
	}
}

func fhirSearch(w http.ResponseWriter, req *http.Request, resourceType string) {
	ctxLog := LoggerFromRequest(req)
	queryString := req.URL.Query()
	profile := queryString.Get("_profile")
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

	if LOG_LEVEL < 0 {
		fmt.Println(gqlStr)
	}

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

func validateResource(resourceType string) error {
	if _, exists := schemaDict[resourceType]; !exists {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
	log.Debug("validated resource type", "type", resourceType)
	return nil
}

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

	log.Debug(gqlStr)

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

func SendError(w http.ResponseWriter, msg string, code int) {
	body := OperationOutcome(strconv.Itoa(code), msg, nil)
	w.Header().Set("Content-Type", "application/fhir+json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(body)
}

func dispatch(w http.ResponseWriter, req *http.Request) {
	ctxLog := LoggerFromRequest(req)

	switch req.Method {
	case http.MethodPost:
		fmt.Println("Request Method: POST")
		pathComponents := strings.Split(req.URL.Path, "/")

		switch len(pathComponents) {
		case 1:
			// Server Root
			ctxLog.Info("No path components")
		case 2:
			/// Create Resource
			ctxLog.Info("Create Resource", "type", pathComponents[1])
			FhirCreate(w, req, pathComponents[1])
		case 3:
			// Update Resource
		default:
			ctxLog.Error("Bad Request")
			SendError(w, "Bad Request", http.StatusBadRequest)
		}

	case http.MethodGet:
		if req.URL.Path == HEALTHCHECK_PATH {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		pathComponents := strings.Split(req.URL.Path, "/")
		switch len(pathComponents) {
		case 1:
			// Server Root
			ctxLog.Info("No path components")
			ProxyRequest(w, req)
		case 2:
			/// Resource Type Search
			if err := validateResource(pathComponents[1]); err != nil {
				// Invalid resource type, proxy the request
				ProxyRequest(w, req)
				return
			}
			fhirSearch(w, req, pathComponents[1])
		case 3:
			// Resource Type Read
			if err := validateResource(pathComponents[1]); err != nil {
				// Invalid resource type, proxy the request
				ProxyRequest(w, req)
				return
			}
			fhirRead(w, req, pathComponents[1], pathComponents[2])
		case 4:
			// Compartment Search
			ctxLog.Info("Component Search", "component", pathComponents[1], "id", pathComponents[2], "type", pathComponents[3])
		default:
			ctxLog.Error("Bad Request")
			SendError(w, "Bad Request", http.StatusBadRequest)
		}

	default:
		ctxLog.Info("Request Method: Other")
	}
}

func main() {
	fmt.Println(`
    ________  __________     ____  ____________
   / ____/ / / /  _/ __ \   / __ \/_  __/ ____/
  / /_  / /_/ // // /_/ /  / /_/ / / / / / __  
 / __/ / __  // // _, _/  / _, _/ / / / /_/ /  
/_/   /_/ /_/___/_/ |_|  /_/ |_| /_/  \____/   
                                               `)
	fmt.Printf("FHIR RTG Server Version %s\n", VERSION)

	err := introspect()
	if err != nil {
		log.Error("Failed to introspect schema", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Upstream Server: %s\n", upstream)
	fmt.Printf("Startup Successful! Loaded %d FHIR resource types\n", len(schemaDict))
	fmt.Printf("Log Level: %s | Healthcheck Path: %s\n", LOG_LEVEL.String(), HEALTHCHECK_PATH)
	fmt.Printf("Listening on port %d\n\n", PORT)
	log.Info(fmt.Sprintf("FHIR RTG started with upstream server %s", upstream))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", PORT),
		Handler: LoggingMiddleware(http.HandlerFunc(dispatch)),
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("FHIR RTG failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Info("Shutting down gracefully...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("FHIR RTG forced to shutdown", "error", err)
	}

	log.Info("FHIR RTG stopped")
}
