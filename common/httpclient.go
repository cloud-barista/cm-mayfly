package common

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

// defaultHTTPTimeout bounds each api/rest request. Override with the
// MAYFLY_HTTP_TIMEOUT environment variable (whole seconds).
const defaultHTTPTimeout = 30 * time.Second

func httpTimeout() time.Duration {
	if v := os.Getenv("MAYFLY_HTTP_TIMEOUT"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return defaultHTTPTimeout
}

// isIdempotent reports whether an HTTP method is safe to retry (no duplicated
// side effect). Only GET/HEAD are treated as retryable here — PUT/DELETE are
// idempotent by spec but excluded to stay conservative for the api command.
func isIdempotent(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// NewHTTPClient returns a resty client configured for cm-mayfly's short-lived
// CLI usage.
//
// cm-mayfly runs one (or a few) calls per process, so a connection pool buys
// little but introduces a real failure mode: on a long-lived host behind
// NAT/conntrack an idle pooled connection can be dropped on the path without a
// FIN/RST. The next request reuses that dead connection and then waits forever
// for a response that never arrives. curl never hits this because it dials a
// fresh connection per call.
//
//   - DisableKeepAlives — every request uses a fresh connection (like curl),
//     so a dropped idle connection can never be reused. For stateless REST with
//     per-request auth (Basic/Bearer, which is all cm-mayfly calls) this is
//     behaviourally identical to keep-alive; only the per-call TCP/TLS handshake
//     cost differs, which is negligible for a CLI.
//   - SetTimeout — bounds each request so a genuinely stuck call fails instead
//     of hanging indefinitely.
//   - Retry — idempotent (GET/HEAD) requests are retried on transport errors and
//     transient 5xx (502/503/504). Non-idempotent methods and 4xx are never
//     retried.
func NewHTTPClient() *resty.Client {
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: -1,
		}).DialContext,
	}

	return resty.New().
		SetTransport(transport).
		SetTimeout(httpTimeout()).
		SetRetryCount(2).
		SetRetryWaitTime(300 * time.Millisecond).
		SetRetryMaxWaitTime(2 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if r == nil || r.Request == nil || !isIdempotent(r.Request.Method) {
				return false
			}
			if err != nil {
				// transport-level error: no usable response was received
				return true
			}
			switch r.StatusCode() {
			case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				return true
			}
			return false
		})
}
