package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// newHTTPServer configures a HTTP server with all required handle funcs
// and their dependencies
func newHTTPServer(port int, fail bool, stop chan os.Signal) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler(fail))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	log.Printf("starting http server [addr: localhost:%d]", port)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Printf("encountered critical error from HTTP server: %v", err)
			// if the server encounters and error, stop everything
			stop <- os.Kill
		}
	}()

	return server
}

// statusResponseWriter wraps the standard http.ResponseWriter and records
// the status when calling WriteHeader
type statusResponseWriter struct {
	status int
	http.ResponseWriter
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// httpLog logs specific HTTP request information
func httpLog(r *http.Request, status int) {
	log.Printf(`"%s %s %s" %d %s`, r.Method, r.URL.Path, r.Proto, status, http.StatusText(status))
}

func handler(fail bool) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		srw := &statusResponseWriter{
			ResponseWriter: rw,
		}

		switch r.Method {
		case http.MethodGet:
			healthz(srw, fail)
		default:
			status := http.StatusText(http.StatusMethodNotAllowed)
			http.Error(srw, status, http.StatusMethodNotAllowed)
		}

		httpLog(r, srw.status)
	}
}

func healthz(rw http.ResponseWriter, fail bool) {
	rw.Header().Set("Content-Type", "text/plain")

	if fail {
		rw.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(rw, "FAILING")
		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintln(rw, "OK")
}
