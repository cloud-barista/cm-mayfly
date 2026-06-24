package common

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// GET retries idempotent requests on transient 5xx, then succeeds.
func TestRetryGetOnTransient5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	resp, err := NewHTTPClient().R().Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode())
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("hits=%d, want 3 (initial + 2 retries)", got)
	}
}

// POST is non-idempotent and must never be retried, even on 5xx.
func TestNoRetryPostOn5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, _ = NewHTTPClient().R().Post(srv.URL)
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("POST hits=%d, want 1 (no retry)", got)
	}
}

// 4xx is a real response and must not be retried.
func TestNoRetryOn4xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _ = NewHTTPClient().R().Get(srv.URL)
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("404 hits=%d, want 1 (no retry)", got)
	}
}

// With keep-alive disabled, every request opens a fresh connection, so a path
// that silently drops an idle connection can never be reused. We assert each
// request arrives on a distinct client address.
func TestFreshConnectionPerRequest(t *testing.T) {
	var mu sync.Mutex
	addrs := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		addrs[r.RemoteAddr] = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient()
	for i := 0; i < 3; i++ {
		if _, err := c.R().Get(srv.URL); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if len(addrs) != 3 {
		t.Fatalf("distinct client connections=%d, want 3 (fresh connection per request)", len(addrs))
	}
}

// A handler slower than the timeout returns an error promptly instead of
// hanging forever. POST is used so the timeout is observed as a single attempt.
func TestTimeoutBounded(t *testing.T) {
	t.Setenv("MAYFLY_HTTP_TIMEOUT", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	start := time.Now()
	_, err := NewHTTPClient().R().Post(srv.URL)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("took %v — single-attempt timeout not enforced (POST should not retry)", elapsed)
	}
}

// A normal call succeeds in a single attempt, and a large response body is
// received in full (a fresh connection per request handles large transfers).
func TestNormalAndLargeResponse(t *testing.T) {
	const size = 8 << 20 // 8 MiB
	body := make([]byte, size)
	for i := range body {
		body[i] = byte('a' + i%26)
	}
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	resp, err := NewHTTPClient().R().Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode())
	}
	if len(resp.Body()) != size {
		t.Fatalf("body=%d bytes, want %d (large response truncated)", len(resp.Body()), size)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("hits=%d, want 1 (a successful call is not retried)", got)
	}
}

// MAYFLY_HTTP_TIMEOUT overrides the default, and an unset/invalid value falls
// back to the default.
func TestHTTPTimeoutOverride(t *testing.T) {
	t.Setenv("MAYFLY_HTTP_TIMEOUT", "7")
	if got := httpTimeout(); got != 7*time.Second {
		t.Fatalf("override: got %v, want 7s", got)
	}
	t.Setenv("MAYFLY_HTTP_TIMEOUT", "")
	if got := httpTimeout(); got != defaultHTTPTimeout {
		t.Fatalf("default: got %v, want %v", got, defaultHTTPTimeout)
	}
	t.Setenv("MAYFLY_HTTP_TIMEOUT", "abc")
	if got := httpTimeout(); got != defaultHTTPTimeout {
		t.Fatalf("invalid falls back: got %v, want %v", got, defaultHTTPTimeout)
	}
}
