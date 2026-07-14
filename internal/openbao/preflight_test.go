package openbao

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- signal helpers ---------------------------------------------------------

func TestInitFileShapeParsesRootToken(t *testing.T) {
	raw := `{"keys":["abc123"],"keys_base64":["Zm9v"],"root_token":"test-root-token-not-real"}`
	var s initFileShape
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.firstKey() != "abc123" {
		t.Errorf("firstKey = %q, want abc123", s.firstKey())
	}
	if s.RootToken != "test-root-token-not-real" {
		t.Errorf("RootToken = %q, want test-root-token-not-real", s.RootToken)
	}
}

// fixtureRoot builds a temp mayfly root with conf/docker/{.env,data/openbao/...}
// and chdir's into it for the duration of the test.
func fixtureRoot(t *testing.T, token, initJSON string, dataPopulated bool) {
	t.Helper()
	root := t.TempDir()
	dockerDir := filepath.Join(root, "conf", "docker")
	secretsDir := filepath.Join(dockerDir, "data", "openbao", "secrets")
	dataDir := filepath.Join(dockerDir, "data", "openbao", "data")
	for _, d := range []string{secretsDir, dataDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := ""
	if token != "" {
		env = "VAULT_TOKEN=" + token + "\nTUMBLEBUG_DB_PASSWORD=keepme\n"
	} else {
		env = "VAULT_TOKEN=\nTUMBLEBUG_DB_PASSWORD=keepme\n"
	}
	if err := os.WriteFile(filepath.Join(dockerDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if initJSON != "" {
		if err := os.WriteFile(filepath.Join(secretsDir, "openbao-init.json"), []byte(initJSON), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if dataPopulated {
		if err := os.WriteFile(filepath.Join(dataDir, "core"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
}

func TestDataDirState(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		t.Chdir(t.TempDir())
		pop, known := dataDirState()
		if pop || !known {
			t.Errorf("absent dir: pop=%v known=%v, want false,true", pop, known)
		}
	})
	t.Run("empty", func(t *testing.T) {
		fixtureRoot(t, "", "", false)
		pop, known := dataDirState()
		if pop || !known {
			t.Errorf("empty dir: pop=%v known=%v, want false,true", pop, known)
		}
	})
	t.Run("populated", func(t *testing.T) {
		fixtureRoot(t, "", "", true)
		pop, known := dataDirState()
		if !pop || !known {
			t.Errorf("populated dir: pop=%v known=%v, want true,true", pop, known)
		}
	})
}

func TestReadInitFile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		fixtureRoot(t, "", `{"keys":["k1"],"root_token":"test-root-token-not-real"}`, true)
		shape, ok := readInitFile()
		if !ok || shape.RootToken != "test-root-token-not-real" {
			t.Errorf("ok=%v root=%q", ok, shape.RootToken)
		}
	})
	t.Run("absent", func(t *testing.T) {
		fixtureRoot(t, "", "", false)
		if _, ok := readInitFile(); ok {
			t.Error("absent init.json should be not-ok")
		}
	})
	t.Run("no-keys", func(t *testing.T) {
		fixtureRoot(t, "", `{"root_token":"x"}`, false)
		if _, ok := readInitFile(); ok {
			t.Error("init.json without unseal keys should be not-ok")
		}
	})
}

// --- disk-only verdict (openbao down) --------------------------------------

func TestDecideDisk(t *testing.T) {
	cases := []struct {
		name              string
		envToken, initJSON bool
		dPop, dKnown      bool
		want              Case
		wantOK            bool
	}{
		{"fresh", false, false, false, true, CaseFresh, true},
		{"orphaned-C3", true, false, false, true, CaseOrphanedToken, false},
		{"stale-json-C4", true, true, false, true, CaseStaleInitJSON, false},
		{"unknown-data-unreadable", true, false, false, false, CaseUnknown, false},
		{"unknown-token-and-data", false, true, false, true, CaseUnknown, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Result{EnvToken: c.envToken, InitJSON: c.initJSON, DataDir: c.dPop}
			got := r.decideDisk(c.dPop, c.dKnown)
			if got.Case != c.want || got.OK != c.wantOK {
				t.Errorf("Case=%v OK=%v, want %v %v", got.Case, got.OK, c.want, c.wantOK)
			}
			if !got.OK && got.Advice == "" {
				t.Error("non-OK verdict must carry advice")
			}
		})
	}
}

// --- API-authoritative verdict (openbao up) --------------------------------

func TestDecideReachable(t *testing.T) {
	shape := initFileShape{Keys: []string{"k1"}, RootToken: "test-root-token-not-real"}
	cases := []struct {
		name                                                  string
		init, sealed, active, envTok, tokValid, tokUnknown, initJSON bool
		dPop                                                  bool
		want                                                  Case
		wantOK                                                bool
	}{
		// active=true is the normal reachable+unsealed state; token cases below
		// exercise the tri-state V signal.
		{"consistent-C2", true, false, true, true, true, false, true, true, CaseConsistent, true},
		{"lost-token-C5", true, false, true, false, false, false, true, true, CaseLostToken, false},
		{"wrong-token-C7", true, false, true, true, false, false, true, true, CaseWrongToken, false},
		{"stuck-sealed-C8", true, true, false, true, false, false, true, true, CaseStuckSealed, false},
		{"corrupt-C6", false, false, false, true, false, false, true, true, CaseCorrupt, false},
		{"orphaned-C3", false, false, false, true, false, false, false, false, CaseOrphanedToken, false},
		{"fresh-C1", false, false, false, false, false, false, false, false, CaseFresh, true},
		// follow-up: transient token → unknown → non-blocking OK (not C7).
		{"token-unknown-nonblocking", true, false, true, true, false, true, true, true, CaseConsistent, true},
		// follow-up: unsealed but API never became active → not-ready (not C7).
		{"not-ready", true, false, false, true, false, false, true, true, CaseNotReady, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Result{
				Initialized: c.init, Sealed: c.sealed, Active: c.active,
				EnvToken: c.envTok, TokenValid: c.tokValid, TokenUnknown: c.tokUnknown, InitJSON: c.initJSON,
			}
			got := r.decideReachable(shape, c.dPop)
			if got.Case != c.want || got.OK != c.wantOK {
				t.Errorf("Case=%v OK=%v, want %v %v", got.Case, got.OK, c.want, c.wantOK)
			}
			// The unknown case must proceed (OK) yet carry a note so the user is told.
			if c.name == "token-unknown-nonblocking" && got.Note == "" {
				t.Error("token-unknown verdict must carry an informational Note")
			}
		})
	}
}

// --- advice content ---------------------------------------------------------

func TestAdviceForMasksAndPaths(t *testing.T) {
	shape := initFileShape{Keys: []string{"k1"}, RootToken: "test-root-token-not-real"}
	adv := adviceFor(CaseLostToken, shape, true)
	if !strings.Contains(adv, "test-roo") {
		t.Errorf("C5 advice should show masked root_token, got: %s", adv)
	}
	if strings.Contains(adv, "test-root-token-not-real") {
		t.Error("advice must not contain the full unmasked token")
	}
	if !strings.Contains(adv, jsonPathHint) || !strings.Contains(adv, envPathHint) {
		t.Error("advice should name both the init.json and .env paths")
	}
	if adviceFor(CaseFresh, shape, true) != "" || adviceFor(CaseConsistent, shape, true) != "" {
		t.Error("OK cases should carry no advice")
	}
}

// --- follow-up: reachability-aware advice + not-ready ------------------------

func TestAdviceForReachabilityAndNotReady(t *testing.T) {
	// CaseUnknown must not claim "not running" when openbao IS reachable.
	up := adviceFor(CaseUnknown, initFileShape{}, true)
	if strings.Contains(up, "not running") {
		t.Errorf("reachable CaseUnknown advice must not say 'not running', got: %s", up)
	}
	down := adviceFor(CaseUnknown, initFileShape{}, false)
	if !strings.Contains(down, "not running") {
		t.Errorf("unreachable CaseUnknown advice should say 'not running', got: %s", down)
	}
	// CaseNotReady carries transition guidance and is distinct from wrong-token.
	nr := adviceFor(CaseNotReady, initFileShape{}, true)
	if nr == "" || !strings.Contains(nr, "active") {
		t.Errorf("CaseNotReady advice should explain the not-active state, got: %s", nr)
	}
}

// --- follow-up: readiness gate + tri-state token probe (httptest) -----------

// TestWaitOpenbaoActiveWaitsForTransition drives the FR-01 gate: /v1/sys/health
// answers 503 (still transitioning) for the first two polls, then 200 (active).
// The gate must keep polling and return nil once it sees 200 — proving the
// unseal→active transition window is absorbed as readiness, not leaked onward.
func TestWaitOpenbaoActiveWaitsForTransition(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 — sealed/transitioning
			return
		}
		w.WriteHeader(http.StatusOK) // 200 — active
	}))
	defer srv.Close()

	if err := waitOpenbaoActive(srv.URL, 10*time.Second); err != nil {
		t.Fatalf("gate should pass once health returns 200, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got < 3 {
		t.Errorf("gate returned after %d polls, expected to wait through the 503 window (>=3)", got)
	}
}

// TestWaitOpenbaoActiveTimesOut: a health endpoint stuck at 503 must make the
// gate return an error within the bound (→ CaseNotReady, never a token verdict).
func TestWaitOpenbaoActiveTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if err := waitOpenbaoActive(srv.URL, 2*time.Second); err == nil {
		t.Error("gate should time out when health never returns 200")
	}
}

// TestProbeTokenAuthStatusMapping covers the FR-02 tri-state: 200→valid,
// 401/403→invalid (definitive), 500/503/connection-error→unknown (never
// invalid). Empty token is invalid without a network call.
func TestProbeTokenAuthStatusMapping(t *testing.T) {
	newSrv := func(code int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
	}
	cases := []struct {
		name string
		code int
		want tokenAuthState
	}{
		{"200-valid", http.StatusOK, authValid},
		{"401-invalid", http.StatusUnauthorized, authInvalid},
		{"403-invalid", http.StatusForbidden, authInvalid},
		{"500-unknown", http.StatusInternalServerError, authUnknown},
		{"503-unknown", http.StatusServiceUnavailable, authUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newSrv(c.code)
			defer srv.Close()
			if got := probeTokenAuth(srv.URL, "some-token"); got != c.want {
				t.Errorf("status %d → %v, want %v", c.code, got, c.want)
			}
		})
	}

	t.Run("empty-token-invalid", func(t *testing.T) {
		if got := probeTokenAuth("http://localhost:1", ""); got != authInvalid {
			t.Errorf("empty token → %v, want authInvalid", got)
		}
	})

	t.Run("connection-error-unknown", func(t *testing.T) {
		// Point at a closed server → connection refused → transient → unknown.
		srv := newSrv(http.StatusOK)
		addr := srv.URL
		srv.Close()
		if got := probeTokenAuth(addr, "some-token"); got != authUnknown {
			t.Errorf("connection error → %v, want authUnknown (never invalid)", got)
		}
	})

	t.Run("redirect-not-followed-token-not-leaked", func(t *testing.T) {
		// A malicious/misconfigured endpoint answering 3xx must not cause the
		// X-Vault-Token to be forwarded to the redirect target. The probe must
		// refuse to follow (→ unknown), and the target must receive no request.
		var leaked int32
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Vault-Token") != "" {
				atomic.AddInt32(&leaked, 1)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer target.Close()
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/v1/auth/token/lookup-self", http.StatusFound)
		}))
		defer redirector.Close()

		if got := probeTokenAuth(redirector.URL, "secret-root-token"); got != authUnknown {
			t.Errorf("redirect → %v, want authUnknown (not followed)", got)
		}
		if atomic.LoadInt32(&leaked) != 0 {
			t.Error("X-Vault-Token was forwarded to the redirect target — token leak")
		}
	})

	t.Run("transient-then-valid", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable) // first probe transient
				return
			}
			w.WriteHeader(http.StatusOK) // retry sees a valid token
		}))
		defer srv.Close()
		if got := probeTokenAuth(srv.URL, "some-token"); got != authValid {
			t.Errorf("503-then-200 → %v, want authValid after retry", got)
		}
	})
}

// --- signal C: the token the running cb-tumblebug holds ---------------------

// probeContainerToken must isolate the container's token as the culprit only
// when OpenBao itself is ruled out. Every ambiguous answer stays unknown, which
// is non-blocking — folding those into "invalid" is what would misreport a
// server that is merely still starting, or a sealed OpenBao, as a bad token.
func TestProbeContainerTokenStateMapping(t *testing.T) {
	newSrv := func(code int, body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_, _ = w.Write([]byte(body))
		}))
	}
	const healthy = `{"reachable":true,"initialized":true,"sealed":false,"vaultTokenSet":true,"tokenValid":true,"available":true}`
	const rejected = `{"reachable":true,"initialized":true,"sealed":false,"vaultTokenSet":true,"tokenValid":false,"message":"VAULT_TOKEN was rejected by OpenBao"}`
	const noToken = `{"reachable":true,"initialized":true,"sealed":false,"vaultTokenSet":false,"tokenValid":false,"message":"VAULT_TOKEN is not set in the cb-tumblebug environment"}`
	const sealed = `{"reachable":true,"initialized":true,"sealed":true,"vaultTokenSet":true,"tokenValid":false,"message":"OpenBao is sealed"}`
	const uninit = `{"reachable":true,"initialized":false,"sealed":false,"vaultTokenSet":true,"tokenValid":false,"message":"OpenBao is not initialized"}`
	const unreachable = `{"reachable":false,"vaultTokenSet":true,"tokenValid":false,"message":"cannot reach OpenBao"}`

	cases := []struct {
		name string
		code int
		body string
		want containerTokenState
	}{
		{"token accepted", 200, healthy, containerTokenValid},
		{"token rejected", 200, rejected, containerTokenInvalid},
		{"container has no token", 200, noToken, containerTokenInvalid},
		// OpenBao's own fault — signals A/V already diagnose these precisely.
		{"openbao sealed", 200, sealed, containerTokenUnknown},
		{"openbao not initialized", 200, uninit, containerTokenUnknown},
		{"openbao unreachable", 200, unreachable, containerTokenUnknown},
		// Pre-0.12.25 server: no such endpoint. Unknown, never invalid — an old
		// lineup is a lineup problem, not a claim that the token is bad.
		{"endpoint missing (pre-0.12.25)", 404, `{}`, containerTokenUnknown},
		{"server error", 500, `{}`, containerTokenUnknown},
		{"auth failure", 401, `{}`, containerTokenUnknown},
		{"unparsable body", 200, `not-json`, containerTokenUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newSrv(c.code, c.body)
			defer srv.Close()
			got, _ := probeContainerToken(srv.URL, "user", "pass")
			if got != c.want {
				t.Errorf("state = %v, want %v", got, c.want)
			}
		})
	}

	t.Run("cb-tumblebug not listening", func(t *testing.T) {
		srv := newSrv(200, healthy)
		addr := srv.URL
		srv.Close() // nothing is listening now
		if got, _ := probeContainerToken(addr, "user", "pass"); got != containerTokenUnknown {
			t.Errorf("state = %v, want containerTokenUnknown", got)
		}
	})
}

// A stale container token is only reported when every host signal is healthy.
// In any other case the host already holds the more precise diagnosis and the
// container's token is merely wrong as a consequence — reporting it twice would
// send the user after the wrong fix.
func TestDecideReachableContainerStaleToken(t *testing.T) {
	shape := initFileShape{Keys: []string{"k1"}, RootToken: "test-root-token-not-real"}
	healthyHost := Result{Initialized: true, Sealed: false, Active: true, EnvToken: true, TokenValid: true, InitJSON: true}

	t.Run("consistent host + rejected container token → container-stale-token", func(t *testing.T) {
		r := healthyHost
		r.ContainerTokenInvalid = true
		got := r.decideReachable(shape, true)
		if got.Case != CaseContainerStaleToken || got.OK {
			t.Fatalf("Case=%v OK=%v, want container-stale-token / not OK", got.Case, got.OK)
		}
		// The advice must use mayfly's own command. A bare "docker compose ... up -d
		// cb-tumblebug" would run under a different compose project than the one
		// mayfly starts the stack with, and fail on already-taken container names
		// instead of recreating anything.
		if !strings.Contains(got.Advice, "./mayfly infra run -d -s cb-tumblebug") {
			t.Errorf("advice must tell the user to recreate the container with mayfly, got: %q", got.Advice)
		}
		if strings.Contains(got.Advice, "docker compose") {
			t.Errorf("advice must not hand the user a raw compose command (wrong project name), got: %q", got.Advice)
		}
	})

	t.Run("consistent host + healthy container token → consistent", func(t *testing.T) {
		r := healthyHost
		r.ContainerTokenValid = true
		if got := r.decideReachable(shape, true); got.Case != CaseConsistent || !got.OK {
			t.Errorf("Case=%v OK=%v, want consistent / OK", got.Case, got.OK)
		}
	})

	t.Run("consistent host + unknown container signal → consistent (non-blocking)", func(t *testing.T) {
		r := healthyHost // neither flag set = unknown
		if got := r.decideReachable(shape, true); got.Case != CaseConsistent || !got.OK {
			t.Errorf("Case=%v OK=%v, want consistent / OK", got.Case, got.OK)
		}
	})

	t.Run("host fault wins over container signal", func(t *testing.T) {
		for _, c := range []struct {
			name string
			r    Result
			want Case
		}{
			{"wrong-token", Result{Initialized: true, Active: true, EnvToken: true, InitJSON: true}, CaseWrongToken},
			{"stuck-sealed", Result{Initialized: true, Sealed: true, EnvToken: true, InitJSON: true}, CaseStuckSealed},
			{"not-ready", Result{Initialized: true, Active: false, EnvToken: true, TokenValid: true, InitJSON: true}, CaseNotReady},
			{"lost-token", Result{Initialized: true, Active: true, EnvToken: false, InitJSON: true}, CaseLostToken},
		} {
			t.Run(c.name, func(t *testing.T) {
				r := c.r
				r.ContainerTokenInvalid = true // the container is wrong too — as a consequence
				if got := r.decideReachable(shape, true); got.Case != c.want {
					t.Errorf("Case=%v, want %v (host diagnosis must win)", got.Case, c.want)
				}
			})
		}
	})
}
