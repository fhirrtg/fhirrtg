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

	"github.com/fhirrtg/fhirrtg/gql"
)

const (
	VERSION                   = "0.1"
	DEFAULT_PORT              = 8888
	DEFAULT_GQL_ACCEPT_HEADER = "application/graphql-response+json;charset=utf-8, application/json;charset=utf-8"
)

var (
	PORT              int
	GQL_ACCEPT_HEADER string
	SKIP_TLS_VERIFY   bool
)

var (
	client   *http.Client
	log      *slog.Logger
	upstream string
)

func init() {
	var logLevel slog.Level
	logLevelStr := getEnv("RTG_LOG_LEVEL", "info")
	switch strings.ToLower(logLevelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
		fmt.Printf("Invalid log level: %s, using default: info\n", logLevelStr)
	}
	log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	portStr := getEnv("RTG_PORT", strconv.Itoa(DEFAULT_PORT))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Printf("Invalid port number: %s, using default: %d\n", portStr, DEFAULT_PORT)
		PORT = DEFAULT_PORT
	} else {
		PORT = port
	}

	GQL_ACCEPT_HEADER = getEnv("RTG_GQL_ACCEPT_HEADER", DEFAULT_GQL_ACCEPT_HEADER)
	SKIP_TLS_VERIFY = getEnv("RTG_SKIP_TLS_VERIFY", "false") == "true"

	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: SKIP_TLS_VERIFY,
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
			http.Error(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, err.Error(), http.StatusBadRequest)
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

	// fmt.Println("GQL Query:")
	log.Debug(gqlStr)

	resp := GqlRequest(gqlStr, profile)
	bundle := ProcessBundle(resp, req)
	w.Write(bundle)
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

	resp := GqlRequest(gqlStr, profile)
	resource := ProcessRead(resp, req)
	w.Write(resource)
}

func parseQueryString(w http.ResponseWriter, req *http.Request) {
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
			log.Info("Create Resource")
			log.Info("Create Resource", "type", pathComponents[1])
			FhirCreate(w, req, pathComponents[1])
		case 3:
			// Update Resource
		default:
			log.Debug("Bad Request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
		}

		// Handle POST request
	case http.MethodGet:

		pathComponents := strings.Split(req.URL.Path, "/")
		switch len(pathComponents) {
		case 1:
			// Server Root
			log.Info("No path components")
		case 2:
			/// Resource Type Search
			fhirSearch(w, req, pathComponents[1])
		case 3:
			// Resource Type Read
			fhirRead(w, req, pathComponents[1], pathComponents[2])
		case 4:
			// Compartment Search
			log.Info("Compartment Search")
			log.Info("Component", "value", pathComponents[1])
			log.Info("ID", "value", pathComponents[2])
			log.Info("Type", "value", pathComponents[3])
		default:
			log.Error("Bad Request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
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

	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("No upstream server specified")
		os.Exit(1)
	}
	upstream = args[0]
	log.Info(fmt.Sprintf("FHIR RTG started with upstream server %s", upstream))

	introspect()

	// fmt.Println("-----------\n Reqeust:")
	// patientFragment := GenerateFragment("Patient")
	// encounterFragment := GenerateFragment("Encounter")
	// q := FullResourceRequest(encounterFragment)
	// fmt.Print(patientFragment.String())
	// fmt.Print(encounterFragment.String())
	// fmt.Println(q.String())

	http.HandleFunc("/", parseQueryString)
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}
