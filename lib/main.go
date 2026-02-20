package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	VERSION                   = "0.1"
	DEFAULT_PORT              = 8888
	DEFAULT_GQL_ACCEPT_HEADER = "application/graphql-response+json;charset=utf-8, application/json;charset=utf-8"
)

var (
	HEALTHCHECK_PATH   = "/health"
	PORT               int
	GQL_ACCEPT_HEADER  string
	LOG_LEVEL          slog.Level
	MAX_STARTUP_WAIT_S = 60 // seconds
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

	maxTimeout, err := strconv.Atoi(getEnv("RTG_MAX_STARTUP_WAIT_S", "60"))
	if err != nil {
		fmt.Printf("Invalid MAX_STARTUP_WAIT_S value: %s, using default: 60\n", getEnv("RTG_MAX_STARTUP_WAIT_S", "60"))
		MAX_STARTUP_WAIT_S = 60
	} else {
		MAX_STARTUP_WAIT_S = maxTimeout
	}

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

func validateResource(resourceType string) error {
	if _, exists := schemaDict[resourceType]; !exists {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
	log.Debug("validated resource type", "type", resourceType)
	return nil
}

func SendError(w http.ResponseWriter, msg string, code int) {
	body := OperationOutcome(strconv.Itoa(code), msg, nil)
	w.Header().Set("Content-Type", "application/fhir+json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(body)
}

func dispatch(w http.ResponseWriter, req *http.Request) {
	ctxLog := LoggerFromRequest(req)

	// Ignore Accept-encoding (gzip, deflate, br)
	req.Header.Del("Accept-Encoding")

	// Remove /fhir prefix if present
	// TODO: Make this configurable
	if strings.HasPrefix(req.URL.Path, "/fhir") {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/fhir")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}

	switch req.Method {
	case http.MethodPost:
		fmt.Println("Request Method: POST")
		pathComponents := strings.Split(req.URL.Path, "/")

		switch len(pathComponents) {
		case 1:
			// Server Root
			ProxyRequest(w, req)
			return
		case 2:
			/// Create Resource
			fhirCreate(w, req, pathComponents[1])
			return
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
		ProxyRequest(w, req)
	}
}

func main() {
	fmt.Printf("Starting FHIR RTG server with upstream %s for %d seconds...\n\n", upstream, MAX_STARTUP_WAIT_S)

	startupAt := time.Now()
	for {
		err := introspect()
		if err == nil {
			break
		}
		if time.Since(startupAt) > time.Duration(MAX_STARTUP_WAIT_S)*time.Second {
			fmt.Fprintf(os.Stderr, "\nFailed to connect to upstream server %s within %d seconds\n\n", upstream, MAX_STARTUP_WAIT_S)
			os.Exit(1)
		}
		log.Warn("Upstream server not available, retrying...", "error", err)
		time.Sleep(5 * time.Second)
	}

	fmt.Printf("Startup successful! Loaded %d FHIR resource types\n", len(schemaDict))
	fmt.Println(`
	    ________  __________     ____  ____________
	   / ____/ / / /  _/ __ \   / __ \/_  __/ ____/
	  / /_  / /_/ // // /_/ /  / /_/ / / / / / __  
	 / __/ / __  // // _, _/  / _, _/ / / / /_/ /  
	/_/   /_/ /_/___/_/ |_|  /_/ |_| /_/  \____/   
	`)
	fmt.Printf("FHIR RTG server version %s\n", VERSION)
	fmt.Printf("Connected to : %s\n", upstream)
	fmt.Printf("Log level: %s | Healthcheck path: %s\n", LOG_LEVEL.String(), HEALTHCHECK_PATH)
	fmt.Printf("Awaiting connections on port %d\n\n", PORT)
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
