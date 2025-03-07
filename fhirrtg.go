package main

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	PORT = 8888
)

func parseQueryString(w http.ResponseWriter, req *http.Request) {

	// Determine if the request is a POST or GET
	switch req.Method {
	case http.MethodPost:
		fmt.Println("Request Method: POST")
	case http.MethodGet:
		fmt.Println("Request Method: GET")

		pathComponents := strings.Split(req.URL.Path, "/")
		switch len(pathComponents) {
		case 1:
			// Server Root
			fmt.Println("No path components")
		case 2:
			/// Resource Type Search
			fmt.Println("Resource Type Search")
			fmt.Println(" Type: ", pathComponents[1])
		case 3:
			// Resource Type Read
			fmt.Println("Resource Type Read")
			fmt.Println(" Type:", pathComponents[1])
			fmt.Println(" ID:", pathComponents[2])
		case 4:
			// Compartment Search
			fmt.Println("Compartment Search")
			fmt.Println(" Component: ", pathComponents[1])
			fmt.Println(" ID: ", pathComponents[2])
			fmt.Println(" Type: ", pathComponents[3])
		default:
			fmt.Println("Invalid path")
		}

	default:
		fmt.Println("Request Method: Other")
	}

	// Parse the query string
	queryString := req.URL.Query()

	// Print the query string
	fmt.Print("Query String:\n")

	for key, values := range queryString {
		for _, value := range values {
			fmt.Fprintf(w, "%s: %s\n", key, value)
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	fmt.Println("-----------")
	fmt.Println("")
}

func main() {
	fmt.Println(`    ________  __________     ____  ____________
   / ____/ / / /  _/ __ \   / __ \/_  __/ ____/
  / /_  / /_/ // // /_/ /  / /_/ / / / / / __  
 / __/ / __  // // _, _/  / _, _/ / / / /_/ /  
/_/   /_/ /_/___/_/ |_|  /_/ |_| /_/  \____/   
                                               `)
	fmt.Println("FHIR RTG Server Version 0.1")
	fmt.Printf("Listening on port %d\n", PORT)

	http.HandleFunc("/", parseQueryString)
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}
