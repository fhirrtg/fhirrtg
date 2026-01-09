package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	log.Info("Healthcheck path set to", "path", HEALTHCHECK_PATH)

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

	resp, err := GqlRequest(gqlStr, profile, req)
	if err != nil && resp == nil {
		SendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bundle := ProcessBundle(resp, req)
	w.Write(bundle)
}

func validateResource(resourceType string) error {
	if _, exists := schemaDict[resourceType]; !exists {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
	log.Info("validated resource type", "type", resourceType)
	return nil
}

func fhirRead(w http.ResponseWriter, req *http.Request, resourceType string, id string) {
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

	log.Debug("GQL Query:")
	log.Debug(gqlStr)

	resp, err := GqlRequest(gqlStr, profile, req)
	if err != nil && resp == nil {
		SendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resource := ProcessRead(resp, req)
	w.Write(resource)
}

func SendError(w http.ResponseWriter, msg string, code int) {
	body := OperationOutcome(strconv.Itoa(code), msg, nil)
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(code)
	w.Write(body)
}

func dispatch(w http.ResponseWriter, req *http.Request) {
	log.Info("parsing request", "method", req.Method, "path", req.URL.Path, "query", req.URL.RawQuery, "remote_addr", req.RemoteAddr)

	switch req.Method {
	case http.MethodPost:
		fmt.Println("Request Method: POST")
		pathComponents := strings.Split(req.URL.Path, "/")

		switch len(pathComponents) {
		case 1:
			// Server Root
			log.Info("No path components")
		case 2:
			/// Create Resource
			log.Info("Create Resource", "type", pathComponents[1])
			FhirCreate(w, req, pathComponents[1])
		case 3:
			// Update Resource
		default:
			log.Error("Bad Request")
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
			log.Info("No path components")
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
			log.Info("Component Search", "component", pathComponents[1], "id", pathComponents[2], "type", pathComponents[3])
		default:
			log.Error("Bad Request")
			SendError(w, "Bad Request", http.StatusBadRequest)
		}

	default:
		log.Info("Request Method: Other")
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
	fmt.Printf("Listening on port %d\n", PORT)
	log.Info(fmt.Sprintf("FHIR RTG started with upstream server %s", upstream))

	err := introspect()
	if err != nil {
		log.Error("Failed to introspect schema", "error", err)
		os.Exit(1)
	}

	http.HandleFunc("/", dispatch)
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}
