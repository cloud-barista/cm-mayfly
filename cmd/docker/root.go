/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package docker

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/cmd"
	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// DockerFilePath is a variable that holds path to the docker-compose.yaml.
var DockerFilePath string

// ProjectName is a variable that holds docker compose project name.
var ProjectName string

// ServiceNames holds the raw -s values. The flag may be repeated, and each
// occurrence may itself list several services separated by commas or spaces.
// Use resolveSelectedServices to turn it into a validated service list.
var ServiceNames []string

// restCmd represents the rest command
var dockerCmd = &cobra.Command{
	Use:   "infra",
	Short: "Installing and managing cloud-migrator's infrastructure",
	Long: `Build the environment of the infrastructure required for cloud-migrator and monitor the running status of the infrastructure.
For example, you can install and run, stop, update and ... Cloud-Migrator runtimes.

- ./mayfly infra install
- ./mayfly infra run
- ./mayfly infra run -d
- ./mayfly infra run -d -s cb-tumblebug
- ./mayfly infra info
- ./mayfly infra stop
- ./mayfly infra update
- ./mayfly infra remove
- ./mayfly infra remove --clean-db
- ./mayfly infra remove --clean-all
	     `,
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println(cmd.UsageString())
		//fmt.Println(cmd.Help())
		_ = cmd.Help()
	},
	// Before any docker (infra) subcommand runs a `docker compose` command,
	// make sure the shared environment file exists. The compose file relies on
	// it for ${VAR} interpolation, so running without it would fail with
	// confusing "variable is not set" warnings.
	//
	// Whether every value in that file must also be filled in depends on the
	// subcommand — see startsContainers.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// The bare `infra` command only prints help and does not invoke
		// docker compose, so it does not require the .env file.
		if !cmd.HasParent() || cmd.Name() == "infra" {
			return nil
		}
		if err := ensureDockerEnvFile(); err != nil {
			return err
		}
		if !startsContainers[cmd.Name()] {
			return nil
		}
		// Fill in the values we can generate ourselves before checking for blanks,
		// so a user is never asked to invent one by hand.
		if err := ensureGeneratedEnvValues(); err != nil {
			return err
		}
		return validateDockerEnvFile()
	},
}

// startsContainers lists the infra subcommands that bring containers up
// (`docker compose up`). Only those need every value in .env to be filled in,
// because only they hand those values to a container.
//
// Every other subcommand tears down or reads: `remove` (down/rm/stop), `stop`,
// `install` (pull), `info` (ps/images), `logs`. None of them pass a value into a
// running process, so a missing key cannot hurt them.
//
// Why this distinction matters: requiring a complete .env to *delete* an
// environment blocks the very thing the user is trying to do. Switching branch or
// lineup adds new keys to .env.example, and until the user copies them across,
// `infra remove --clean-all` would refuse to run — so the clean rebuild that
// would have fixed everything is exactly what gets blocked, and the only way out
// is raw docker. Startup validation belongs on startup.
var startsContainers = map[string]bool{
	"run":    true, // docker compose up
	"update": true, // pull + up (restarts the stack)
}

// Every key declared in conf/docker/.env.example must hold a non-empty value in
// .env before any `mayfly infra` subcommand may run docker compose — EXCEPT the
// keys in optionalEnvKeys below. The required set is derived from .env.example
// (see requiredKeysFrom) rather than hardcoded, so a value a user accidentally
// blanks or deletes is caught even for variables that ship with a default, and
// newly added variables are required by default (opt out via optionalEnvKeys).
//
// Rationale: a container that reads a blank value and has no built-in default of
// its own silently misbehaves (e.g. each postgres healthcheck `pg_isready -U
// ${*_DB_USER}` fails when the user is blank, deadlocking every dependent
// service; cb-spider 0.12.17+ aborts on blank REST auth). Values are also passed
// between containers, so a downstream container cannot be assumed to have a safe
// default for an upstream one — hence "require everything except the explicit
// exceptions".
//
// optionalEnvKeys are the ONLY entries allowed to be blank:
//   - SMTP_* : cm-cicada email notifications are optional; the stack runs without them.
//   - VAULT_TOKEN : intentionally blank in the template; it is generated/written by
//     the OpenBao init flow during `infra run` (a fresh clean install starts with it
//     blank — requiring it would block the auto-init the install depends on).
//   - OPENBAO_UNSEAL_POLL_INTERVAL : the compose file already substitutes a default
//     for it (`${OPENBAO_UNSEAL_POLL_INTERVAL:-30}`), so a blank value never reaches
//     the sidecar — 30 does. It is not a secret either.
//
// Before adding a key here, check both: does a blank value actually reach a
// container, and is the key a secret? A key the compose file defaults with `:-`
// answers "no" to the first and belongs here; a credential never does.
var optionalEnvKeys = map[string]bool{
	"SMTP_HOST":                    true,
	"SMTP_PORT":                    true,
	"SMTP_USER":                    true,
	"SMTP_PASSWORD":                true,
	"SMTP_MAIL_FROM":               true,
	"VAULT_TOKEN":                  true,
	"OPENBAO_UNSEAL_POLL_INTERVAL": true,
}

// fallbackRequiredEnvKeys is used only when .env.example cannot be read (so the
// required set can't be derived) — the critical keys whose blank value hard-fails
// or deadlocks startup, so the guard never silently disappears.
var fallbackRequiredEnvKeys = []string{
	"SPIDER_USERNAME",
	"SPIDER_PASSWORD",
	"TUMBLEBUG_DB_USER",
	"TUMBLEBUG_DB_PASSWORD",
	"BUTTERFLY_DB_USER",
	"BUTTERFLY_DB_PASSWORD",
	"ANT_DB_USER",
	"ANT_DB_PASSWORD",
	"AIRFLOW_DB_USER",
	"AIRFLOW_DB_PASSWORD",
	"AIRFLOW_DB_ROOT_PASSWORD",
	"BEETLE_API_PASSWORD",
}

// requiredKeysFrom returns the sorted list of keys that must be non-empty: every
// key in .env.example minus optionalEnvKeys. If .env.example can't be parsed it
// falls back to fallbackRequiredEnvKeys so the check is never lost entirely.
func requiredKeysFrom(examplePath string) []string {
	exampleVals, err := parseDotEnv(examplePath)
	if err != nil || len(exampleVals) == 0 {
		return fallbackRequiredEnvKeys
	}
	keys := make([]string, 0, len(exampleVals))
	for k := range exampleVals {
		if !optionalEnvKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// ensureDockerEnvFile verifies that the docker-compose environment file exists
// next to the compose file (DockerFilePath). It returns an error with English
// guidance if the file is missing, so the docker compose command is never run
// with unset variables.
func ensureDockerEnvFile() error {
	dir := filepath.Dir(DockerFilePath)
	envPath := filepath.Join(dir, ".env")
	examplePath := filepath.Join(dir, ".env.example")
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("environment file not found: %s\n\n"+
				"docker-compose.yaml requires this file for ${VAR} interpolation "+
				"(DB credentials, SMTP, log levels, etc.).\n"+
				"Create it from the template, then edit the values before running this command again:\n\n"+
				"  cp %s %s\n",
				envPath, examplePath, envPath)
		}
		return fmt.Errorf("failed to check environment file %s: %w", envPath, err)
	}
	return nil
}

// generatedEnvKeys are the required keys mayfly fills in itself when .env leaves
// them blank, so the user never has to invent a value by hand.
//
// A key belongs here when it is a secret with no sensible default that only this
// stack consumes — nothing outside issues it, and nothing outside has to agree on
// it. AIRFLOW_JWT_SECRET is exactly that: Airflow signs the tokens its own
// processes exchange with it, and any random string will do as long as every
// process sees the same one.
//
// Note this is the opposite of optionalEnvKeys. These keys stay *required* — they
// are just filled in before the check runs, so a blank line in .env is a value
// waiting to be generated rather than an error.
var generatedEnvKeys = []string{
	"AIRFLOW_JWT_SECRET",
}

// ensureGeneratedEnvValues fills every blank generatedEnvKeys entry in .env with
// a fresh random value and writes it back.
//
// It only ever fills a blank. An existing value is never touched: rotating the
// key invalidates tokens that were already issued, so tasks sitting in the celery
// queue would fail on a key the scheduler no longer signs with. Keeping the value
// stable is the whole point — the key needs to be unguessable, not new.
//
// Note on ordering: this writes to .env from PersistentPreRunE, which is before
// `infra run` asks "Do you want to proceed? (y/N)". Answering N therefore leaves
// a generated AIRFLOW_JWT_SECRET behind in .env. That is deliberate, and moving
// the generation after the prompt was considered and rejected: validateDockerEnvFile
// runs in the same hook and treats a blank AIRFLOW_JWT_SECRET as a hard error, so
// generation has to precede validation, and validation has to precede the prompt
// for the check to fail fast rather than after the user has committed. Deferring
// generation past the prompt would make the first run fail validation before the
// prompt was ever reached.
//
// Leaving the value behind is harmless in a way the removal steps are not: it is
// a random local secret nothing outside this stack has to agree on, it is written
// once and never rotated, and a cancelled run is left in exactly the state a
// successful one would have started from. What was missing was not the ordering
// but the disclosure — the message below now says the file was changed and that
// the value stays, so a user who cancels is not surprised by an .env diff.
func ensureGeneratedEnvValues() error {
	dir := filepath.Dir(DockerFilePath)
	envPath := filepath.Join(dir, ".env")
	values, err := parseDotEnv(envPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", envPath, err)
	}
	for _, key := range generatedEnvKeys {
		if strings.TrimSpace(values[key]) != "" {
			continue
		}
		secret, err := randomSecret()
		if err != nil {
			return fmt.Errorf("failed to generate a value for %s: %w", key, err)
		}
		if err := setEnvKey(envPath, key, secret); err != nil {
			return fmt.Errorf("failed to write %s to %s: %w", key, envPath, err)
		}
		fmt.Printf("Generated %s and saved it to %s (first run).\n", key, envPath)
		fmt.Println("  This value is kept even if you cancel at the confirmation prompt; it is reused on every later run.")
	}
	return nil
}

// randomSecret returns 32 random bytes, base64 encoded — comfortably above the
// 16 bytes Airflow generates for the same key when one is not configured.
func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// setEnvKey writes key=value into .env in place, replacing the existing line if
// there is one and appending it otherwise (an .env carried over from an older
// version simply has no line for a key added since). Every other line, including
// comments, is preserved.
func setEnvKey(path, key, value string) error {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the internal .env next to the compose file, not user input
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = key + "=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		// Keep the trailing newline the file almost certainly ends with, rather
		// than gluing the new entry onto the last line.
		if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "" {
			lines[n-1] = key + "=" + value
			lines = append(lines, "")
		} else {
			lines = append(lines, key+"="+value)
		}
	}
	// .env holds secrets; writeEnvFile replaces it atomically and keeps it
	// owner-only.
	return writeEnvFile(path, []byte(strings.Join(lines, "\n")))
}

// validateDockerEnvFile parses conf/docker/.env and reports any required key
// (see requiredKeysFrom) that is missing or set to an empty value. It assumes
// ensureDockerEnvFile has already confirmed the file exists.
func validateDockerEnvFile() error {
	dir := filepath.Dir(DockerFilePath)
	envPath := filepath.Join(dir, ".env")
	examplePath := filepath.Join(dir, ".env.example")
	values, err := parseDotEnv(envPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", envPath, err)
	}
	var missing []string
	for _, key := range requiredKeysFrom(examplePath) {
		if strings.TrimSpace(values[key]) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("required values are missing or blank in %s:\n  - %s\n\n"+
		"Starting the stack needs every variable in %s, because a container that reads "+
		"a blank value has no safe built-in default (e.g. cb-spider 0.12.17+ exits on "+
		"blank REST auth; a postgres healthcheck `pg_isready -U ${*_DB_USER}` fails on "+
		"a blank user and deadlocks every dependent service).\n"+
		"Only the cm-cicada SMTP_* settings, VAULT_TOKEN (auto-generated on first run) "+
		"and OPENBAO_UNSEAL_POLL_INTERVAL (compose defaults it to 30) may be left blank.\n"+
		"Copy the defaults from %s and fill in the secret values, then re-run.\n\n"+
		"This check only guards the commands that start containers. If you are trying "+
		"to tear the environment down or look at it, `infra remove`, `stop`, `info` and "+
		"`logs` run without it.\n",
		envPath, strings.Join(missing, "\n  - "), examplePath, examplePath)
}

// parseDotEnv delegates to common.ParseDotEnv so the .env parser is shared with
// the api/rest auth env resolution (single source of truth).
func parseDotEnv(path string) (map[string]string, error) {
	return common.ParseDotEnv(path)
}

// SysCallDockerComposePsWithAll executes `docker compose ps` with an optional
// --all flag, scoped to the services named by -s (all services when -s is
// omitted).
func SysCallDockerComposePsWithAll(showAll bool) {
	fmt.Println("\n[v]Status of Cloud-Migrator runtimes")

	services, err := resolveSelectedServices()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	args := []string{"ps"}
	if showAll {
		args = append(args, "-a")
	}
	args = append(args, services...)

	if err := runCompose(args...); err != nil {
		fmt.Printf("❌ docker compose ps failed: %v\n", err)
	}
}

func init() {
	cmd.RootCmd.AddCommand(dockerCmd)

	// Add flags for Docker Compose yaml File Path
	dockerCmd.PersistentFlags().StringVarP(&DockerFilePath, "file", "f", common.DefaultDockerComposeConfig, "User-defined configuration file")

	// Add flags for Docker Compose project name
	dockerCmd.PersistentFlags().StringVarP(&ProjectName, "project-name", "p", common.ComposeProjectName, "User-defined docker compose porject name")

	// -s may be repeated and each value may list several services separated by
	// commas or spaces; all three forms produce the same target set.
	dockerCmd.PersistentFlags().StringArrayVarP(&ServiceNames, "service", "s", nil, "Target specific services only (repeatable; comma or space separated). Default: all")
}
