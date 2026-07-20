package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
)

// sendFunc is the shape of every resty verb method (Get, Post, Put, Patch, Delete).
type sendFunc func(string) (*resty.Response, error)

// verbs returns a fresh request per verb so no state leaks between calls.
func verbs() map[string]sendFunc {
	return map[string]sendFunc{
		"GET":    client.R().Get,
		"POST":   client.R().Post,
		"PUT":    client.R().Put,
		"PATCH":  client.R().Patch,
		"DELETE": client.R().Delete,
	}
}

// A 2xx response means the call succeeded, so the process must exit with 0.
// Anything else keeps reporting the first digit of the status code, which is
// what the docker-compose healthchecks read.
func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected int
	}{
		{"200 OK", http.StatusOK, 0},
		{"201 Created", http.StatusCreated, 0},
		{"202 Accepted", http.StatusAccepted, 0},
		{"204 No Content", http.StatusNoContent, 0},
		{"299 upper bound", 299, 0},
		{"301 Moved Permanently", http.StatusMovedPermanently, 3},
		{"400 Bad Request", http.StatusBadRequest, 4},
		{"404 Not Found", http.StatusNotFound, 4},
		{"500 Internal Server Error", http.StatusInternalServerError, 5},
		{"503 Service Unavailable", http.StatusServiceUnavailable, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.status); got != tt.expected {
				t.Errorf("exitCodeFor(%d) = %d, want %d", tt.status, got, tt.expected)
			}
		})
	}
}

// Every verb, not only GET, has to report a failing status code through the
// exit code. Before the fix POST/PUT/PATCH/DELETE always ended with 0, so a
// script could not tell a failed call from a successful one.
func TestDoRequestExitCodePerVerb(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected int
	}{
		{"created", http.StatusCreated, 0},
		{"no content", http.StatusNoContent, 0},
		{"not found", http.StatusNotFound, 4},
		{"server error", http.StatusInternalServerError, 5},
	}

	for _, tt := range tests {
		tt := tt
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
		}))

		for verb, send := range verbs() {
			t.Run(tt.name+"/"+verb, func(t *testing.T) {
				if got := doRequest(server.URL, send); got != tt.expected {
					t.Errorf("doRequest(%s) with status %d = %d, want %d", verb, tt.status, got, tt.expected)
				}
			})
		}

		server.Close()
	}
}

// A transport error (nothing listening on the address) must end with exit code
// 1 for every verb rather than being printed and then swallowed.
func TestDoRequestTransportErrorExitsNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := server.URL
	server.Close() // nothing is listening on deadURL any more

	for verb, send := range verbs() {
		t.Run(verb, func(t *testing.T) {
			if got := doRequest(deadURL, send); got != 1 {
				t.Errorf("doRequest(%s) against a closed server = %d, want 1", verb, got)
			}
		})
	}
}
