package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Only cm-cicada SMTP and the auto-generated VAULT_TOKEN may be blank; every
// other .env.example variable must be required. Guard the exception registry so
// a DB user (or any other key) can never quietly become optional again.
func TestOptionalEnvKeys(t *testing.T) {
	mustBeOptional := []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "SMTP_MAIL_FROM", "VAULT_TOKEN"}
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
