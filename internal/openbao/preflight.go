package openbao

// preflight.go implements the OpenBao state-consistency check shared by
// `infra run`, `setup tumblebug-init`, and `setup openbao status`. It is a
// detection/diagnosis layer only: it NEVER writes .env or destroys data. When a
// mismatch is found it returns a human-readable Advice telling the user exactly
// which file/field to change, to what, and which command to run next.
//
// Keeping this in internal/openbao (next to Init/Unseal/Status) means all three
// entry points share one judgement — the single source of truth that prevents
// the "silent deadlock" a broken OpenBao state can cause.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Case enumerates the OpenBao state-consistency verdicts. See the state-signal
// model (T/J/D/A/V) for the full matrix.
type Case int

const (
	CaseFresh         Case = iota // C1: no token, no storage → normal first install
	CaseConsistent                // C2: token + storage + initialized + valid token
	CaseOrphanedToken             // C3: token present but storage wiped
	CaseStaleInitJSON             // C4: init.json present but storage wiped — untrustworthy
	CaseLostToken                 // C5: storage + init.json intact, .env token only lost
	CaseCorrupt                   // C6: storage present but API reports not-initialized
	CaseWrongToken                // C7: token present but authentication fails (403)
	CaseStuckSealed               // C8: initialized but stays sealed (unseal key mismatch)
	CaseNotReady                  // unsealed but the API never became active within the timeout — a transition/infra state, NOT a token problem
	CaseUnknown                   // openbao down + disk signals ambiguous → cannot confirm
	// C9: every host signal is healthy, but the running cb-tumblebug holds a
	// token OpenBao rejects. Since 0.12.25 the server registers credentials with
	// the token from its own env, read once at startup — so a container started
	// before the current token fails registration silently while the host still
	// looks consistent.
	CaseContainerStaleToken
)

// String returns a short label for status output / logs.
func (c Case) String() string {
	switch c {
	case CaseFresh:
		return "fresh"
	case CaseConsistent:
		return "consistent"
	case CaseOrphanedToken:
		return "orphaned-token"
	case CaseStaleInitJSON:
		return "stale-init.json"
	case CaseLostToken:
		return "lost-token"
	case CaseCorrupt:
		return "corrupt"
	case CaseWrongToken:
		return "wrong-token"
	case CaseStuckSealed:
		return "stuck-sealed"
	case CaseNotReady:
		return "not-ready"
	case CaseContainerStaleToken:
		return "container-stale-token"
	default:
		return "unknown"
	}
}

// Result is the outcome of Preflight. OK is true only for the two states that
// are safe to proceed on unchanged (C1 fresh, C2 consistent); every other case
// carries a populated Advice and should stop the caller.
type Result struct {
	Case      Case
	OK        bool
	Reachable bool
	// raw signals (surfaced by `setup openbao status`)
	EnvToken    bool // T
	InitJSON    bool // J
	DataDir     bool // D
	Initialized bool // A
	TokenValid  bool // V: confirmed valid (200 on lookup-self)
	// TokenUnknown means token validity could NOT be confirmed — the probe hit a
	// residual transient (5xx / timeout / connection error) after retry. It is
	// deliberately distinct from "invalid": readiness (unseal+active) is
	// guaranteed upstream, so an unconfirmed token is treated as non-blocking,
	// never as CaseWrongToken.
	TokenUnknown bool // V
	// Active is the readiness signal: GET /v1/sys/health returned 200 (active).
	// Only meaningful when Initialized && !Sealed. False here (with a reachable,
	// unsealed openbao) means the API is still transitioning → CaseNotReady.
	Active bool
	Sealed bool
	// Signal C: does the token the RUNNING cb-tumblebug holds still authenticate?
	// Answered by cb-tumblebug's own openbaoStatus (it runs lookup-self with its
	// container-env token). Tri-state like V, but encoded so that the zero value
	// is "unknown": neither flag set means we could not tell (server not healthy
	// yet, transient error, or the response blamed OpenBao itself rather than the
	// token), which is never blocking. Only ContainerTokenInvalid blocks.
	ContainerTokenValid   bool // C
	ContainerTokenInvalid bool // C
	// Note is an informational, non-blocking message surfaced even when OK is
	// true (e.g. "token validity could not be confirmed, proceeding").
	Note   string
	Advice string
}

// openbao host-side paths, resolved relative to the mayfly working directory —
// the same root every other cm-mayfly command assumes.
func openbaoInitFilePath() string {
	return filepath.Join("conf", "docker", "data", "openbao", "secrets", "openbao-init.json")
}

func openbaoDataDirPath() string {
	return filepath.Join("conf", "docker", "data", "openbao", "data")
}

// readInitFile parses openbao-init.json. ok is true only when the file parses
// and carries at least one usable unseal key (signal J).
func readInitFile() (shape initFileShape, ok bool) {
	raw, err := os.ReadFile(openbaoInitFilePath())
	if err != nil {
		return shape, false
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return shape, false
	}
	return shape, shape.firstKey() != ""
}

// dataDirState reports whether the openbao storage directory holds data
// (signal D). known is false when the directory can't be read (e.g. it is owned
// by UID 100 with tight perms) — in that case callers must NOT treat it as
// empty, to avoid blocking a healthy environment on a false negative.
func dataDirState() (populated, known bool) {
	entries, err := os.ReadDir(openbaoDataDirPath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, true // dir absent → definitively empty
		}
		return false, false // permission/other error → cannot tell
	}
	return len(entries) > 0, true
}

// tokenAuthState is the tri-state verdict of a token-validity probe. Because
// readiness (unseal + active) is guaranteed upstream (see waitOpenbaoActive), a
// non-200 here is either a genuine auth failure (authInvalid) or a residual
// transient we could not resolve (authUnknown) — the two must never collapse to
// the same "false" the old boolean returned, which mis-flagged a transition-
// window 503 as a wrong token.
type tokenAuthState int

const (
	authUnknown tokenAuthState = iota // transient (5xx / 429 / timeout / connection error) — do NOT treat as invalid
	authValid                         // 200 — token authenticates
	authInvalid                       // 401 / 403 (or empty token) — genuine auth failure
)

// probeTokenAuth reports whether token authenticates against OpenBao (signal V)
// via GET /v1/auth/token/lookup-self. It returns a definitive valid/invalid on
// a 200/401/403, and — as a second line of defence for any transient that slips
// past the upstream readiness gate — retries briefly on 5xx/timeout/connection
// errors before giving up as authUnknown (never authInvalid).
func probeTokenAuth(addr, token string) tokenAuthState {
	if token == "" {
		return authInvalid // no token can never authenticate
	}
	const attempts = 3
	for i := 0; i < attempts; i++ {
		if state, done := probeTokenOnce(addr, token); done {
			return state
		}
		if i < attempts-1 {
			time.Sleep(1 * time.Second)
		}
	}
	return authUnknown
}

// probeTokenOnce runs a single lookup-self probe. done is true only for a
// definitive verdict (200 → valid, 401/403 → invalid); every other outcome
// (5xx/429, timeout, connection error) returns done=false so the caller retries.
func probeTokenOnce(addr, token string) (state tokenAuthState, done bool) {
	// Never follow redirects: this request carries the root token in the
	// X-Vault-Token header, which Go does NOT strip on a cross-host redirect
	// (it only strips Authorization/Cookie). A compromised or misconfigured
	// endpoint answering 3xx must not be able to forward the token elsewhere.
	// ErrUseLastResponse returns the 3xx as-is (→ treated as a transient, not
	// a leak). openbao's API never legitimately redirects these endpoints.
	c := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(addr, "/")+"/v1/auth/token/lookup-self", nil)
	if err != nil {
		return authUnknown, true // malformed request won't fix on retry; not an auth failure either
	}
	req.Header.Set("X-Vault-Token", token)
	resp, err := c.Do(req)
	if err != nil {
		return authUnknown, false // connection error / timeout → transient, retry
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return authValid, true
	case http.StatusUnauthorized, http.StatusForbidden:
		return authInvalid, true
	default:
		return authUnknown, false // 5xx / 429 / etc → transient, retry
	}
}

// waitOpenbaoActive polls GET /v1/sys/health until it returns 200 (initialized,
// unsealed, AND active) or the timeout elapses. This is the readiness gate that
// must clear BEFORE probeTokenAuth: right after startOpenbaoAlone+UnsealWith,
// OpenBao spends a short window loading its mount table / settling leadership
// during which it still answers 503 (or 429 on a standby) — health 200, not
// seal-status' sealed:false, is the correct "ready to serve" signal. Bounding
// the wait keeps a genuinely stuck API from hanging the caller.
func waitOpenbaoActive(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	c := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	url := strings.TrimRight(addr, "/") + "/v1/sys/health"
	for {
		if resp, err := c.Get(url); err == nil {
			code := resp.StatusCode
			_ = resp.Body.Close()
			if code == http.StatusOK {
				return nil // active
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("openbao did not become active (health 200) within %s", timeout)
		}
		time.Sleep(1 * time.Second)
	}
}

// startOpenbaoAlone brings up only the openbao service (depends_on flows
// dependent→dependency, so nothing else is pulled in) and waits for its API.
// This is exactly the staged flow Init() already relies on; `infra run` uses it
// to get a definitive diagnosis before deciding whether to bring up the rest.
func startOpenbaoAlone() error {
	if err := composeUpOpenbao(); err != nil {
		return err
	}
	return waitOpenbaoAPI(60 * time.Second)
}

// Preflight collects the state signals and returns a verdict. allowStartOpenbao
// lets a caller that is going to start containers anyway (`infra run`) bring up
// openbao alone to obtain an authoritative API judgement; read-only callers
// (`setup openbao status`) pass false and get a disk-based provisional verdict
// when openbao is down.
func Preflight(allowStartOpenbao bool) Result {
	var r Result
	r.EnvToken = HasVaultToken()
	shape, jOK := readInitFile()
	r.InitJSON = jOK
	dPop, dKnown := dataDirState()
	r.DataDir = dPop

	st, err := fetchSealStatus(openbaoAddr)
	reachable := err == nil

	// Start openbao only when there is something to diagnose — a token in .env
	// or data already on disk. A truly fresh install (no token, no data) stays
	// unreachable here and the caller takes its normal staged auto-init path,
	// so we don't start a container needlessly.
	if !reachable && allowStartOpenbao && (r.EnvToken || dPop) {
		if startErr := startOpenbaoAlone(); startErr == nil {
			if st2, err2 := fetchSealStatus(openbaoAddr); err2 == nil {
				st, reachable = st2, true
			}
		}
	}
	r.Reachable = reachable

	if !reachable {
		return r.decideDisk(dPop, dKnown)
	}

	r.Initialized = st.Initialized
	r.Sealed = st.Sealed
	// openbao started alone is not auto-unsealed (the openbao-unseal sidecar
	// isn't pulled in). If it is initialized+sealed, try the init.json key so
	// we can (a) check token validity and (b) distinguish a genuine stuck seal
	// from a normal sealed-on-start. Unsealing is non-destructive.
	if st.Initialized && st.Sealed && jOK {
		_ = UnsealWith(openbaoInitFilePath(), openbaoAddr)
		if st2, err2 := fetchSealStatus(openbaoAddr); err2 == nil {
			r.Sealed = st2.Sealed
		}
	}
	// FR-01 readiness gate: once initialized+unsealed, wait for the API to become
	// active (health 200) BEFORE judging the token. openbao started/unsealed just
	// above still answers 503 during its short mount-table/leadership transition;
	// gating here absorbs that as infra readiness so it can never leak into
	// tokenValid and be mistaken for a wrong token. If the stack is already up and
	// active, the first health probe returns immediately (no added latency).
	if r.Initialized && !r.Sealed {
		// A cold start (run, which just brought openbao up) needs headroom for the
		// mount-table/leadership transition; a read-only caller (status/info) that
		// finds openbao already up should not hang 30s when it is up-but-not-active,
		// so it uses a short bound and reports not-ready instead.
		gateTimeout := 8 * time.Second
		if allowStartOpenbao {
			gateTimeout = 30 * time.Second
		}
		r.Active = waitOpenbaoActive(openbaoAddr, gateTimeout) == nil
		if r.Active && r.EnvToken {
			switch probeTokenAuth(openbaoAddr, readEnvToken()) {
			case authValid:
				r.TokenValid = true
			case authInvalid:
				// confirmed invalid — both flags stay false → CaseWrongToken
			case authUnknown:
				r.TokenUnknown = true // could not confirm → non-blocking
			}
		}
	}
	r = r.probeContainerSignal()
	return r.decideReachable(shape, dPop)
}

// CompactStatus returns a short, multi-line OpenBao consistency summary suitable
// for embedding in `infra info`. It is read-only (never starts openbao) and,
// when the state is inconsistent, points at `setup openbao status` for the full
// remediation rather than dumping the whole advice into the info output.
func CompactStatus() string {
	pf := Preflight(false)
	tok := "(empty)"
	if pf.EnvToken {
		tok = maskToken(readEnvToken())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  API        : reachable=%v initialized=%v sealed=%v\n", pf.Reachable, pf.Initialized, pf.Sealed)
	fmt.Fprintf(&b, "  .env token : %s\n", tok)
	if pf.TokenUnknown {
		fmt.Fprintf(&b, "  token      : validity unconfirmed (transient API error)\n")
	}
	fmt.Fprintf(&b, "  consistency: %s", pf.Case)
	if !pf.OK {
		b.WriteString("\n  ⚠ inconsistent — run './mayfly setup openbao status' for details and remediation")
	}
	return b.String()
}

// probeContainerSignal fills signal C. The readiness gate comes first: an
// unhealthy or absent cb-tumblebug has not read VAULT_TOKEN yet, so asking it
// would yield noise rather than a verdict.
func (r Result) probeContainerSignal() Result {
	if !tumblebugHealthy() {
		return r // unknown
	}
	switch state, _ := probeContainerToken(tumblebugAddr, readEnvValue("TB_API_USERNAME"), readEnvValue("TB_API_PASSWORD")); state {
	case containerTokenValid:
		r.ContainerTokenValid = true
	case containerTokenInvalid:
		r.ContainerTokenInvalid = true
	}
	return r
}

// decideReachable judges from the authoritative API signals (openbao is up).
func (r Result) decideReachable(shape initFileShape, dPop bool) Result {
	switch {
	case !r.Initialized:
		switch {
		case dPop:
			r.Case = CaseCorrupt // data on disk but API says not initialized
		case r.EnvToken:
			r.Case = CaseOrphanedToken // storage gone, stale token left
		default:
			r.Case, r.OK = CaseFresh, true
		}
	case r.Sealed:
		r.Case = CaseStuckSealed // initialized but couldn't be unsealed
	case !r.Active:
		// Unsealed but the API never became active within the readiness window —
		// a transition/infra state, not a token problem. Must be judged before any
		// token case so a transient can't be read as a wrong token.
		r.Case = CaseNotReady
	case !r.EnvToken && r.InitJSON:
		r.Case = CaseLostToken // storage intact, only .env token lost
	case r.EnvToken && r.TokenValid:
		r.Case, r.OK = CaseConsistent, true
	case r.EnvToken && r.TokenUnknown:
		// Token validity could not be confirmed (residual transient). Readiness is
		// already cleared upstream, so this is a healthy stack we simply couldn't
		// re-verify — proceed, but surface a note rather than silently claiming OK.
		r.Case, r.OK = CaseConsistent, true
		r.Note = "ℹ OpenBao token validity could not be confirmed (transient API error); proceeding.\n" +
			"  Re-check with: ./mayfly setup openbao status"
	case r.EnvToken && !r.TokenValid:
		r.Case = CaseWrongToken // confirmed 401/403 only reaches here
	default:
		r.Case = CaseUnknown
	}
	// Signal C is only consulted once the host signals say everything is fine.
	// In every other case the host already has the more precise diagnosis, and
	// the container's token is merely wrong as a consequence — reporting one
	// fault under two names would send the user after the wrong fix.
	if r.Case == CaseConsistent && r.ContainerTokenInvalid {
		r.Case, r.OK, r.Note = CaseContainerStaleToken, false, ""
	}
	r.Advice = adviceFor(r.Case, shape, true)
	return r
}

// decideDisk judges from disk signals only (openbao is down and could not be
// started). It never hard-fails on ambiguity — unknown states point the user at
// `setup openbao status` rather than blocking.
func (r Result) decideDisk(dPop, dKnown bool) Result {
	empty := dKnown && !dPop
	switch {
	case !r.EnvToken && !r.InitJSON && !dPop:
		r.Case, r.OK = CaseFresh, true
	case r.EnvToken && r.InitJSON && empty:
		r.Case = CaseStaleInitJSON
	case r.EnvToken && !r.InitJSON && empty:
		r.Case = CaseOrphanedToken
	default:
		r.Case = CaseUnknown
	}
	r.Advice = adviceFor(r.Case, initFileShape{}, false)
	return r
}

const (
	envPathHint  = "conf/docker/.env"
	jsonPathHint = "conf/docker/data/openbao/secrets/openbao-init.json"
)

// adviceFor builds the masked, actionable remediation message for a case.
// shape carries the init.json root_token so C5/C7 can show which masked value
// to restore. Secrets are always masked (first 8 chars + ***). reachable lets
// the ambiguous CaseUnknown message distinguish "openbao is running but its
// signals are unclear" from the disk-path "openbao is not running".
func adviceFor(c Case, shape initFileShape, reachable bool) string {
	rootTok := maskToken(shape.RootToken)
	envTok := maskToken(readEnvToken())
	switch c {
	case CaseOrphanedToken:
		return fmt.Sprintf(
			"⚠ OpenBao storage is empty but .env still has a stale VAULT_TOKEN (%s), so "+
				"auto-init is skipped and services won't start.\n"+
				"  Fix: clear VAULT_TOKEN in %s, then run:\n"+
				"    ./mayfly infra run -d          (empty token → OpenBao auto re-init)\n"+
				"  or: ./mayfly infra remove --clean-all  then  ./mayfly infra run -d",
			envTok, envPathHint)
	case CaseStaleInitJSON:
		return fmt.Sprintf(
			"⚠ OpenBao storage is empty but a stale init.json remains — its keys/token are no longer valid.\n"+
				"  Fix: clear VAULT_TOKEN in %s, delete (or back up) %s, then run:\n"+
				"    ./mayfly infra run -d",
			envPathHint, jsonPathHint)
	case CaseLostToken:
		return fmt.Sprintf(
			"ℹ OpenBao storage and init.json are intact, but VAULT_TOKEN in %s is empty.\n"+
				"  Restore: copy the \"root_token\" (%s) from %s into VAULT_TOKEN in %s, then run:\n"+
				"    ./mayfly infra run -d          (existing data is reused as-is)",
			envPathHint, rootTok, jsonPathHint, envPathHint)
	case CaseCorrupt:
		return fmt.Sprintf(
			"⚠ OpenBao data exists on disk but the API reports it is not initialized (possible mount misconfig).\n" +
				"  Check: ./mayfly setup openbao status\n" +
				"  If the data is truly unusable: ./mayfly infra remove --clean-all  then re-init.")
	case CaseWrongToken:
		return fmt.Sprintf(
			"⚠ VAULT_TOKEN in %s (%s) fails OpenBao authentication (403 — stale/invalid).\n"+
				"  Restore: replace VAULT_TOKEN with the \"root_token\" (%s) from %s, then run:\n"+
				"    ./mayfly infra run -d\n"+
				"  (If the init.json token is also invalid, keys/data are mismatched → "+
				"./mayfly setup openbao init --force, or ./mayfly infra remove --clean-all then re-init.)",
			envPathHint, envTok, rootTok, jsonPathHint)
	case CaseStuckSealed:
		return fmt.Sprintf(
			"⚠ OpenBao stays sealed — the unseal key in init.json may not match the data.\n"+
				"  Check %s; if there is no matching backup, run ./mayfly infra remove --clean-all then re-init.",
			jsonPathHint)
	case CaseContainerStaleToken:
		return "⚠ " + containerTokenAdvice()
	case CaseNotReady:
		return "ℹ OpenBao is unsealed but its API has not become active yet (still loading / settling).\n" +
			"  This is usually transient — wait a few seconds and re-run, or check: ./mayfly setup openbao status"
	case CaseUnknown:
		if reachable {
			return "ℹ OpenBao is running but its state signals are ambiguous — consistency can't be confirmed.\n" +
				"  Check: ./mayfly setup openbao status"
		}
		return "ℹ OpenBao is not running, so its state can't be confirmed.\n" +
			"  Check: ./mayfly setup openbao status"
	default:
		return ""
	}
}
