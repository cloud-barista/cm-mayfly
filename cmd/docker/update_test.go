package docker

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// withStubbedPath puts dir at the front of PATH for one test, so a stub `docker`
// executable is found instead of the real one.
func withStubbedPath(t *testing.T, dir string) {
	t.Helper()
	prev := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+prev)
}

// writeStubDocker installs a fake `docker` that exits with the given status.
func writeStubDocker(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A failing `docker images` must be reported, not reported as "not_installed".
//
// The two are different states and the command exits 0 with empty output for a
// genuinely absent image. Collapsing a broken daemon into "not_installed" made
// the version table claim every image was missing, which reads to a user as
// "everything needs updating".
func TestGetCurrentLocalVersionReportsDockerFailure(t *testing.T) {
	withStubbedPath(t, writeStubDocker(t, 1))
	withComposeFile(t, "services:\n  cb-spider:\n    image: cloudbaristaorg/cb-spider:1.0.0\n")

	_, err := getCurrentLocalVersion("cloudbaristaorg/cb-spider", "1.0.0", "cb-spider")
	if err == nil {
		t.Fatal("a failing `docker images` should be returned as an error, not swallowed as not_installed")
	}
	if !strings.Contains(err.Error(), "cb-spider") {
		t.Errorf("error should name the image it failed on, got: %v", err)
	}
}

// checkVersionUpdates must return an error when it could not read the local
// version of a single service — its hasUpdates would otherwise be an artefact of
// the failures rather than a finding, and the caller puts a confirmation prompt
// in front of the user based on it.
//
// This is the branch that used to be unreachable: the function returned a
// literal nil on every path, so the caller's `if err != nil` was dead code.
func TestCheckVersionUpdatesReportsTotalLocalFailure(t *testing.T) {
	withStubbedPath(t, writeStubDocker(t, 1))
	withComposeFile(t, "services:\n  cb-spider:\n    image: cloudbaristaorg/cb-spider:1.0.0\n")

	services := map[string]ServiceInfo{
		"cb-spider":    {Image: "cloudbaristaorg/cb-spider:1.0.0"},
		"cb-tumblebug": {Image: "cloudbaristaorg/cb-tumblebug:0.12.9"},
	}

	hasUpdates, err := checkVersionUpdates(services)
	if err == nil {
		t.Fatal("checkVersionUpdates should report an error when no local version could be read")
	}
	if hasUpdates {
		t.Error("hasUpdates must be false when the check failed; a caller may not act on it")
	}
	if !strings.Contains(err.Error(), "docker daemon") {
		t.Errorf("error should point at the likely cause, got: %v", err)
	}
}

// Services with no parseable image reference are skipped rather than counted as
// failures — otherwise a compose file full of build-only services would look
// like a broken daemon.
func TestCheckVersionUpdatesIgnoresServicesWithoutAnImageTag(t *testing.T) {
	withStubbedPath(t, writeStubDocker(t, 1))
	withComposeFile(t, "services:\n  local-build:\n    build: .\n")

	// No service carries an image:tag, so nothing is examined and there is no
	// failure to report.
	hasUpdates, err := checkVersionUpdates(map[string]ServiceInfo{
		"local-build": {Image: ""},
	})
	if err != nil {
		t.Fatalf("nothing to examine should not be an error, got: %v", err)
	}
	if hasUpdates {
		t.Error("hasUpdates should be false when no service was examined")
	}
}
