// Package openbao manages the OpenBao secret-manager container that backs
// cb-tumblebug's encrypted credential store.
//
// The cm-mayfly first-run flow matches the upstream cb-tumblebug `make up`
// pattern:
//
//  1. docker compose up -d openbao   (openbao alone — depends_on chain
//     doesn't pull other services in)
//  2. wait for openbao API to respond
//  3. run cb-tumblebug's openbao-init.sh, which writes VAULT_TOKEN into
//     the shared .env file
//  4. (caller) docker compose up -d   (everything else — these containers
//     now see the populated VAULT_TOKEN)
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
// A value that is nothing but a pair of quotes counts as empty — ParseDotEnv
// strips the quotes, so both the double- and single-quoted forms reduce to the
// empty string here.
func HasVaultToken() bool {
	return readEnvValue(vaultTokenKey) != ""
}

// composeUpOpenbao brings up the openbao service on its own. `docker compose up
// -d openbao` only starts the named service — depends_on flows from dependents
// to dependencies, so nothing else is pulled in.
//
// Init() and the preflight's startOpenbaoAlone() both need exactly this, and
// both used to assemble their own `COMPOSE_PROJECT_NAME=… docker compose …`
// string for /bin/sh. One copy, executed as an argument vector, means the
// compose file path is passed to the kernel literally instead of being re-parsed
// by a shell, and there is no second copy to drift.
func composeUpOpenbao() error {
	return common.RunCommand(
		"docker",
		[]string{"compose", "-f", common.DefaultDockerComposeConfig, "up", "-d", "openbao"},
		[]string{"COMPOSE_PROJECT_NAME=" + common.ComposeProjectName},
	)
}

// warnOnFailure prints what went wrong and why it matters, and carries on.
//
// The permission fixes in Init used to discard their results with `_ =`. Most of
// them are best-effort by design: sudo may not be usable without a password, and
// the directories are frequently already owned correctly, in which case the
// failure is harmless and aborting would break a setup that works today. But
// when one of them does matter, the consequence used to appear much later and in
// an unrelated place — typically as OpenBao failing to write its keyring — with
// nothing to connect it back to the chown that never ran. Naming the failure at
// the moment it happens costs nothing and removes that guessing.
//
// Only failures that make the rest of the flow impossible are returned as errors
// (see the mkdir fallbacks); everything else comes through here.
func warnOnFailure(err error, what, consequence string) {
	if err == nil {
		return
	}
	fmt.Printf("  warn: %s: %v\n", what, err)
	if consequence != "" {
		fmt.Printf("        %s\n", consequence)
	}
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
	// #nosec G204 -- fixed argv; openbaoDataDir is an absolute path built from repo-relative constants, and no shell parses it
	if err := exec.Command("sudo", "-n", "mkdir", "-p", openbaoDataDir).Run(); err != nil {
		// Fall back to plain mkdir if sudo isn't usable — better than aborting.
		// This one is not optional: without the directory there is nothing for
		// the container to bind-mount, so a failure here is reported rather than
		// left to surface later as an unexplained openbao startup error.
		if err2 := os.MkdirAll(openbaoDataDir, 0700); err2 != nil {
			return fmt.Errorf("failed to create %s: %w (sudo fallback also failed: %v)",
				openbaoDataDir, err2, err)
		}
	}
	// 100:100 matches the openbao user/group inside the official image.
	warnOnFailure(
		exec.Command("sudo", "-n", "chown", "-R", "100:100", openbaoDataDir).Run(), // #nosec G204 -- fixed argv, constant-derived path, no shell involved
		fmt.Sprintf("could not give %s to UID 100 (the openbao user inside the container)", openbaoDataDir),
		"OpenBao will fail to write its keyring if the directory is not writable by that user.",
	)

	fmt.Println("Step 1/3: docker compose up -d openbao")
	if err := composeUpOpenbao(); err != nil {
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
	warnOnFailure(
		// #nosec G204 -- fixed argv; the only interpolation is our own numeric uid/gid, and openbaoRoot is constant-derived
		exec.Command("sudo", "-n", "chown",
			fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			openbaoRoot,
		).Run(),
		fmt.Sprintf("could not take ownership of %s", openbaoRoot),
		"If docker created it as root, creating the secrets directory below it will fail.",
	)
	// 0700: openbao-init.json lands here and holds the root token and the
	// unseal keys. Nobody but the owner has any reason to read it, and the
	// openbao-unseal sidecar mounts the directory into a container rather than
	// reading it as another host user.
	if err := os.MkdirAll(absSecretsDir, 0700); err != nil {
		// #nosec G204 -- fixed argv, constant-derived absolute path, no shell involved
		if err2 := exec.Command("sudo", "-n", "mkdir", "-p", absSecretsDir).Run(); err2 != nil {
			return fmt.Errorf("failed to create %s: %w (sudo fallback also failed: %v)",
				absSecretsDir, err, err2)
		}
	}
	// Make sure secrets dir is owned by us so openbao-init.sh can write
	// init.json. Recursive is fine here — secrets is a sibling of data,
	// the chown above on openbaoRoot is non-recursive.
	warnOnFailure(
		// #nosec G204 -- fixed argv; the only interpolation is our own numeric uid/gid, and absSecretsDir is constant-derived
		exec.Command("sudo", "-n", "chown", "-R",
			fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			absSecretsDir,
		).Run(),
		fmt.Sprintf("could not take ownership of %s", absSecretsDir),
		"openbao-init.sh writes openbao-init.json there and will fail if it cannot.",
	)
	// MkdirAll only applies its mode when it creates the directory, so a
	// secrets directory left over from an earlier run keeps whatever mode it
	// had (0755 before this change). Narrow it explicitly.
	warnOnFailure(
		os.Chmod(absSecretsDir, 0700), // #nosec G302 -- a directory, not a file: 0700 is already owner-only
		fmt.Sprintf("could not restrict %s to the owner", absSecretsDir),
		"The root token and unseal keys stored there would stay readable by other local users.",
	)
	absEnv, _ := filepath.Abs(envPath())
	initOut := filepath.Join(absSecretsDir, "openbao-init.json")
	script := filepath.Join(cbDir, "init", "openbao", "openbao-init.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("openbao-init.sh not found at %s: %w", script, err)
	}
	warnOnFailure(
		os.Chmod(script, 0750), // #nosec G302 -- openbao-init.sh must stay executable to be run; 0750 keeps it off other users
		fmt.Sprintf("could not make %s executable", script),
		"Running it will fail if it is not already executable.",
	)
	// The script is executed directly with cbDir as its working directory. It
	// used to be assembled into a `cd %q && ENV_FILE=%q … ./init/openbao/…`
	// string handed to /bin/sh, but %q is Go quoting, not shell quoting: it
	// produces double quotes, inside which a shell still expands $VAR and
	// $(command). A checkout path or an .env path containing either — a
	// directory literally named `$(id)`, or simply `$HOME` — was substituted or
	// executed by the shell before the script ever ran.
	if err := common.RunCommandInDir(
		cbDir,
		"./init/openbao/openbao-init.sh",
		nil,
		[]string{"ENV_FILE=" + absEnv, "INIT_OUTPUT=" + initOut},
	); err != nil {
		return fmt.Errorf("openbao-init.sh failed: %w", err)
	}

	// openbao-init.json now holds the root token and the unseal keys. The
	// upstream script does not restrict it, so do it here.
	warnOnFailure(
		os.Chmod(initOut, 0600),
		fmt.Sprintf("could not restrict %s to the owner", initOut),
		"It contains the root token and the unseal keys.",
	)

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
	// RootToken is the initial root token from POST /v1/sys/init. It is only
	// used by the preflight diagnostics to tell the user which value in
	// openbao-init.json to restore into .env; the unseal path never needs it.
	RootToken string `json:"root_token"`
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
	// initFile is openbaoInitFilePath() in every call site: a repo-relative
	// constant path, never an operator-supplied one.
	raw, err := os.ReadFile(initFile) // #nosec G304 -- constant repo-relative path, not caller-supplied
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
				"./mayfly infra run -d -s cb-tumblebug")
	} else if st.EnvTokenSet && st.TumblebugTokenSet && tumblebugHealthy() {
		// The container has *a* token — ask cb-tumblebug whether it still works.
		// An empty token is not the only way to be stale: a container started
		// before the current token holds an old, non-empty one, and only the
		// server can tell us OpenBao rejects it (signal C).
		if state, info := probeContainerToken(tumblebugAddr, readEnvValue("TB_API_USERNAME"), readEnvValue("TB_API_PASSWORD")); state == containerTokenInvalid {
			note := "cb-tumblebug holds a VAULT_TOKEN that OpenBao rejects — it was started before the " +
				"current token. Recreate it: ./mayfly infra run -d -s cb-tumblebug"
			if info.Message != "" {
				note += " (cb-tumblebug: " + info.Message + ")"
			}
			st.Notes = append(st.Notes, note)
		}
	}
	if st.EnvTokenSet && !st.TerrariumTokenSet {
		st.Notes = append(st.Notes,
			"mc-terrarium has empty VAULT_TOKEN although .env has one — same fix applies: "+
				"./mayfly infra run -d -s mc-terrarium")
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
		// curl is invoked as an argument vector rather than a shell string, in
		// line with the rest of this package. openbaoAddr is a constant so the
		// shell form was not exploitable, but keeping one style means the next
		// address that stops being constant does not reintroduce the question.
		// -sf already keeps curl quiet and makes an HTTP error a non-zero exit,
		// so the discarded stderr redirect is not needed.
		out, err := common.RunCommandOutput("curl", []string{"-sf", openbaoAddr + "/v1/sys/seal-status"}, nil)
		if err == nil && len(out) > 0 {
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
//
// The checkout's *actual* version is verified, not merely the presence of the
// init script. cb-tumblebug changes its credential yaml field names and its
// OpenBao registration between releases, and running the wrong release's
// openbao-init.sh does not fail — it reports success while writing an
// inconsistent state (empty credential values that only surface much later).
// A directory left over from an earlier version therefore has to be caught
// here, on what is the main path of a first-time build.
func ensureCbTumblebugSource() (string, error) {
	version, err := tumblebugVersionFromCompose()
	if err != nil {
		return "", fmt.Errorf("could not read cb-tumblebug version from docker-compose.yaml: %w", err)
	}
	gitTag := "v" + version
	// $HOME plus fixed segments; nothing operator-supplied contributes, and
	// Clean resolves the result before any file operation sees it.
	targetDir := filepath.Clean(filepath.Join(os.Getenv("HOME"), "go", "src", "github.com", "cloud-barista"))
	cbDir := filepath.Clean(filepath.Join(targetDir, "cb-tumblebug"))

	if _, err := os.Stat(cbDir); err == nil { // #nosec G703 -- $HOME plus fixed segments, cleaned above
		if err := reconcileCbTumblebugCheckout(cbDir, gitTag); err != nil {
			return "", err
		}
		return cbDir, initScriptErr(cbDir, gitTag)
	}

	// 0750 rather than 0755: only the operator running mayfly reads this checkout.
	if err := os.MkdirAll(targetDir, 0750); err != nil { // #nosec G703 -- $HOME plus fixed segments, cleaned above
		return "", err
	}
	fmt.Printf("  Cloning cb-tumblebug %s into %s ...\n", gitTag, cbDir)
	if err := common.RunCommandInDir(
		targetDir,
		"git",
		[]string{"clone", "-b", gitTag, "https://github.com/cloud-barista/cb-tumblebug.git"},
		nil,
	); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	// Confirm the clone really landed on the wanted tag. `clone -b` accepts a
	// branch name just as happily as a tag, so checking the checked-out version
	// is what distinguishes "cloned release v0.12.25" from "cloned some branch".
	tag, commit, err := common.GitCheckoutVersion(cbDir)
	if err != nil {
		return "", fmt.Errorf("cb-tumblebug was cloned but its version cannot be read: %w", err)
	}
	if tag != gitTag {
		return "", fmt.Errorf(
			"cb-tumblebug was cloned but is not on %s (currently on %s) — remove %s and run the command again",
			gitTag, describeCheckout(tag, commit), cbDir)
	}
	return cbDir, initScriptErr(cbDir, gitTag)
}

// cbTumblebugInitScript is the OpenBao initialization script shipped inside a
// cb-tumblebug checkout.
func cbTumblebugInitScript(cbDir string) string {
	return filepath.Join(cbDir, "init", "openbao", "openbao-init.sh")
}

// initScriptErr reports whether the checkout actually carries the script this
// package is about to run.
func initScriptErr(cbDir, gitTag string) error {
	if _, err := os.Stat(cbTumblebugInitScript(cbDir)); err != nil {
		return fmt.Errorf("openbao-init.sh is missing from the cb-tumblebug %s checkout at %s: %w", gitTag, cbDir, err)
	}
	return nil
}

// reconcileCbTumblebugCheckout makes an existing cb-tumblebug directory match
// gitTag, or fails loudly.
//
// This runs unattended (`infra run` initializes OpenBao by itself), so it
// cannot put a menu in front of the user the way `setup tumblebug-init` does.
// The order of preference is therefore: never proceed on a mismatched version;
// switch to the right tag when that is unambiguously safe; otherwise stop with
// an error that says what to do. A mismatch is always reported, whichever
// branch is taken.
func reconcileCbTumblebugCheckout(cbDir, gitTag string) error {
	tag, commit, err := common.GitCheckoutVersion(cbDir)
	if err != nil {
		return fmt.Errorf(
			"%s already exists but its cb-tumblebug version cannot be verified (%v). "+
				"Running the wrong version's openbao-init.sh corrupts the credential store without reporting an error. "+
				"Remove the directory and run the command again to get a clean %s checkout",
			cbDir, err, gitTag)
	}
	if tag == gitTag {
		return nil
	}

	fmt.Printf("  cb-tumblebug at %s is on %s, but docker-compose.yaml runs %s.\n",
		cbDir, describeCheckout(tag, commit), gitTag)

	clean, err := common.GitWorkTreeClean(cbDir)
	if err != nil {
		return fmt.Errorf("cb-tumblebug version mismatch at %s (%s, expected %s), and its state cannot be read: %w",
			cbDir, describeCheckout(tag, commit), gitTag, err)
	}
	if !clean {
		return fmt.Errorf(
			"cb-tumblebug at %s is on %s but %s is required, and the checkout has local changes so it cannot be switched automatically. "+
				"Commit or discard the changes and run `git -C %s checkout %s`, or remove the directory and run the command again",
			cbDir, describeCheckout(tag, commit), gitTag, cbDir, gitTag)
	}
	if !common.GitTagExists(cbDir, gitTag) {
		return fmt.Errorf(
			"cb-tumblebug at %s is on %s but %s is required, and that tag is not present in the local checkout. "+
				"Run `git -C %s fetch --tags && git -C %s checkout %s`, or remove the directory and run the command again",
			cbDir, describeCheckout(tag, commit), gitTag, cbDir, cbDir, gitTag)
	}

	fmt.Printf("  Switching cb-tumblebug to %s ...\n", gitTag)
	if _, err := common.GitOutputInDir(cbDir, "checkout", gitTag); err != nil {
		return fmt.Errorf("could not switch cb-tumblebug at %s to %s: %w — remove the directory and run the command again",
			cbDir, gitTag, err)
	}

	// Read the version back rather than trusting the checkout to have done what
	// was asked: the whole point here is that a silent mismatch is the failure.
	tag, commit, err = common.GitCheckoutVersion(cbDir)
	if err != nil || tag != gitTag {
		return fmt.Errorf("cb-tumblebug at %s is still not on %s after switching (currently on %s)",
			cbDir, gitTag, describeCheckout(tag, commit))
	}
	fmt.Printf("  cb-tumblebug is now on %s.\n", gitTag)
	return nil
}

// describeCheckout renders a checkout for a message: its tag when it sits on
// one, otherwise the commit it is parked at.
func describeCheckout(tag, commit string) string {
	if tag != "" {
		return tag
	}
	if commit == "" {
		return "an unknown version"
	}
	short := commit
	if len(short) > 12 {
		short = short[:12]
	}
	return "commit " + short + " (no release tag)"
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
	return readEnvValue(vaultTokenKey)
}

// readEnvValue returns the value of key from mayfly's .env, or "" when the file
// or the key is absent. Values are used as-is apart from quote stripping (no
// shell-style expansion) — the keys read here (VAULT_TOKEN, TB_API_*) are plain
// literals.
//
// This delegates to common.ParseDotEnv rather than scanning the file itself.
// The hand-rolled scanner it replaces did not strip surrounding quotes, and the
// values are used as credentials: TB_API_PASSWORD="pass" came back with the
// quotes attached, so the Basic auth header carried `"pass"` and cb-tumblebug
// answered 401. probeContainerToken reads that 401 as "cannot tell", which
// downgrades the container-token check to unknown and quietly disables the very
// signal container_token.go exists to provide. Having one parser means a value
// is read the same way whoever reads it.
func readEnvValue(key string) string {
	values, err := common.ParseDotEnv(envPath())
	if err != nil {
		return ""
	}
	return values[key]
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
