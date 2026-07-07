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

	"github.com/cm-mayfly/cm-mayfly/common"
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
	CaseUnknown                   // openbao down + disk signals ambiguous → cannot confirm
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
	TokenValid  bool // V
	Sealed      bool
	Advice      string
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

// tokenValid reports whether token authenticates against OpenBao (signal V).
// GET /v1/auth/token/lookup-self returns 200 for a valid token and 403 for a
// stale/wrong one — seal-status alone never validates the token.
func tokenValid(addr, token string) bool {
	if token == "" {
		return false
	}
	c := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(addr, "/")+"/v1/auth/token/lookup-self", nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-Vault-Token", token)
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// startOpenbaoAlone brings up only the openbao service (depends_on flows
// dependent→dependency, so nothing else is pulled in) and waits for its API.
// This is exactly the staged flow Init() already relies on; `infra run` uses it
// to get a definitive diagnosis before deciding whether to bring up the rest.
func startOpenbaoAlone() error {
	up := fmt.Sprintf(
		"COMPOSE_PROJECT_NAME=%s docker compose -f %s up -d openbao",
		common.ComposeProjectName, common.DefaultDockerComposeConfig,
	)
	if err := common.SysCallWithError(up); err != nil {
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
	if r.Initialized && !r.Sealed && r.EnvToken {
		r.TokenValid = tokenValid(openbaoAddr, readEnvToken())
	}
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
	fmt.Fprintf(&b, "  consistency: %s", pf.Case)
	if !pf.OK {
		b.WriteString("\n  ⚠ inconsistent — run './mayfly setup openbao status' for details and remediation")
	}
	return b.String()
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
	case !r.EnvToken && r.InitJSON:
		r.Case = CaseLostToken // storage intact, only .env token lost
	case r.EnvToken && r.TokenValid:
		r.Case, r.OK = CaseConsistent, true
	case r.EnvToken && !r.TokenValid:
		r.Case = CaseWrongToken
	default:
		r.Case = CaseUnknown
	}
	r.Advice = adviceFor(r.Case, shape)
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
	r.Advice = adviceFor(r.Case, initFileShape{})
	return r
}

const (
	envPathHint  = "conf/docker/.env"
	jsonPathHint = "conf/docker/data/openbao/secrets/openbao-init.json"
)

// adviceFor builds the masked, actionable remediation message for a case.
// shape carries the init.json root_token so C5/C7 can show which masked value
// to restore. Secrets are always masked (first 8 chars + ***).
func adviceFor(c Case, shape initFileShape) string {
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
			"⚠ OpenBao data exists on disk but the API reports it is not initialized (possible mount misconfig).\n"+
				"  Check: ./mayfly setup openbao status\n"+
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
	case CaseUnknown:
		return "ℹ OpenBao is not running, so its state can't be confirmed.\n" +
			"  Check: ./mayfly setup openbao status"
	default:
		return ""
	}
}
