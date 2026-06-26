// Package openbao manages the OpenBao secret-manager container that backs
// cb-tumblebug's encrypted credential store.
//
// The cm-mayfly first-run flow matches the upstream cb-tumblebug `make up`
// pattern:
//
//	1. docker compose up -d openbao   (openbao alone — depends_on chain
//	                                   doesn't pull other services in)
//	2. wait for openbao API to respond
//	3. run cb-tumblebug's openbao-init.sh, which writes VAULT_TOKEN into
//	   the shared .env file
//	4. (caller) docker compose up -d   (everything else — these containers
//	                                   now see the populated VAULT_TOKEN)
//
// Keeping Init/Unseal/Status here as plain functions lets `setup openbao`
// commands and `infra run` share one implementation (single source of truth).
package openbao

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cm-mayfly/cm-mayfly/common"
)

const (
	openbaoAddr   = "http://localhost:8200"
	vaultTokenKey = "VAULT_TOKEN"
)

// envPath resolves mayfly's docker .env file relative to the current working
// directory — the same root every other cm-mayfly command assumes.
func envPath() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "conf", "docker", ".env")
}

// HasVaultToken reports whether mayfly's .env has a non-empty VAULT_TOKEN.
// Quoted empty values ("" / '') count as empty.
func HasVaultToken() bool {
	f, err := os.Open(envPath())
	if err != nil {
		return false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if !strings.HasPrefix(line, vaultTokenKey+"=") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, vaultTokenKey+"="))
		return v != "" && v != `""` && v != `''`
	}
	return false
}

// Init runs the official OpenBao initialization flow. When force is false and
// VAULT_TOKEN is already set, Init refuses — re-initializing would generate a
// new unseal key and root token, leaving the existing encrypted data
// inaccessible.
func Init(force bool) error {
	if !force && HasVaultToken() {
		return fmt.Errorf(
			"VAULT_TOKEN is already set in %s\n\n"+
				"Re-initializing OpenBao would generate a NEW unseal key and root token,\n"+
				"and the existing encrypted data (CSP credentials, namespaces, etc.) would\n"+
				"become inaccessible.\n\n"+
				"If you really want to re-initialize, run:\n"+
				"  ./mayfly setup openbao init --force\n",
			envPath())
	}

	fmt.Println("\n[OpenBao initialization]")
	fmt.Println("  Following the official cb-tumblebug 'make up' staged flow:")
	fmt.Println("    1) bring up openbao alone")
	fmt.Println("    2) wait for the API")
	fmt.Println("    3) run openbao-init.sh (writes VAULT_TOKEN into .env)")
	fmt.Println()

	// Step 1 — openbao alone. `docker compose up -d openbao` only brings up
	// the named service; depends_on flows from dependents to dependencies,
	// so nothing else is pulled in.
	//
	// Pre-create the openbao bind-mount path owned by UID 100 (the openbao
	// user inside the official openbao/openbao image). The container's
	// entrypoint runs `chown -R openbao:openbao /openbao/data` but that
	// runs as the openbao user too — it can't fix a root-owned bind-mount
	// path materialized by docker daemon on first run. Pre-fixing on the
	// host is the only reliable workaround that doesn't require changing
	// docker-compose.yaml to run the entrypoint as root.
	openbaoDataDir, _ := filepath.Abs(filepath.Join("conf", "docker", "data", "openbao", "data"))
	if err := exec.Command("sudo", "-n", "mkdir", "-p", openbaoDataDir).Run(); err != nil {
		// Fall back to plain mkdir if sudo isn't usable — better than aborting.
		_ = os.MkdirAll(openbaoDataDir, 0755)
	}
	// 100:100 matches the openbao user/group inside the official image.
	_ = exec.Command("sudo", "-n", "chown", "-R", "100:100", openbaoDataDir).Run()

	fmt.Println("Step 1/3: docker compose up -d openbao")
	upCmd := fmt.Sprintf(
		"COMPOSE_PROJECT_NAME=%s docker compose -f %s up -d openbao",
		common.ComposeProjectName, common.DefaultDockerComposeConfig,
	)
	if err := common.SysCallWithError(upCmd); err != nil {
		return fmt.Errorf("failed to bring up openbao: %w", err)
	}

	// Step 2 — wait until the API answers. Don't insist on "initialized=true"
	// here; that's exactly what step 3 is about.
	fmt.Println("Step 2/3: waiting for OpenBao API to respond...")
	if err := waitOpenbaoAPI(90 * time.Second); err != nil {
		return fmt.Errorf("openbao API never became reachable within 90s: %w", err)
	}
	fmt.Println("  OpenBao API is reachable.")

	// Step 3 — run the upstream init script. It does its own seal-status
	// check, generates an unseal key + root token, persists init.json, and
	// writes VAULT_TOKEN back into ENV_FILE.
	fmt.Println("Step 3/3: running cb-tumblebug's openbao-init.sh")
	cbDir, err := ensureCbTumblebugSource()
	if err != nil {
		return err
	}
	secretsHostDir := filepath.Join("conf", "docker", "data", "openbao", "secrets")
	absSecretsDir, _ := filepath.Abs(secretsHostDir)
	// docker daemon may have created the data/openbao/ parent as root the
	// first time the openbao bind-mount materialized. Reclaim ownership of
	// just the parent dir itself (NOT recursive — recursive would clobber
	// the UID 100 ownership we just set on data/openbao/data, which the
	// openbao container needs to write its keyring). Then ensure the
	// secrets subdir exists and is owned by us (we'll write init.json into
	// it; the openbao-unseal sidecar mounts it read-only as :ro).
	openbaoRoot := filepath.Dir(absSecretsDir) // .../data/openbao
	_ = exec.Command("sudo", "-n", "chown",
		fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		openbaoRoot,
	).Run()
	if err := os.MkdirAll(absSecretsDir, 0755); err != nil {
		if err2 := exec.Command("sudo", "-n", "mkdir", "-p", absSecretsDir).Run(); err2 != nil {
			return fmt.Errorf("failed to create %s: %w (sudo fallback also failed: %v)",
				absSecretsDir, err, err2)
		}
	}
	// Make sure secrets dir is owned by us so openbao-init.sh can write
	// init.json. Recursive is fine here — secrets is a sibling of data,
	// the chown above on openbaoRoot is non-recursive.
	_ = exec.Command("sudo", "-n", "chown", "-R",
		fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		absSecretsDir,
	).Run()
	absEnv, _ := filepath.Abs(envPath())
	initOut := filepath.Join(absSecretsDir, "openbao-init.json")
	script := filepath.Join(cbDir, "init", "openbao", "openbao-init.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("openbao-init.sh not found at %s: %w", script, err)
	}
	_ = os.Chmod(script, 0755)
	runCmd := fmt.Sprintf(
		"cd %q && ENV_FILE=%q INIT_OUTPUT=%q ./init/openbao/openbao-init.sh",
		cbDir, absEnv, initOut,
	)
	if err := common.SysCallWithError(runCmd); err != nil {
		return fmt.Errorf("openbao-init.sh failed: %w", err)
	}

	if !HasVaultToken() {
		return fmt.Errorf(
			"openbao-init.sh finished but VAULT_TOKEN was not written into %s.\n"+
				"This usually means OpenBao was already initialized with different keys\n"+
				"that cm-mayfly does not have access to. Remove the OpenBao data volume\n"+
				"and try again:  docker compose -f %s down -v",
			absEnv, common.DefaultDockerComposeConfig)
	}
	fmt.Println("\n✅ OpenBao initialization completed. VAULT_TOKEN written to .env.")
	return nil
}

// Unseal applies the first unseal key from openbao-init.json to OpenBao on the
// host (localhost). The openbao-unseal sidecar normally does this on every
// container start; this command exists for the case where the sidecar is
// intentionally disabled (KMS auto-unseal trial, manual ops mode).
func Unseal() error {
	fmt.Println("\n[OpenBao unseal]")
	absSecretsDir, _ := filepath.Abs(filepath.Join("conf", "docker", "data", "openbao", "secrets"))
	initOut := filepath.Join(absSecretsDir, "openbao-init.json")
	if _, err := os.Stat(initOut); err != nil {
		return fmt.Errorf("openbao-init.json not found at %s; OpenBao has not been initialized", initOut)
	}
	if err := waitOpenbaoAPI(15 * time.Second); err != nil {
		return fmt.Errorf("openbao API is not reachable: %w", err)
	}
	return UnsealWith(initOut, openbaoAddr)
}

// sealStatus is the subset of GET /v1/sys/seal-status that we act on.
type sealStatus struct {
	Initialized bool `json:"initialized"`
	Sealed      bool `json:"sealed"`
}

// initFileShape models cb-tumblebug's openbao-init.json. The REST
// POST /v1/sys/init flow that openbao-init.sh uses returns "keys" (and
// "keys_base64"); the `bao operator init -format=json` CLI would instead emit
// "unseal_keys_hex"/"unseal_keys_b64". We read whichever is present by parsing
// the file as JSON — this handles pretty-printed files and avoids brittle
// text/regex scraping. /v1/sys/unseal accepts either hex or base64 keys.
type initFileShape struct {
	Keys          []string `json:"keys"`
	KeysBase64    []string `json:"keys_base64"`
	UnsealKeysHex []string `json:"unseal_keys_hex"`
	UnsealKeysB64 []string `json:"unseal_keys_b64"`
}

func (s initFileShape) firstKey() string {
	for _, ks := range [][]string{s.Keys, s.KeysBase64, s.UnsealKeysHex, s.UnsealKeysB64} {
		if len(ks) > 0 && ks[0] != "" {
			return ks[0]
		}
	}
	return ""
}

// UnsealWith unseals the OpenBao reachable at addr using the first unseal key
// found in initFile. Both the seal-status response and the init file are parsed
// as JSON natively (no curl/grep/python), so it behaves identically whether
// invoked on the host (`setup openbao unseal`) or from inside the
// openbao-unseal sidecar container, which passes container paths via --file and
// --addr. It is safe to call repeatedly: when OpenBao is already unsealed it
// returns nil without acting (and without logging), which is exactly what the
// sidecar's poll loop relies on for a quiet steady state.
func UnsealWith(initFile, addr string) error {
	st, err := fetchSealStatus(addr)
	if err != nil {
		return fmt.Errorf("OpenBao seal-status not reachable at %s: %w", addr, err)
	}
	if !st.Initialized {
		return fmt.Errorf("OpenBao at %s is not initialized; run 'setup openbao init' first", addr)
	}
	if !st.Sealed {
		return nil // already unsealed — nothing to do
	}
	raw, err := os.ReadFile(initFile)
	if err != nil {
		return fmt.Errorf("cannot read unseal key file %s: %w", initFile, err)
	}
	var shape initFileShape
	if err := json.Unmarshal(raw, &shape); err != nil {
		return fmt.Errorf("cannot parse %s as JSON: %w", initFile, err)
	}
	key := shape.firstKey()
	if key == "" {
		return fmt.Errorf("no unseal key found in %s (looked for keys/keys_base64/unseal_keys_hex/unseal_keys_b64)", initFile)
	}
	if err := postUnseal(addr, key); err != nil {
		return fmt.Errorf("unseal request to %s failed: %w", addr, err)
	}
	fmt.Println("OpenBao is now unsealed.")
	return nil
}

func fetchSealStatus(addr string) (sealStatus, error) {
	var st sealStatus
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get(strings.TrimRight(addr, "/") + "/v1/sys/seal-status")
	if err != nil {
		return st, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(body, &st); err != nil {
		return st, fmt.Errorf("unexpected seal-status response: %w", err)
	}
	return st, nil
}

func postUnseal(addr, key string) error {
	c := &http.Client{Timeout: 10 * time.Second}
	payload, _ := json.Marshal(map[string]string{"key": key})
	resp, err := c.Post(strings.TrimRight(addr, "/")+"/v1/sys/unseal", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var st sealStatus
	if err := json.Unmarshal(body, &st); err == nil && st.Sealed {
		return fmt.Errorf("OpenBao still sealed after applying key (multi-key unseal threshold?)")
	}
	return nil
}

// StatusInfo is a snapshot of the OpenBao API state and how well the running
// containers reflect the .env VAULT_TOKEN. It's intended for a one-line
// human-readable summary, not a machine API.
type StatusInfo struct {
	EnvTokenSet        bool
	EnvTokenMasked     string
	OpenbaoReachable   bool
	OpenbaoInitialized bool
	OpenbaoSealed      bool
	TumblebugTokenSet  bool
	TerrariumTokenSet  bool
	Notes              []string
}

// Status returns a snapshot suitable for the `setup openbao status` command.
func Status() StatusInfo {
	var st StatusInfo
	if HasVaultToken() {
		st.EnvTokenSet = true
		st.EnvTokenMasked = maskToken(readEnvToken())
	} else {
		st.EnvTokenMasked = "(empty)"
	}
	out := common.SysCallWithOutput(
		fmt.Sprintf(`curl -sf %s/v1/sys/seal-status 2>/dev/null`, openbaoAddr),
	)
	if out != "" {
		st.OpenbaoReachable = true
		st.OpenbaoInitialized = strings.Contains(out, `"initialized":true`)
		st.OpenbaoSealed = strings.Contains(out, `"sealed":true`)
	}
	st.TumblebugTokenSet = containerHasVaultToken("cb-tumblebug")
	st.TerrariumTokenSet = containerHasVaultToken("mc-terrarium")
	if st.EnvTokenSet && !st.TumblebugTokenSet {
		st.Notes = append(st.Notes,
			"cb-tumblebug has empty VAULT_TOKEN although .env has one — the container "+
				"was started before .env was populated. Recreate it: "+
				"docker compose -f "+common.DefaultDockerComposeConfig+" up -d cb-tumblebug.")
	}
	if st.EnvTokenSet && !st.TerrariumTokenSet {
		st.Notes = append(st.Notes,
			"mc-terrarium has empty VAULT_TOKEN although .env has one — same fix applies as cb-tumblebug.")
	}
	if st.OpenbaoReachable && !st.OpenbaoInitialized {
		st.Notes = append(st.Notes,
			"OpenBao is reachable but not initialized. Run: ./mayfly setup openbao init")
	}
	if st.OpenbaoReachable && st.OpenbaoSealed {
		st.Notes = append(st.Notes,
			"OpenBao is sealed. The openbao-unseal sidecar normally unseals it; "+
				"if the sidecar is disabled, run: ./mayfly setup openbao unseal")
	}
	return st
}

func waitOpenbaoAPI(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		out := common.SysCallWithOutput(
			fmt.Sprintf(`curl -sf %s/v1/sys/seal-status 2>/dev/null`, openbaoAddr),
		)
		if out != "" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

// ensureCbTumblebugSource returns the path to a cb-tumblebug git checkout
// matching the image tag in docker-compose.yaml, cloning it on demand.
func ensureCbTumblebugSource() (string, error) {
	version, err := tumblebugVersionFromCompose()
	if err != nil {
		return "", fmt.Errorf("could not read cb-tumblebug version from docker-compose.yaml: %w", err)
	}
	gitTag := "v" + version
	targetDir := filepath.Join(os.Getenv("HOME"), "go", "src", "github.com", "cloud-barista")
	cbDir := filepath.Join(targetDir, "cb-tumblebug")
	scriptPath := filepath.Join(cbDir, "init", "openbao", "openbao-init.sh")
	if _, err := os.Stat(scriptPath); err == nil {
		return cbDir, nil
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}
	clone := fmt.Sprintf("cd %q && git clone -b %s https://github.com/cloud-barista/cb-tumblebug.git", targetDir, gitTag)
	fmt.Printf("  Cloning cb-tumblebug %s into %s ...\n", gitTag, cbDir)
	if err := common.SysCallWithError(clone); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("openbao-init.sh still missing after clone: %w", err)
	}
	return cbDir, nil
}

func tumblebugVersionFromCompose() (string, error) {
	f, err := os.Open(common.DefaultDockerComposeConfig)
	if err != nil {
		return "", err
	}
	defer f.Close()
	re := regexp.MustCompile(`cloudbaristaorg/cb-tumblebug:([0-9]+\.[0-9]+\.[0-9]+)`)
	s := bufio.NewScanner(f)
	for s.Scan() {
		if m := re.FindStringSubmatch(s.Text()); len(m) > 1 {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("cb-tumblebug image tag not found in %s", common.DefaultDockerComposeConfig)
}

func readEnvToken() string {
	f, err := os.Open(envPath())
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, vaultTokenKey+"=") {
			return strings.TrimSpace(strings.TrimPrefix(line, vaultTokenKey+"="))
		}
	}
	return ""
}

func maskToken(tok string) string {
	if tok == "" {
		return "(empty)"
	}
	if len(tok) <= 8 {
		return "***"
	}
	return tok[:8] + "***"
}

func containerHasVaultToken(container string) bool {
	cmdStr := fmt.Sprintf(`docker exec %s sh -c 'printf "%%s" "$VAULT_TOKEN"' 2>/dev/null`, container)
	out := common.SysCallWithOutput(cmdStr)
	return strings.TrimSpace(out) != ""
}
