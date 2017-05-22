package main

import (
	"io"
	"log"
	"net/http"
)

const version string = "2.0.2"

// VersionHandler handles incoming requests to /version
// and just returns a simple version number
func versionHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, version)
}

// RootHandler handles incoming requests to /
// and just returns Hello, World!
func rootHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello, World!")
}

func main() {
	log.Printf("Listening on port 8000...")
	http.HandleFunc("/version", versionHandler)
	http.HandleFunc("/", rootHandler)
	http.ListenAndServe(":8000", nil)
}
