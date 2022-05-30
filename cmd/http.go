package main

import (
	"fmt"
	"log"
	"net/http"
)

// NewHTTPServer configures a HTTP server with all required handle funcs
// and their dependencies.
func NewHTTPServer(addr string, errCh chan error) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server
}

// httpLog logs specific HTTP request information
func httpLog(r *http.Request, status int) {
	log.Printf(`"%s %s %s" %d %s`, r.Method, r.URL.Path, r.Proto, status, http.StatusText(status))
}

func healthzHandler(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		status := http.StatusMethodNotAllowed
		http.Error(rw, http.StatusText(status), http.StatusMethodNotAllowed)
		httpLog(r, status)
		return
	}

	rw.Header().Set("Content-Type", "text/plain")
	rw.WriteHeader(http.StatusOK)
	fmt.Fprintln(rw, "OK")
	httpLog(r, http.StatusOK)
}
