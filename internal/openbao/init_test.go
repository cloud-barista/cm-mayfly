package openbao

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cm-mayfly/cm-mayfly/common"
)

// writeEnv lays down a conf/docker/.env under a fresh temp root and chdirs
// there, which is what envPath() resolves against.
func writeEnv(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dockerDir := filepath.Join(root, "conf", "docker")
	if err := os.MkdirAll(dockerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dockerDir, ".env")
	if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return p
}

// readEnvValue must strip surrounding quotes.
//
// This is the regression that mattered most: the previous hand-rolled scanner
// returned TB_API_PASSWORD="pass" with the quotes attached, so the Basic auth
// header carried `"pass"`, cb-tumblebug answered 401, and probeContainerToken
// read that 401 as "cannot tell" — silently turning the container-token check
// into a permanent "unknown".
func TestReadEnvValueStripsQuotes(t *testing.T) {
	writeEnv(t, strings.Join([]string{
		`TB_API_USERNAME="default"`,
		`TB_API_PASSWORD="p@ss word"`,
		`SINGLE='sq-value'`,
		`PLAIN=plain-value`,
		`SPACED =  spaced-value  `,
		`export EXPORTED="exported-value"`,
		`# COMMENTED="ignored"`,
		``,
	}, "\n"))

	for _, tc := range []struct{ key, want string }{
		{"TB_API_USERNAME", "default"},
		{"TB_API_PASSWORD", "p@ss word"},
		{"SINGLE", "sq-value"},
		{"PLAIN", "plain-value"},
		{"SPACED", "spaced-value"},
		{"EXPORTED", "exported-value"},
		{"COMMENTED", ""},
		{"ABSENT", ""},
	} {
		if got := readEnvValue(tc.key); got != tc.want {
			t.Errorf("readEnvValue(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

// HasVaultToken has to agree with readEnvValue, including on the quoted-empty
// forms it used to special-case by hand.
func TestHasVaultToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		want bool
	}{
		{"set", "VAULT_TOKEN=s.abc123\n", true},
		{"set quoted", `VAULT_TOKEN="s.abc123"` + "\n", true},
		{"blank", "VAULT_TOKEN=\n", false},
		{"double-quoted empty", `VAULT_TOKEN=""` + "\n", false},
		{"single-quoted empty", `VAULT_TOKEN=''` + "\n", false},
		{"absent", "TUMBLEBUG_DB_PASSWORD=keepme\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeEnv(t, tc.env)
			if got := HasVaultToken(); got != tc.want {
				t.Errorf("HasVaultToken() = %v, want %v (env %q)", got, tc.want, tc.env)
			}
		})
	}
}

// A missing .env is not a token.
func TestHasVaultTokenNoFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if HasVaultToken() {
		t.Error("HasVaultToken() = true with no .env at all")
	}
}

// The openbao flow runs its script through common.RunCommandInDir, not a shell
// string. This pins the property that actually protects it: a working directory
// holding shell metacharacters is passed to the kernel literally instead of
// being expanded.
//
// The old code built `cd %q && ENV_FILE=%q … ./init/openbao/openbao-init.sh`
// for /bin/sh. %q is Go quoting, which emits double quotes — and inside double
// quotes a shell still expands $VAR and runs $(command). A checkout path
// containing either was substituted, or executed, before the script started.
func TestRunCommandInDirDoesNotGoThroughAShell(t *testing.T) {
	parent := t.TempDir()
	// A directory name a shell would mangle: command substitution, a variable
	// reference, and whitespace.
	hostile := filepath.Join(parent, `$(touch pwned) $HOME dir`)
	if err := os.Mkdir(hostile, 0o755); err != nil {
		t.Fatal(err)
	}

	// A script in that directory, invoked by relative path exactly as
	// openbao-init.sh is, that reports where it ran and what it was handed.
	script := filepath.Join(hostile, "probe.sh")
	body := "#!/bin/sh\npwd\nprintf 'ENV_FILE=%s\\n' \"$ENV_FILE\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	// The env value carries metacharacters too — it stands in for a path that
	// would have been interpolated into the old command string.
	envValue := `/tmp/$(touch pwned-env)/.env`

	out := captureStdout(t, func() {
		if err := common.RunCommandInDir(hostile, "./probe.sh", nil, []string{"ENV_FILE=" + envValue}); err != nil {
			t.Fatalf("RunCommandInDir: %v", err)
		}
	})

	// The script ran in the hostile directory, name intact.
	if !strings.Contains(out, hostile) {
		t.Errorf("script did not run in %q; output was %q", hostile, out)
	}
	// The env var arrived byte-for-byte, unexpanded.
	if !strings.Contains(out, "ENV_FILE="+envValue) {
		t.Errorf("ENV_FILE was not passed literally; output was %q", out)
	}
	// Nothing executed the command substitutions.
	for _, dir := range []string{parent, hostile} {
		if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
			t.Errorf("command substitution in the directory name was executed (found %s/pwned)", dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "pwned-env")); err == nil {
			t.Errorf("command substitution in the env value was executed (found %s/pwned-env)", dir)
		}
	}
}

// captureStdout collects everything fn prints, since RunCommandInDir streams the
// child's output to stdout rather than returning it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	return <-done
}
