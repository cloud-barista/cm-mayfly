package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Only cm-cicada SMTP, the auto-generated VAULT_TOKEN and the compose-defaulted
// OPENBAO_UNSEAL_POLL_INTERVAL may be blank; every other .env.example variable
// must be required. Guard the exception registry so a DB user (or any other key)
// can never quietly become optional again.
func TestOptionalEnvKeys(t *testing.T) {
	mustBeOptional := []string{
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "SMTP_MAIL_FROM",
		"VAULT_TOKEN",
		// compose substitutes `${OPENBAO_UNSEAL_POLL_INTERVAL:-30}`, so a blank
		// value never reaches the sidecar.
		"OPENBAO_UNSEAL_POLL_INTERVAL",
	}
	for _, k := range mustBeOptional {
		if !optionalEnvKeys[k] {
			t.Errorf("%q must be optional (blank-allowed)", k)
		}
	}
	mustNotBeOptional := []string{"TUMBLEBUG_DB_USER", "SPIDER_PASSWORD", "ANT_DB_USER", "VAULT_ADDR"}
	for _, k := range mustNotBeOptional {
		if optionalEnvKeys[k] {
			t.Errorf("%q must NOT be optional — it has to be a required value", k)
		}
	}
}

// Required-value validation guards startup only. Tearing an environment down or
// looking at it must never be blocked by a missing key: a branch or lineup switch
// adds keys to .env.example, and gating `remove` on them blocks the clean rebuild
// that would have fixed the very problem.
func TestStartsContainers(t *testing.T) {
	for _, name := range []string{"run", "update"} {
		if !startsContainers[name] {
			t.Errorf("%q brings containers up — it must validate .env before starting", name)
		}
	}
	// teardown (remove/stop) and read-only (info/logs) + image pull (install)
	for _, name := range []string{"remove", "stop", "info", "logs", "install"} {
		if startsContainers[name] {
			t.Errorf("%q does not start containers — gating it on a complete .env blocks teardown/inspection", name)
		}
	}
}

// withEnvFixture writes an .env.example and .env into a temp dir and points
// DockerFilePath at it for the test's duration.
func withEnvFixture(t *testing.T, exampleBody, envBody string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte(exampleBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := DockerFilePath
	DockerFilePath = filepath.Join(dir, "docker-compose.yaml")
	t.Cleanup(func() { DockerFilePath = prev })
	return dir
}

// a representative .env.example: non-secret defaults, blank secrets, and the two
// blank-allowed categories (SMTP + VAULT_TOKEN).
const exampleFixture = `# header
SPIDER_USERNAME=
SPIDER_PASSWORD=
SPIDER_LOG_LEVEL=error
TUMBLEBUG_DB_USER=tumblebug
TUMBLEBUG_DB_PASSWORD=
VAULT_ADDR=http://openbao:8200
VAULT_TOKEN=
SMTP_HOST=smtp.gmail.com
SMTP_USER=
`

func TestRequiredKeysFrom(t *testing.T) {
	dir := withEnvFixture(t, exampleFixture, "")
	got := requiredKeysFrom(filepath.Join(dir, ".env.example"))
	want := map[string]bool{
		"SPIDER_USERNAME": true, "SPIDER_PASSWORD": true, "SPIDER_LOG_LEVEL": true,
		"TUMBLEBUG_DB_USER": true, "TUMBLEBUG_DB_PASSWORD": true, "VAULT_ADDR": true,
	}
	if len(got) != len(want) {
		t.Fatalf("required keys = %v, want %d keys", got, len(want))
	}
	for _, k := range got {
		if !want[k] {
			t.Errorf("unexpected required key %q (SMTP_*/VAULT_TOKEN must be excluded)", k)
		}
	}

	t.Run("fallback when example unreadable", func(t *testing.T) {
		if fk := requiredKeysFrom(filepath.Join(t.TempDir(), "missing.example")); len(fk) == 0 {
			t.Error("must fall back to a non-empty critical key set when .env.example is unreadable")
		}
	})
}

func TestValidateDockerEnvFile(t *testing.T) {
	// .env with every required key set, both optional categories left blank.
	fullEnv := `SPIDER_USERNAME=u
SPIDER_PASSWORD=p
SPIDER_LOG_LEVEL=error
TUMBLEBUG_DB_USER=tumblebug
TUMBLEBUG_DB_PASSWORD=p
VAULT_ADDR=http://openbao:8200
VAULT_TOKEN=
SMTP_HOST=smtp.gmail.com
SMTP_USER=
`

	t.Run("all required set, optionals blank → ok", func(t *testing.T) {
		withEnvFixture(t, exampleFixture, fullEnv)
		if err := validateDockerEnvFile(); err != nil {
			t.Errorf("should validate, got: %v", err)
		}
	})

	t.Run("blank required (DB user) → flagged", func(t *testing.T) {
		withEnvFixture(t, exampleFixture, strings.Replace(fullEnv, "TUMBLEBUG_DB_USER=tumblebug\n", "TUMBLEBUG_DB_USER=\n", 1))
		err := validateDockerEnvFile()
		if err == nil || !strings.Contains(err.Error(), "TUMBLEBUG_DB_USER") {
			t.Errorf("blank TUMBLEBUG_DB_USER must be flagged, got: %v", err)
		}
	})

	t.Run("deleted required line → flagged (accidental deletion)", func(t *testing.T) {
		withEnvFixture(t, exampleFixture, strings.Replace(fullEnv, "SPIDER_LOG_LEVEL=error\n", "", 1))
		err := validateDockerEnvFile()
		if err == nil || !strings.Contains(err.Error(), "SPIDER_LOG_LEVEL") {
			t.Errorf("a required key missing from .env entirely must be flagged, got: %v", err)
		}
	})

	t.Run("blank optionals (SMTP / VAULT_TOKEN) → still ok", func(t *testing.T) {
		// fullEnv already leaves SMTP_USER and VAULT_TOKEN blank.
		withEnvFixture(t, exampleFixture, fullEnv)
		if err := validateDockerEnvFile(); err != nil {
			t.Errorf("blank SMTP_USER / VAULT_TOKEN must be allowed, got: %v", err)
		}
	})
}

// The reported bug, end to end through the hook that actually gates the commands:
// .env is missing a key that .env.example gained on a newer branch, so `remove`
// refused to run and the user could not clean up and rebuild.
func TestPersistentPreRunEScope(t *testing.T) {
	// .env predating a branch that added SPIDER_LOG_LEVEL to .env.example.
	staleEnv := `SPIDER_USERNAME=u
SPIDER_PASSWORD=p
TUMBLEBUG_DB_USER=tumblebug
TUMBLEBUG_DB_PASSWORD=p
VAULT_ADDR=http://openbao:8200
VAULT_TOKEN=
SMTP_HOST=smtp.gmail.com
SMTP_USER=
`
	run := func(t *testing.T, name string) error {
		t.Helper()
		withEnvFixture(t, exampleFixture, staleEnv)
		sub := &cobra.Command{Use: name}
		dockerCmd.AddCommand(sub)
		t.Cleanup(func() { dockerCmd.RemoveCommand(sub) })
		return dockerCmd.PersistentPreRunE(sub, nil)
	}

	// teardown and inspection must survive an incomplete .env — this is the bug.
	for _, name := range []string{"remove", "stop", "info", "logs", "install"} {
		t.Run(name+" → not blocked by a missing key", func(t *testing.T) {
			if err := run(t, name); err != nil {
				t.Errorf("%q must run with an incomplete .env, got: %v", name, err)
			}
		})
	}

	// starting the stack must still be blocked — the original guard stays intact.
	for _, name := range []string{"run", "update"} {
		t.Run(name+" → still blocked", func(t *testing.T) {
			err := run(t, name)
			if err == nil || !strings.Contains(err.Error(), "SPIDER_LOG_LEVEL") {
				t.Errorf("%q must still refuse to start with a missing key, got: %v", name, err)
			}
		})
	}
}
