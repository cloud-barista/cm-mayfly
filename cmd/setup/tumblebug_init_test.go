package setup

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

// writeFile writes name inside dir, creating or overwriting it.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// testRepo builds a throwaway repository shaped like a checked-out
// cb-tumblebug clone: two commits, a lightweight tag on the first and an
// annotated tag on the second.
//
// cb-tumblebug publishes its release tags as lightweight tags, so the
// lightweight case is the one that matters in production; the annotated tag
// is kept alongside it to prove both kinds resolve.
type testRepo struct {
	dir            string
	firstCommit    string
	secondCommit   string
	lightweightTag string
	annotatedTag   string
}

func newTestRepo(t *testing.T) testRepo {
	t.Helper()
	requireGit(t)

	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "--initial-branch=main")

	writeFile(t, dir, "README.md", "first\n")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "--quiet", "-m", "first commit")
	first := runGit(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "tag", "v1.2.3")

	writeFile(t, dir, "README.md", "second\n")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "--quiet", "-m", "second commit")
	second := runGit(t, dir, "rev-parse", "HEAD")
	runGit(t, dir, "tag", "-a", "v1.2.4", "-m", "release v1.2.4")

	writeFile(t, dir, "README.md", "third\n")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "--quiet", "-m", "third commit")

	return testRepo{
		dir:            dir,
		firstCommit:    first,
		secondCommit:   second,
		lightweightTag: "v1.2.3",
		annotatedTag:   "v1.2.4",
	}
}

// TestGetExistingTumblebugVersionLightweightTag is the regression test for the
// version detection bug: `git describe --exact-match` without --tags only
// considers annotated tags, so a correctly checked-out cb-tumblebug release
// reported its commit hash and the caller kept asking the user to switch
// versions on every run.
func TestGetExistingTumblebugVersionLightweightTag(t *testing.T) {
	repo := newTestRepo(t)
	runGit(t, repo.dir, "checkout", "--quiet", repo.lightweightTag)

	got, err := getExistingTumblebugVersion(repo.dir)
	if err != nil {
		t.Fatalf("getExistingTumblebugVersion: %v", err)
	}
	if got != repo.lightweightTag {
		t.Fatalf("expected the lightweight tag %q, got %q (a commit hash here means --tags was dropped)", repo.lightweightTag, got)
	}
}

func TestGetExistingTumblebugVersionAnnotatedTag(t *testing.T) {
	repo := newTestRepo(t)
	runGit(t, repo.dir, "checkout", "--quiet", repo.annotatedTag)

	got, err := getExistingTumblebugVersion(repo.dir)
	if err != nil {
		t.Fatalf("getExistingTumblebugVersion: %v", err)
	}
	if got != repo.annotatedTag {
		t.Fatalf("expected the annotated tag %q, got %q", repo.annotatedTag, got)
	}
}

// TestGetExistingTumblebugVersionUntaggedBranchHead checks that an untagged
// branch tip still reports its commit hash, so the caller correctly treats it
// as a different version.
func TestGetExistingTumblebugVersionUntaggedBranchHead(t *testing.T) {
	repo := newTestRepo(t)

	got, err := getExistingTumblebugVersion(repo.dir)
	if err != nil {
		t.Fatalf("getExistingTumblebugVersion: %v", err)
	}
	head := runGit(t, repo.dir, "rev-parse", "HEAD")
	if got != head {
		t.Fatalf("expected the commit hash %q for an untagged branch head, got %q", head, got)
	}
	if strings.HasPrefix(got, "v") {
		t.Fatalf("an untagged head must not resolve to a tag, got %q", got)
	}
}

// TestGetExistingTumblebugVersionUntaggedIntermediateCommit checks that a
// commit sitting between tags is not reported as the nearest tag: --exact-match
// must keep describe from walking back through history.
func TestGetExistingTumblebugVersionUntaggedIntermediateCommit(t *testing.T) {
	repo := newTestRepo(t)

	writeFile(t, repo.dir, "extra.txt", "untagged\n")
	runGit(t, repo.dir, "add", "extra.txt")
	runGit(t, repo.dir, "commit", "--quiet", "-m", "untagged commit")
	untagged := runGit(t, repo.dir, "rev-parse", "HEAD")

	got, err := getExistingTumblebugVersion(repo.dir)
	if err != nil {
		t.Fatalf("getExistingTumblebugVersion: %v", err)
	}
	if got != untagged {
		t.Fatalf("expected the commit hash %q for an untagged commit, got %q", untagged, got)
	}
}

// TestGetExistingTumblebugVersionDirtyWorktree checks that uncommitted changes
// do not hide the tag: users edit files inside the clone, and the version must
// still resolve from HEAD.
func TestGetExistingTumblebugVersionDirtyWorktree(t *testing.T) {
	repo := newTestRepo(t)
	runGit(t, repo.dir, "checkout", "--quiet", repo.lightweightTag)
	writeFile(t, repo.dir, "README.md", "locally modified\n")

	got, err := getExistingTumblebugVersion(repo.dir)
	if err != nil {
		t.Fatalf("getExistingTumblebugVersion: %v", err)
	}
	if got != repo.lightweightTag {
		t.Fatalf("expected the lightweight tag %q despite local changes, got %q", repo.lightweightTag, got)
	}
}

// TestGetExistingTumblebugVersionNotAGitRepository checks the guard for a
// directory that exists but was never cloned.
func TestGetExistingTumblebugVersionNotAGitRepository(t *testing.T) {
	if _, err := getExistingTumblebugVersion(t.TempDir()); err == nil {
		t.Fatal("expected an error for a directory that is not a git repository")
	}
}

// TestIsTagExistsInRepo checks the lookup that tells the user a release is
// present but not checked out. It must find lightweight tags as well, and must
// not match a tag name by prefix.
func TestIsTagExistsInRepo(t *testing.T) {
	repo := newTestRepo(t)

	cases := []struct {
		name string
		tag  string
		want bool
	}{
		{name: "lightweight tag", tag: repo.lightweightTag, want: true},
		{name: "annotated tag", tag: repo.annotatedTag, want: true},
		{name: "unknown tag", tag: "v9.9.9", want: false},
		{name: "prefix of an existing tag", tag: "v1.2", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTagExistsInRepo(repo.dir, tc.tag); got != tc.want {
				t.Fatalf("isTagExistsInRepo(%q) = %v, want %v", tc.tag, got, tc.want)
			}
		})
	}
}
