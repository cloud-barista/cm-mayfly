package common

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

// gitRepoWithTags builds a throwaway repository with two commits: a
// lightweight tag on the first and an annotated tag on the second.
//
// cb-tumblebug publishes its release tags as lightweight tags, so that is the
// case that matters in production; the annotated tag sits alongside it to show
// both kinds resolve.
func gitRepoWithTags(t *testing.T) (dir, firstCommit, secondCommit string) {
	t.Helper()
	requireGit(t)
	dir = t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "first")
	firstCommit = runGit(t, dir, "rev-parse", "HEAD")
	// Lightweight tag: no -a/-m, so no tag object is created.
	runGit(t, dir, "tag", "v0.12.22")

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "second")
	secondCommit = runGit(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "tag", "-a", "v0.12.25", "-m", "annotated")

	return dir, firstCommit, secondCommit
}

// TestGitCheckoutVersionFindsLightweightTag is the regression this helper
// exists for. cb-tumblebug's release tags are lightweight, and `git describe
// --exact-match` without --tags only ever sees annotated tags — so dropping
// --tags makes a correctly checked-out release look like a bare commit.
func TestGitCheckoutVersionFindsLightweightTag(t *testing.T) {
	dir, firstCommit, _ := gitRepoWithTags(t)
	runGit(t, dir, "checkout", "-q", "v0.12.22")

	tag, commit, err := GitCheckoutVersion(dir)
	if err != nil {
		t.Fatalf("GitCheckoutVersion: %v", err)
	}
	if tag != "v0.12.22" {
		t.Errorf("lightweight tag not resolved: got tag %q, want %q", tag, "v0.12.22")
	}
	if commit != firstCommit {
		t.Errorf("commit = %q, want %q", commit, firstCommit)
	}
}

func TestGitCheckoutVersionFindsAnnotatedTag(t *testing.T) {
	dir, _, secondCommit := gitRepoWithTags(t)
	runGit(t, dir, "checkout", "-q", "v0.12.25")

	tag, commit, err := GitCheckoutVersion(dir)
	if err != nil {
		t.Fatalf("GitCheckoutVersion: %v", err)
	}
	if tag != "v0.12.25" {
		t.Errorf("tag = %q, want %q", tag, "v0.12.25")
	}
	if commit != secondCommit {
		t.Errorf("commit = %q, want %q", commit, secondCommit)
	}
}

// TestGitCheckoutVersionOffTag covers a checkout parked on a commit that
// carries no tag: that is reported as "no tag, this commit" rather than as an
// error, because the caller decides what to make of it.
func TestGitCheckoutVersionOffTag(t *testing.T) {
	dir, _, _ := gitRepoWithTags(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("three"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "third")
	head := runGit(t, dir, "rev-parse", "HEAD")

	tag, commit, err := GitCheckoutVersion(dir)
	if err != nil {
		t.Fatalf("GitCheckoutVersion: %v", err)
	}
	if tag != "" {
		t.Errorf("tag = %q, want empty for an untagged commit", tag)
	}
	if commit != head {
		t.Errorf("commit = %q, want %q", commit, head)
	}
}

func TestGitCheckoutVersionRejectsNonRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := GitCheckoutVersion(dir); err == nil {
		t.Fatal("a plain directory was accepted as a git repository")
	}
}

func TestGitWorkTreeClean(t *testing.T) {
	dir, _, _ := gitRepoWithTags(t)

	clean, err := GitWorkTreeClean(dir)
	if err != nil {
		t.Fatalf("GitWorkTreeClean: %v", err)
	}
	if !clean {
		t.Error("a freshly committed checkout was reported dirty")
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean, err = GitWorkTreeClean(dir)
	if err != nil {
		t.Fatalf("GitWorkTreeClean: %v", err)
	}
	if clean {
		t.Error("a checkout with a modified tracked file was reported clean")
	}
}

func TestGitTagExists(t *testing.T) {
	dir, _, _ := gitRepoWithTags(t)

	if !GitTagExists(dir, "v0.12.22") {
		t.Error("existing lightweight tag not found")
	}
	if !GitTagExists(dir, "v0.12.25") {
		t.Error("existing annotated tag not found")
	}
	if GitTagExists(dir, "v9.9.9") {
		t.Error("absent tag reported as present")
	}
	// `git tag -l v0.12.2` must not match v0.12.22 — a prefix is not a tag.
	if GitTagExists(dir, "v0.12.2") {
		t.Error("a tag prefix was accepted as a tag")
	}
}
