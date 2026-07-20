package apicall

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An api.yaml method that is not one of the five supported verbs used to fall
// through the switch with a nil response and panic in ProcessResultInfo. It has
// to come back as a plain error naming the supported methods instead.
func TestCallRestUnsupportedMethodReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	unsupported := []string{"TRACE", "HEAD", "OPTIONS", "gett", "", "   "}

	for _, method := range unsupported {
		t.Run("method_"+method, func(t *testing.T) {
			serviceInfo = ServiceInfo{BaseURL: server.URL, ResourcePath: "/", Method: method}

			err := callRest()
			if err == nil {
				t.Fatalf("callRest() with method %q returned no error, want an unsupported method error", method)
			}
			if !strings.Contains(err.Error(), "unsupported HTTP method") {
				t.Errorf("callRest() error = %q, want it to mention the unsupported method", err)
			}
			if !strings.Contains(err.Error(), "GET, POST, PUT, DELETE, PATCH") {
				t.Errorf("callRest() error = %q, want it to list the supported methods", err)
			}
		})
	}
}

// Methods written in any case, or padded with spaces, are still accepted.
func TestCallRestAcceptsSupportedMethods(t *testing.T) {
	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := map[string]string{
		"get":      http.MethodGet,
		"POST":     http.MethodPost,
		"  put  ":  http.MethodPut,
		"Delete":   http.MethodDelete,
		"\tpatch ": http.MethodPatch,
	}

	for method, want := range tests {
		t.Run("method_"+strings.TrimSpace(method), func(t *testing.T) {
			seen = ""
			serviceInfo = ServiceInfo{BaseURL: server.URL, ResourcePath: "/", Method: method}

			if err := callRest(); err != nil {
				t.Fatalf("callRest() with method %q returned error: %v", method, err)
			}
			if seen != want {
				t.Errorf("server received %s, want %s", seen, want)
			}
		})
	}
}

// A path parameter must not be able to redirect the call to another endpoint.
func TestIsSafePathParamValue(t *testing.T) {
	safe := []string{"ns01", "mci01", "ap-northeast-3", "aws-config01", "AWS", "my/object/key.txt", "a..b", "..leading"}
	for _, value := range safe {
		if !isSafePathParamValue(value) {
			t.Errorf("isSafePathParamValue(%q) = false, want true", value)
		}
	}

	unsafe := []string{"../../v1/ns", "..", "ns01/..", "ns01/../../other", "ns01?option=terminate", "ns01#frag"}
	for _, value := range unsafe {
		if isSafePathParamValue(value) {
			t.Errorf("isSafePathParamValue(%q) = true, want false", value)
		}
	}
}

// parsePathParam has to refuse a traversal value rather than substituting it
// into the resource path.
func TestParsePathParamRejectsTraversal(t *testing.T) {
	serviceInfo = ServiceInfo{ResourcePath: "/ns/{nsId}/mci/{mciId}"}
	pathParam = "nsId:../../v1/ns mciId:mci01"
	defer func() { pathParam = "" }()

	err := parsePathParam()
	if err == nil {
		t.Fatal("parsePathParam() accepted a traversal path parameter, want an error")
	}
	if !strings.Contains(err.Error(), "nsId") {
		t.Errorf("parsePathParam() error = %q, want it to name the offending parameter", err)
	}
}

// The normal substitution path keeps working unchanged.
func TestParsePathParamSubstitutesNormalValues(t *testing.T) {
	serviceInfo = ServiceInfo{ResourcePath: "/ns/{nsId}/mci/{mciId}"}
	pathParam = "nsId:ns01 mciId:mci01"
	defer func() { pathParam = "" }()

	if err := parsePathParam(); err != nil {
		t.Fatalf("parsePathParam() returned error: %v", err)
	}
	if serviceInfo.ResourcePath != "/ns/ns01/mci/mci01" {
		t.Errorf("ResourcePath = %q, want %q", serviceInfo.ResourcePath, "/ns/ns01/mci/mci01")
	}
}
