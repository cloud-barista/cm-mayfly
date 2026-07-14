package openbao

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cm-mayfly/cm-mayfly/common"
)

// Signal C — "does the token the running cb-tumblebug container holds actually
// work?"
//
// Since cb-tumblebug 0.12.25 the server registers CSP credentials into OpenBao
// itself, using the VAULT_TOKEN from its own container environment, which it
// reads once at startup. A container started before the current token therefore
// keeps an old one, and credential registration fails silently while every
// host-side signal (T/J/D/A/V) still looks healthy — the host cannot see inside
// the container.
//
// cb-tumblebug's GET /credential/openbaoStatus closes exactly that gap: the
// server performs lookup-self with its own token and reports the outcome.
const tumblebugAddr = "http://localhost:1323/tumblebug"

// containerTokenState is deliberately tri-state, like the host token probe:
// folding "cannot tell" into "invalid" would misreport a container that is
// merely still starting up.
type containerTokenState int

const (
	containerTokenUnknown containerTokenState = iota // could not tell → non-blocking
	containerTokenValid                              // the container's token was accepted
	containerTokenInvalid                            // OpenBao is fine, but the container's token is not
)

// openbaoStatusInfo mirrors cb-tumblebug's model.OpenBaoStatusInfo.
type openbaoStatusInfo struct {
	Reachable     bool   `json:"reachable"`
	Initialized   bool   `json:"initialized"`
	Sealed        bool   `json:"sealed"`
	VaultTokenSet bool   `json:"vaultTokenSet"`
	TokenValid    bool   `json:"tokenValid"`
	Available     bool   `json:"available"`
	Message       string `json:"message"`
}

// probeContainerToken asks cb-tumblebug whether the token it holds is accepted
// by OpenBao.
//
// It returns containerTokenInvalid ONLY when the answer isolates the container's
// token as the problem: OpenBao is reachable, initialized and unsealed, yet the
// token is missing or rejected. When the same response says OpenBao itself is
// down, uninitialized or sealed, the verdict is unknown — that is not a
// container-token fault, and signals A/V already diagnose it precisely. Calling
// it here too would report one fault under two names and advise the wrong fix.
//
// A 404 (cb-tumblebug older than 0.12.25) is likewise unknown, not invalid: the
// lineup pins 0.12.25+, so an older server is a lineup problem, not a reason to
// claim the token is bad. There is no compatibility fallback — see the design
// note on why silently degrading here would hide the very failure this signal
// exists to catch.
func probeContainerToken(addr, user, pass string) (containerTokenState, openbaoStatusInfo) {
	var info openbaoStatusInfo

	c := &http.Client{
		Timeout: 10 * time.Second,
		// The request carries Basic credentials; do not let a misconfigured or
		// hostile endpoint redirect them elsewhere. Go strips Authorization on a
		// cross-host redirect, but returning the 3xx as-is (→ unknown) is simpler
		// to reason about than depending on that.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(addr, "/")+"/credential/openbaoStatus", nil)
	if err != nil {
		return containerTokenUnknown, info
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := c.Do(req)
	if err != nil {
		return containerTokenUnknown, info // not up yet / network error
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return containerTokenUnknown, info // 404 on pre-0.12.25, 5xx while starting, auth failure
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return containerTokenUnknown, info
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return containerTokenUnknown, info
	}

	if info.TokenValid {
		return containerTokenValid, info
	}
	// Only blame the container's token once OpenBao itself is ruled out.
	if info.Reachable && info.Initialized && !info.Sealed {
		return containerTokenInvalid, info
	}
	return containerTokenUnknown, info
}

// tumblebugHealthy reports whether the cb-tumblebug container is running and has
// passed its healthcheck. This is the readiness gate for signal C: a container
// that is still starting has not read VAULT_TOKEN yet (or serves nothing), and
// probing it would produce noise, not a verdict.
func tumblebugHealthy() bool {
	out := common.SysCallWithOutput(
		`docker inspect -f '{{.State.Running}}:{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' cb-tumblebug 2>/dev/null`,
	)
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "true:") {
		return false
	}
	health := strings.TrimPrefix(out, "true:")
	// "none" = no healthcheck defined; running is the best signal available.
	return health == "healthy" || health == "none"
}

// containerTokenAdvice is the remediation for container-stale-token. Recreating
// the container re-reads .env and costs no data, but mayfly still only advises:
// a preflight that acts on a wrong verdict is exactly what the guide-only policy
// exists to prevent.
//
// The command has to be the mayfly one. A bare "docker compose -f <file> up -d
// cb-tumblebug" does NOT work: the compose file declares no project name, so
// compose derives one from the directory it is invoked in, while mayfly always
// runs compose as COMPOSE_PROJECT_NAME=cloud-migrator. The user's compose would
// therefore not recognise the running stack at all and would fail trying to
// create containers whose names are already taken.
func containerTokenAdvice() string {
	return fmt.Sprintf(
		"cb-tumblebug is running with a stale VAULT_TOKEN.\n"+
			"  The token in %s is valid, but the one the container holds is not.\n"+
			"  cb-tumblebug reads VAULT_TOKEN once at startup, so a container started before the\n"+
			"  current token keeps the old one — credential registration then fails silently.\n"+
			"  Recreate it (no data loss):\n"+
			"    ./mayfly infra run -d -s cb-tumblebug",
		envPath())
}
