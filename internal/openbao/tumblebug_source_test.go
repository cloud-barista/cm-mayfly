package openbao

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireGit skips the test when git is unavailable so the suite stays green
// on machines and CI images without a git binary.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
}

// runGit runs git inside dir with a fixed identity, since committing fails
// when no user.name/user.email is configured in the environment.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.com",
		"-c", "user.name=test",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// fakeTumblebugCheckout builds a local stand-in for a cb-tumblebug clone: two
// releases carrying different openbao-init.sh contents, both tagged the way
// cb-tumblebug tags its releases — lightweight. No network is involved; the
// point of these tests is the version check, not the clone.
func fakeTumblebugCheckout(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	scriptDir := filepath.Join(dir, "init", "openbao")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scriptDir, "openbao-init.sh")

	if err := os.WriteFile(script, []byte("#!/bin/sh\necho v0.12.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "v0.12.22")
	runGit(t, dir, "tag", "v0.12.22") // lightweight, as upstream does

	if err := os.WriteFile(script, []byte("#!/bin/sh\necho v0.12.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "v0.12.25")
	runGit(t, dir, "tag", "v0.12.25")

	return dir
}

// TestReconcileAcceptsMatchingCheckout: the wanted release is already checked
// out, so nothing happens and nothing is reported.
func TestReconcileAcceptsMatchingCheckout(t *testing.T) {
	dir := fakeTumblebugCheckout(t)
	runGit(t, dir, "checkout", "-q", "v0.12.25")

	if err := reconcileCbTumblebugCheckout(dir, "v0.12.25"); err != nil {
		t.Fatalf("a checkout already on the wanted tag was rejected: %v", err)
	}
	if got := runGit(t, dir, "describe", "--exact-match", "--tags", "HEAD"); got != "v0.12.25" {
		t.Errorf("checkout moved unexpectedly: now on %q", got)
	}
}

// TestReconcileSwitchesStaleCheckout is the bug this change is about: a
// directory left over from an older release must not be used as-is. With a
// clean work tree the fix is to move it onto the wanted tag.
func TestReconcileSwitchesStaleCheckout(t *testing.T) {
	dir := fakeTumblebugCheckout(t)
	runGit(t, dir, "checkout", "-q", "v0.12.22")

	if err := reconcileCbTumblebugCheckout(dir, "v0.12.25"); err != nil {
		t.Fatalf("a clean stale checkout should have been switched, got: %v", err)
	}
	if got := runGit(t, dir, "describe", "--exact-match", "--tags", "HEAD"); got != "v0.12.25" {
		t.Fatalf("checkout was not switched: still on %q", got)
	}
	// The script that would actually run must be the new release's.
	body, err := os.ReadFile(filepath.Join(dir, "init", "openbao", "openbao-init.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "v0.12.25") {
		t.Errorf("the old release's openbao-init.sh is still on disk: %q", body)
	}
}

// TestReconcileRefusesDirtyStaleCheckout: switching would carry local edits
// onto another release, so this stops instead — loudly, with instructions.
func TestReconcileRefusesDirtyStaleCheckout(t *testing.T) {
	dir := fakeTumblebugCheckout(t)
	runGit(t, dir, "checkout", "-q", "v0.12.22")
	if err := os.WriteFile(filepath.Join(dir, "init", "openbao", "openbao-init.sh"),
		[]byte("#!/bin/sh\necho edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := reconcileCbTumblebugCheckout(dir, "v0.12.25")
	if err == nil {
		t.Fatal("a stale checkout with local changes was accepted silently")
	}
	for _, want := range []string{"v0.12.22", "v0.12.25", "local changes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
	if got := runGit(t, dir, "describe", "--exact-match", "--tags", "HEAD"); got != "v0.12.22" {
		t.Errorf("the checkout was moved despite local changes: now on %q", got)
	}
}

// TestReconcileRefusesWhenTagAbsent: the wanted release is not in this
// checkout at all, so there is nothing safe to switch to.
func TestReconcileRefusesWhenTagAbsent(t *testing.T) {
	dir := fakeTumblebugCheckout(t)
	runGit(t, dir, "checkout", "-q", "v0.12.22")

	err := reconcileCbTumblebugCheckout(dir, "v0.99.0")
	if err == nil {
		t.Fatal("a checkout missing the wanted tag was accepted silently")
	}
	if !strings.Contains(err.Error(), "not present in the local checkout") {
		t.Errorf("error should explain the tag is missing, got: %v", err)
	}
}

// TestReconcileRefusesOffTagCheckout: HEAD parked on an untagged commit cannot
// be shown to match the wanted release, so it is not used.
func TestReconcileRefusesOffTagCheckout(t *testing.T) {
	dir := fakeTumblebugCheckout(t)
	runGit(t, dir, "checkout", "-q", "v0.12.22")
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "local work")

	// The wanted tag exists and the tree is clean, so this one is recoverable:
	// it must end up on the tag rather than quietly running the untagged code.
	if err := reconcileCbTumblebugCheckout(dir, "v0.12.25"); err != nil {
		t.Fatalf("a clean off-tag checkout should have been switched, got: %v", err)
	}
	if got := runGit(t, dir, "describe", "--exact-match", "--tags", "HEAD"); got != "v0.12.25" {
		t.Fatalf("checkout was not switched: still on %q", got)
	}
}

// TestReconcileRefusesNonGitDirectory: a directory that is not a checkout has
// no version to verify, so it cannot be trusted to hold the right release.
func TestReconcileRefusesNonGitDirectory(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "init", "openbao")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The script is present — under the old code that alone was enough.
	if err := os.WriteFile(filepath.Join(scriptDir, "openbao-init.sh"), []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := reconcileCbTumblebugCheckout(dir, "v0.12.25")
	if err == nil {
		t.Fatal("a non-git directory was accepted just because openbao-init.sh existed")
	}
	if !strings.Contains(err.Error(), "cannot be verified") {
		t.Errorf("error should say the version cannot be verified, got: %v", err)
	}
}

// TestInitScriptErr checks the remaining half of the guarantee: the checkout is
// on the right release *and* actually carries the script.
func TestInitScriptErr(t *testing.T) {
	dir := fakeTumblebugCheckout(t)
	runGit(t, dir, "checkout", "-q", "v0.12.25")
	if err := initScriptErr(dir, "v0.12.25"); err != nil {
		t.Fatalf("a checkout carrying openbao-init.sh was rejected: %v", err)
	}

	empty := t.TempDir()
	err := initScriptErr(empty, "v0.12.25")
	if err == nil {
		t.Fatal("a checkout without openbao-init.sh was accepted")
	}
	if !strings.Contains(err.Error(), "openbao-init.sh is missing") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestDescribeCheckout(t *testing.T) {
	cases := []struct {
		tag, commit, want string
	}{
		{"v0.12.25", "abcdef1234567890", "v0.12.25"},
		{"", "abcdef1234567890", "commit abcdef123456 (no release tag)"},
		{"", "", "an unknown version"},
	}
	for _, c := range cases {
		if got := describeCheckout(c.tag, c.commit); got != c.want {
			t.Errorf("describeCheckout(%q, %q) = %q, want %q", c.tag, c.commit, got, c.want)
		}
	}
}
