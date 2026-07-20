package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitOutputInDir runs git with the given arguments inside dir and returns its
// standard output.
//
// The directory is handed to the process as its working directory rather than
// interpolated into a `cd <dir> && git …` shell command. Callers build the path
// from $HOME, and a shell would re-parse whatever that contains — a space alone
// breaks the unquoted form, and a $(…) sequence in it would be executed.
// Dropping the shell removes the question entirely.
func GitOutputInDir(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) // #nosec G204 -- arguments are passed as a vector, never re-parsed by a shell
	cmd.Dir = dir
	return cmd.Output()
}

// GitCheckoutVersion reports which release a git checkout is sitting on.
//
// It returns the tag HEAD points at exactly, and the HEAD commit hash. When
// HEAD is not on a tag the tag is empty and only the commit is filled in, so a
// caller can tell "on release X" from "on some arbitrary commit" without
// parsing anything. A directory that is not a git repository is an error.
//
// This is the single place the `--tags` rule is written down. cb-tumblebug
// publishes its release tags as *lightweight* tags, and `git describe
// --exact-match` on its own considers annotated tags only. Without --tags the
// tag is never found, so a correctly checked-out release still reports its
// commit hash — and every caller that compares that against a wanted version
// concludes the checkout is wrong. Both the interactive `setup tumblebug-init`
// flow and the unattended OpenBao initialization path call in here rather than
// spelling the git invocation out twice, because getting it wrong twice is
// exactly what happened before.
func GitCheckoutVersion(dir string) (tag string, commit string, err error) {
	gitDir := filepath.Join(dir, ".git")
	if _, statErr := os.Stat(gitDir); statErr != nil {
		return "", "", fmt.Errorf("%s is not a git repository", dir)
	}

	out, err := GitOutputInDir(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("could not read HEAD of %s: %w", dir, err)
	}
	commit = strings.TrimSpace(string(out))
	if commit == "" {
		return "", "", fmt.Errorf("could not read HEAD of %s: empty output", dir)
	}

	// --tags: see the note above. Removing it silently breaks every caller.
	out, err = GitOutputInDir(dir, "describe", "--exact-match", "--tags", "HEAD")
	if err != nil {
		// HEAD is not on a tag. That is a normal state, not a failure — the
		// caller decides what to do with a checkout that is off-release.
		return "", commit, nil
	}
	return strings.TrimSpace(string(out)), commit, nil
}

// GitWorkTreeClean reports whether a checkout has no uncommitted changes and no
// untracked files. Switching tags underneath local edits would either fail or
// carry them onto another release, so callers check this before checking out.
func GitWorkTreeClean(dir string) (bool, error) {
	out, err := GitOutputInDir(dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("could not read the status of %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)) == "", nil
}

// GitTagExists reports whether tag is present in the local checkout.
func GitTagExists(dir, tag string) bool {
	out, err := GitOutputInDir(dir, "tag", "-l", tag)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == tag
}
