package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
// version of a single service — its verdict would otherwise be an artefact of
// the failures rather than a finding, and the caller puts a confirmation prompt
// in front of the user based on it.
//
// This is the branch that used to be unreachable: the function returned a
// literal nil on every path, so the caller's `if err != nil` was dead code.
func TestCheckVersionUpdatesReportsTotalLocalFailure(t *testing.T) {
	withStubbedPath(t, writeStubDocker(t, 1))
	withComposeFile(t, "services:\n  cb-spider:\n    image: cloudbaristaorg/cb-spider:1.0.0\n")
	withStubbedHub(t, map[string]dockerHubTag{})

	services := map[string]ServiceInfo{
		"cb-spider":    {Image: "cloudbaristaorg/cb-spider:1.0.0"},
		"cb-tumblebug": {Image: "cloudbaristaorg/cb-tumblebug:0.12.9"},
	}

	stale, err := checkVersionUpdates(services)
	if err == nil {
		t.Fatal("checkVersionUpdates should report an error when no local version could be read")
	}
	if len(stale) != 0 {
		t.Errorf("no service may be named for update when the check failed, got: %v", stale)
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
	withStubbedHub(t, map[string]dockerHubTag{})

	// No service carries an image:tag, so nothing is examined and there is no
	// failure to report.
	stale, err := checkVersionUpdates(map[string]ServiceInfo{
		"local-build": {Image: ""},
	})
	if err != nil {
		t.Fatalf("nothing to examine should not be an error, got: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("nothing was examined, so nothing may be named for update, got: %v", stale)
	}
}

// ───────────────────────── update scope ─────────────────────────

// withStubbedHub replaces the Docker Hub lookup for one test. Without it the
// version check reaches the network, which makes these tests slow and dependent
// on what happens to be published at the time.
func withStubbedHub(t *testing.T, byImage map[string]dockerHubTag) {
	t.Helper()
	prev := fetchTagInfo
	fetchTagInfo = func(imageName, tag string) (dockerHubTag, error) {
		info, ok := byImage[imageName+":"+tag]
		if !ok {
			return dockerHubTag{}, errors.New("stub: tag not published")
		}
		return info, nil
	}
	t.Cleanup(func() { fetchTagInfo = prev })
}

// writeStubDockerWithState installs a fake `docker` that answers the three
// queries the version check makes: the running containers (compose ps), the
// locally held tag (images), and the digest that tag points at (image inspect).
//
// runningTags and localDigests are keyed by service name and by image:tag.
func writeStubDockerWithState(t *testing.T, runningTags map[string]string, localDigests map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	var psLines []string
	for service, image := range runningTags {
		psLines = append(psLines, fmt.Sprintf(
			`{"Name":"%s","Service":"%s","State":"running","Image":"%s"}`, service, service, image))
	}
	sort.Strings(psLines)

	var digestCases strings.Builder
	for ref, digest := range localDigests {
		digestCases.WriteString(fmt.Sprintf("    %q) printf '%%s\\n' 'repo@%s' ;;\n", ref, digest))
	}

	body := fmt.Sprintf(`#!/bin/sh
case "$1" in
  compose)
    # only `+"`ps --format json`"+` is asked for here
    cat <<'PS'
%s
PS
    ;;
  images)
    # docker images --format {{.Tag}} <repo>:<tag>  → the tag when it is held locally
    ref="$4"
    printf '%%s\n' "${ref##*:}"
    ;;
  image)
    # docker image inspect <ref> --format ...
    case "$3" in
%s    *) printf '' ;;
    esac
    ;;
esac
exit 0
`, strings.Join(psLines, "\n"), digestCases.String())

	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Only the services that actually need updating come back.
//
// This is the defect this change exists for: the caller uses the returned names
// as the scope of the pull and the restart, so a service the table showed as
// unchanged must not appear. It previously returned a single bool, and a run
// without -s went on to pull and recreate everything.
func TestCheckVersionUpdatesReturnsOnlyStaleServices(t *testing.T) {
	withComposeFile(t, "services:\n  old-tag:\n    image: example/old-tag:2.0.0\n  current:\n    image: example/current:1.0.0\n")
	withStubbedPath(t, writeStubDockerWithState(t,
		map[string]string{
			"old-tag": "example/old-tag:1.0.0", // compose asks for 2.0.0
			"current": "example/current:1.0.0", // matches compose
		},
		map[string]string{
			"example/current:1.0.0": "sha256:same",
		}))
	withStubbedHub(t, map[string]dockerHubTag{
		"example/old-tag:2.0.0": {LastUpdated: "2026-07-20T00:00:00Z", Digest: "sha256:newtag"},
		"example/current:1.0.0": {LastUpdated: "2026-07-01T00:00:00Z", Digest: "sha256:same"},
	})

	stale, err := checkVersionUpdates(map[string]ServiceInfo{
		"old-tag": {Image: "example/old-tag:2.0.0"},
		"current": {Image: "example/current:1.0.0"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"old-tag"}; !reflect.DeepEqual(stale, want) {
		t.Errorf("only the out-of-date service may be named for update\n got: %v\nwant: %v", stale, want)
	}
}

// A tag that keeps its name across rebuilds — edge, latest — is caught by
// comparing the content it points at.
//
// Comparing names alone can never mark such a tag stale, which is why a new edge
// build sat unnoticed behind "All services are up to date!".
func TestCheckVersionUpdatesFlagsMovingTagByDigest(t *testing.T) {
	withComposeFile(t, "services:\n  moving:\n    image: example/moving:edge\n")
	withStubbedPath(t, writeStubDockerWithState(t,
		map[string]string{"moving": "example/moving:edge"},
		map[string]string{"example/moving:edge": "sha256:oldcontent"}))
	withStubbedHub(t, map[string]dockerHubTag{
		"example/moving:edge": {LastUpdated: "2026-07-23T04:09:46Z", Digest: "sha256:newcontent"},
	})

	stale, err := checkVersionUpdates(map[string]ServiceInfo{
		"moving": {Image: "example/moving:edge"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"moving"}; !reflect.DeepEqual(stale, want) {
		t.Errorf("a moving tag whose content changed needs updating\n got: %v\nwant: %v", stale, want)
	}
}

// An unavailable digest is not evidence of a difference.
//
// Docker Hub being unreachable, or a locally built image carrying no
// RepoDigests, must leave the service alone. Treating "cannot tell" as "changed"
// would restart the whole environment whenever the network hiccups.
func TestCheckVersionUpdatesKeepsMovingTagWhenDigestUnknown(t *testing.T) {
	withComposeFile(t, "services:\n  moving:\n    image: example/moving:edge\n")
	withStubbedPath(t, writeStubDockerWithState(t,
		map[string]string{"moving": "example/moving:edge"},
		map[string]string{})) // no local digest available
	withStubbedHub(t, map[string]dockerHubTag{
		"example/moving:edge": {LastUpdated: "2026-07-23T04:09:46Z", Digest: "sha256:newcontent"},
	})

	stale, err := checkVersionUpdates(map[string]ServiceInfo{
		"moving": {Image: "example/moving:edge"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("an undeterminable digest must not restart the service, got: %v", stale)
	}
}

// A service whose local version could not be read is left out of the update.
//
// It used to be counted as stale — "unknown" simply differed from the compose
// tag — so one docker hiccup pulled a service into a restart on no evidence.
func TestCheckVersionUpdatesSkipsServiceWithUnreadableLocalVersion(t *testing.T) {
	withComposeFile(t, "services:\n  readable:\n    image: example/readable:1.0.0\n  broken:\n    image: example/broken:1.0.0\n")

	dir := t.TempDir()
	body := `#!/bin/sh
case "$1" in
  compose) printf '{"Name":"readable","Service":"readable","State":"running","Image":"example/readable:1.0.0"}\n' ;;
  images)
    # the broken service is the one whose local lookup fails
    case "$4" in
      example/broken:*) exit 1 ;;
      *) printf '%s\n' "${4##*:}" ;;
    esac
    ;;
  image) printf 'repo@sha256:same\n' ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	withStubbedPath(t, dir)
	withStubbedHub(t, map[string]dockerHubTag{
		"example/readable:1.0.0": {LastUpdated: "2026-07-01T00:00:00Z", Digest: "sha256:same"},
		"example/broken:1.0.0":   {LastUpdated: "2026-07-01T00:00:00Z", Digest: "sha256:same"},
	})

	stale, err := checkVersionUpdates(map[string]ServiceInfo{
		"readable": {Image: "example/readable:1.0.0"},
		"broken":   {Image: "example/broken:1.0.0"},
	})
	if err != nil {
		t.Fatalf("one readable service means the check still stands, got: %v", err)
	}
	for _, name := range stale {
		if name == "broken" {
			t.Error("a service whose local version could not be read must not be updated on that basis")
		}
	}
}

// shortDate trims the timestamp to the day; anything unexpected passes through.
func TestShortDate(t *testing.T) {
	if got := shortDate("2026-07-23T04:09:46.017983Z"); got != "2026-07-23" {
		t.Errorf("got %q, want 2026-07-23", got)
	}
	if got := shortDate(""); got != "" {
		t.Errorf("an empty timestamp should stay empty, got %q", got)
	}
}
