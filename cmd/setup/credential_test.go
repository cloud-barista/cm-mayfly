package setup

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// samplePayload is the shape processCspCredentialEncrypt builds.
func samplePayload() map[string]interface{} {
	return map[string]interface{}{
		"credentialHolder": "admin",
		"providerName":     "aws",
	}
}

// hostPortOf splits an httptest server URL into its host and port.
func hostPortOf(t *testing.T, rawURL string) (string, string) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	return parsed.Hostname(), parsed.Port()
}

// --host/--port must move the credential registration call off the default
// localhost:1323, so one machine can initialise the Tumblebug of another.
func TestUpdateBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		host     string
		port     string
		expected string
	}{
		{"host and port", "http://localhost:1323/tumblebug", "10.0.0.5", "8080", "http://10.0.0.5:8080/tumblebug"},
		{"host only", "http://localhost:1323/tumblebug", "10.0.0.5", "", "http://10.0.0.5:1323/tumblebug"},
		{"port only", "http://localhost:1323/tumblebug", "", "8080", "http://localhost:8080/tumblebug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL := tt.baseURL
			if err := updateBaseURL(&baseURL, tt.host, tt.port); err != nil {
				t.Fatalf("updateBaseURL returned error: %v", err)
			}
			if baseURL != tt.expected {
				t.Errorf("updateBaseURL = %q, want %q", baseURL, tt.expected)
			}
		})
	}
}

// The registration POST used to be hardwired to http://localhost:1323 with a
// hardcoded default:default header. It must now follow the resolved base URL
// and credentials like every other call in this file.
func TestSendCredentialsUsesConfiguredHostAndAuth(t *testing.T) {
	var gotPath, gotMethod, gotAuth, gotContentType, gotCustomHeader string
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotCustomHeader = r.Header.Get("X-Test-Header")

		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"registered"}`))
	}))
	defer server.Close()

	testHost, testPort := hostPortOf(t, server.URL)

	// start from the api.yaml default and apply --host/--port like the command does
	serviceInfo = ServiceInfo{BaseURL: "http://localhost:1323/tumblebug"}
	if err := updateBaseURL(&serviceInfo.BaseURL, testHost, testPort); err != nil {
		t.Fatalf("updateBaseURL returned error: %v", err)
	}

	// --user/--password and -H are resolved the same way as for the other calls
	serviceInfo.Auth = Auth{Type: "basic", Username: "tester", Password: "secret"}
	SetAuth()
	headers = []string{"X-Test-Header: from-flag"}
	defer func() { headers = nil }()

	result, err := sendCredentials(samplePayload())
	if err != nil {
		t.Fatalf("sendCredentials returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/tumblebug"+POST_CREDENTIAL_URL {
		t.Errorf("path = %q, want %q", gotPath, "/tumblebug"+POST_CREDENTIAL_URL)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotCustomHeader != "from-flag" {
		t.Errorf("X-Test-Header = %q, want the value passed with -H", gotCustomHeader)
	}

	// The registration call used to carry a fixed default:default header no
	// matter what the caller configured. Encode it here rather than spelling
	// the base64 out, so the check reads as "the built-in pair" instead of an
	// opaque string.
	builtInPair := base64.StdEncoding.EncodeToString([]byte("default:default"))
	if strings.Contains(gotAuth, builtInPair) {
		t.Errorf("Authorization = %q, want the credentials resolved from the flags", gotAuth)
	}
	if gotAuth == "" {
		t.Error("Authorization header was not sent")
	}

	if gotBody["providerName"] != "aws" {
		t.Errorf("payload providerName = %v, want aws", gotBody["providerName"])
	}
	if result["message"] != "registered" {
		t.Errorf("result = %v, want the parsed response body", result)
	}
}

// A non-2xx response used to look exactly like a success. It has to be reported
// as a failure, with the response body kept for diagnosis.
func TestSendCredentialsFailsOnNon2xx(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", http.StatusUnauthorized, `{"message":"invalid credentials"}`},
		{"server error", http.StatusInternalServerError, `{"message":"boom"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			serviceInfo = ServiceInfo{BaseURL: server.URL}

			result, err := sendCredentials(samplePayload())
			if err == nil {
				t.Fatalf("sendCredentials returned no error for status %d", tt.status)
			}
			if result != nil {
				t.Errorf("result = %v, want nil on failure", result)
			}
			if !strings.Contains(err.Error(), tt.body) {
				t.Errorf("error = %q, want it to carry the response body %q", err, tt.body)
			}
		})
	}
}

// A transport error must be returned, not raised as a panic with a Go stack
// trace in the user's face.
func TestSendCredentialsReturnsErrorInsteadOfPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := server.URL
	server.Close() // nothing is listening any more

	serviceInfo = ServiceInfo{BaseURL: deadURL}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sendCredentials panicked: %v", r)
		}
	}()

	if _, err := sendCredentials(samplePayload()); err == nil {
		t.Error("sendCredentials returned no error for an unreachable server")
	}
}

// Each call builds its own request so a body set by one call cannot leak into
// the next one.
func TestNewRequestDoesNotShareState(t *testing.T) {
	first := newRequest()
	first.SetBody("{}")

	if second := newRequest(); second.Body != nil {
		t.Errorf("a freshly built request already carries a body: %v", second.Body)
	}
}
