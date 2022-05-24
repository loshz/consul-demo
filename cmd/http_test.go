package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPHandler(t *testing.T) {
	t.Run("TestMethodNotAllowed", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(healthzHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected: 500 Method Not Allowed, got: %s", http.StatusText(rr.Code))
		}
	})

	t.Run("TestStatusOK", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(healthzHandler)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected: 200 OK, got: %s", http.StatusText(rr.Code))
		}
	})
}
