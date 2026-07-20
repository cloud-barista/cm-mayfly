package docker

import (
	"os"
	"path/filepath"
	"testing"
)

// withComposeFile points DockerFilePath at a temporary compose file for the
// duration of one test. Each call writes to a fresh directory so the parse
// cache, which is keyed on path plus size plus mtime, cannot serve one test's
// content to another.
func withComposeFile(t *testing.T, content string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "docker-compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write the test compose file: %v", err)
	}

	prev := DockerFilePath
	DockerFilePath = path
	t.Cleanup(func() { DockerFilePath = prev })
}

// A service that declares healthcheck or build before image must still be found.
// The line-oriented scanner this replaced looked for a bare "key:" line followed
// by an "image:" line, so it attributed the image to whichever bare key came
// last — and any service whose image was not its first key vanished, which the
// user then saw as "service not found in docker-compose.yaml".
func TestParseComposeFindsServicesWithImageNotFirst(t *testing.T) {
	withComposeFile(t, `
services:
  image-first:
    image: cloudbaristaorg/image-first:1.0.0
    container_name: image-first

  health-first:
    healthcheck:
      test: ["CMD", "true"]
    image: cloudbaristaorg/health-first:2.0.0

  build-first:
    build:
      context: .
    image: cloudbaristaorg/build-first:3.0.0

  no-image-at-all:
    build:
      context: .
`)

	parsed, err := loadComposeFile()
	if err != nil {
		t.Fatalf("loadComposeFile: %v", err)
	}

	want := map[string]string{
		"image-first":     "cloudbaristaorg/image-first:1.0.0",
		"health-first":    "cloudbaristaorg/health-first:2.0.0",
		"build-first":     "cloudbaristaorg/build-first:3.0.0",
		"no-image-at-all": "",
	}

	if len(parsed.Services) != len(want) {
		t.Fatalf("parsed %d services (%v), want %d", len(parsed.Services), parsed.Order, len(want))
	}
	for name, image := range want {
		svc, exists := parsed.Services[name]
		if !exists {
			t.Errorf("service %q is missing from the parse", name)
			continue
		}
		if svc.Image != image {
			t.Errorf("service %q image = %q, want %q", name, svc.Image, image)
		}
	}
}

// A commented-out image line belongs to no service. The compose file toggles
// between an edge tag and a pinned tag by commenting one of the two out, so
// picking up the wrong one would report a version that is not in use.
func TestParseComposeIgnoresCommentedImage(t *testing.T) {
	withComposeFile(t, `
services:
  cm-ant:
    # image: cloudbaristaorg/cm-ant:edge
    image: cloudbaristaorg/cm-ant:0.5.4
`)

	parsed, err := loadComposeFile()
	if err != nil {
		t.Fatalf("loadComposeFile: %v", err)
	}
	if got := parsed.Services["cm-ant"].Tag; got != "0.5.4" {
		t.Errorf("cm-ant tag = %q, want the uncommented 0.5.4", got)
	}
}

// Services are listed in the order the compose file declares them.
func TestParseComposePreservesOrder(t *testing.T) {
	withComposeFile(t, `
services:
  zulu:
    image: repo/zulu:1
  alpha:
    image: repo/alpha:1
  mike:
    image: repo/mike:1
`)

	got := getServicesFromCompose()
	want := []string{"zulu", "alpha", "mike"}

	if len(got) != len(want) {
		t.Fatalf("getServicesFromCompose() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("getServicesFromCompose() = %v, want %v", got, want)
		}
	}
}

// Keys nested under depends_on are dependency references, not services of
// their own, and must not appear in the service list.
func TestParseComposeExcludesDependsOnEntries(t *testing.T) {
	withComposeFile(t, `
services:
  cm-beetle:
    image: repo/cm-beetle:1
    depends_on:
      cb-tumblebug:
        condition: service_healthy
  cb-tumblebug:
    image: repo/cb-tumblebug:1
`)

	parsed, err := loadComposeFile()
	if err != nil {
		t.Fatalf("loadComposeFile: %v", err)
	}
	if len(parsed.Services) != 2 {
		t.Fatalf("parsed %v, want exactly cm-beetle and cb-tumblebug", parsed.Order)
	}
	deps := parsed.Services["cm-beetle"].DependsOn
	if len(deps) != 1 || deps[0] != "cb-tumblebug" {
		t.Errorf("cm-beetle depends_on = %v, want [cb-tumblebug]", deps)
	}
}

// depends_on has a short list form as well as the map form, and both must read.
func TestParseComposeReadsListFormDependsOn(t *testing.T) {
	withComposeFile(t, `
services:
  a:
    image: repo/a:1
    depends_on:
      - b
      - c
  b:
    image: repo/b:1
  c:
    image: repo/c:1
`)

	parsed, err := loadComposeFile()
	if err != nil {
		t.Fatalf("loadComposeFile: %v", err)
	}
	deps := parsed.Services["a"].DependsOn
	if len(deps) != 2 || deps[0] != "b" || deps[1] != "c" {
		t.Errorf("a depends_on = %v, want [b c]", deps)
	}
}

func TestSplitImageRef(t *testing.T) {
	cases := []struct {
		image      string
		repository string
		tag        string
	}{
		{"cloudbaristaorg/cm-ant:0.5.4", "cloudbaristaorg/cm-ant", "0.5.4"},
		{"postgres:16-alpine", "postgres", "16-alpine"},
		{"rustfs/rustfs:latest", "rustfs/rustfs", "latest"},
		{"postgres", "postgres", ""},
		// A colon before a slash is a registry port, not a tag separator.
		{"localhost:5000/cm-ant", "localhost:5000/cm-ant", ""},
		{"localhost:5000/cm-ant:0.5.4", "localhost:5000/cm-ant", "0.5.4"},
		// A digest reference pins the image without naming a tag.
		{"repo/name@sha256:abc123", "repo/name", ""},
		{"", "", ""},
	}

	for _, c := range cases {
		repository, tag := splitImageRef(c.image)
		if repository != c.repository || tag != c.tag {
			t.Errorf("splitImageRef(%q) = (%q, %q), want (%q, %q)",
				c.image, repository, tag, c.repository, c.tag)
		}
	}
}

// Dependencies are followed the whole way down. Reporting only the direct
// depends_on meant `info --human -s cm-cicada` showed cm-beetle but not the
// cb-tumblebug that cm-beetle cannot start without.
func TestGetDependencyServicesIsTransitive(t *testing.T) {
	withComposeFile(t, `
services:
  cm-cicada:
    image: repo/cm-cicada:1
    depends_on:
      cm-beetle:
        condition: service_healthy
  cm-beetle:
    image: repo/cm-beetle:1
    depends_on:
      cb-tumblebug:
        condition: service_healthy
  cb-tumblebug:
    image: repo/cb-tumblebug:1
    depends_on:
      cb-spider:
        condition: service_started
  cb-spider:
    image: repo/cb-spider:1
  unrelated:
    image: repo/unrelated:1
`)

	got := getDependencyServices([]string{"cm-cicada"})

	want := map[string]bool{"cm-beetle": true, "cb-tumblebug": true, "cb-spider": true}
	if len(got) != len(want) {
		t.Fatalf("getDependencyServices = %v, want the three transitive dependencies", got)
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("getDependencyServices returned %q, which cm-cicada does not depend on", name)
		}
	}
}

// A cycle in depends_on must end the walk rather than recurse forever.
func TestGetDependencyServicesHandlesCycle(t *testing.T) {
	withComposeFile(t, `
services:
  a:
    image: repo/a:1
    depends_on:
      - b
  b:
    image: repo/b:1
    depends_on:
      - c
  c:
    image: repo/c:1
    depends_on:
      - a
`)

	got := getDependencyServices([]string{"a"})

	// a is the requested service, so it is not reported as its own dependency
	// even though the cycle leads back to it.
	if len(got) != 2 {
		t.Fatalf("getDependencyServices = %v, want [b c]", got)
	}
	for _, name := range got {
		if name == "a" {
			t.Errorf("getDependencyServices returned the requested service %q as a dependency", name)
		}
	}
}

// The size column must describe the image the version column names. It used to
// match any repository *containing* a keyword and fall back to the first hit,
// so a row could read "16-alpine (Not Downloaded)" and "291MB" at once — the
// 291MB belonging to an unrelated postgres:14-alpine.
func TestGetImageInfoRequiresExactTagMatch(t *testing.T) {
	withComposeFile(t, `
services:
  cb-tumblebug-postgres:
    image: postgres:16-alpine
  cm-butterfly-db:
    image: postgres:14-alpine
`)

	// Stand in for the local docker image list.
	localImageOnce.Do(func() {})
	prev := localImageIndex
	localImageIndex = map[string]string{"postgres:14-alpine": "291MB"}
	defer func() { localImageIndex = prev }()

	images := getImageInfo()

	if _, exists := images["cb-tumblebug-postgres"]; exists {
		t.Errorf("cb-tumblebug-postgres reported a size, but postgres:16-alpine is not present locally")
	}
	if got := images["cm-butterfly-db"].Size; got != "291MB" {
		t.Errorf("cm-butterfly-db size = %q, want 291MB", got)
	}
}

// The VERSION cell always names the compose tag and marks whether that tag is
// what is running. It used to spell the state out instead ("0.12.9 (Not
// Downloaded)"), which doubled the column width and still never said which
// version was really up.
func TestHumanVersionCells(t *testing.T) {
	withComposeFile(t, `
services:
  present:
    image: repo/present:1.0.0
  absent:
    image: repo/absent:2.0.0
  imageless:
    build:
      context: .
`)

	localImageOnce.Do(func() {})
	prev := localImageIndex
	localImageIndex = map[string]string{"repo/present:1.0.0": "10MB"}
	defer func() { localImageIndex = prev }()

	cases := []struct {
		name       string
		service    string
		container  ContainerInfo
		exists     bool
		wantVer    string
		wantActual string
	}{
		{
			name:      "running on the compose version",
			service:   "present",
			container: ContainerInfo{Status: "running", Version: "1.0.0"},
			exists:    true,
			wantVer:   "1.0.0 ✓",
		},
		{
			name:       "running on some other version",
			service:    "present",
			container:  ContainerInfo{Status: "running", Version: "0.9.0"},
			exists:     true,
			wantVer:    "1.0.0 ✗",
			wantActual: "0.9.0",
		},
		{
			// A stopped container proves nothing about what would come up, so
			// its tag is not reported as actually running.
			name:      "stopped container is not a running version",
			service:   "present",
			container: ContainerInfo{Status: "Stopped", Version: "0.9.0"},
			exists:    true,
			wantVer:   "1.0.0 ✗",
		},
		{
			name:    "no container at all",
			service: "present",
			wantVer: "1.0.0 ✗",
		},
		{
			// Not downloaded is not spelled out; the mark already says the
			// service is not up on that version.
			name:    "image not present locally",
			service: "absent",
			wantVer: "2.0.0 ✗",
		},
		{
			name:    "service without an image tag",
			service: "imageless",
			wantVer: "-",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotVer, gotActual := humanVersionCells(c.service, c.container, c.exists)
			if gotVer != c.wantVer || gotActual != c.wantActual {
				t.Errorf("humanVersionCells(%q) = (%q, %q), want (%q, %q)",
					c.service, gotVer, gotActual, c.wantVer, c.wantActual)
			}
		})
	}
}

// Column widths are measured in terminal columns. Every branch of this function
// used to add 1, so the "Unicode" handling it advertised did nothing and a
// double-width character pushed the table border out by one per character.
func TestGetDisplayWidth(t *testing.T) {
	cases := []struct {
		s     string
		width int
	}{
		{"", 0},
		{"cb-tumblebug-postgres", 21},
		{"16-alpine (Not Downloaded)", 26},
		{"✓", 1},
		{"✗", 1},
		{"-", 1},
		{"한글", 4},   // two Hangul syllables, two columns each
		{"日本語", 6},  // three CJK ideographs
		{"a한b", 4},  // mixed ASCII and wide
		{"🎯", 2},    // emoji used in the section headings
		{"ｆｕｌｌ", 8}, // fullwidth Latin
	}

	for _, c := range cases {
		if got := getDisplayWidth(c.s); got != c.width {
			t.Errorf("getDisplayWidth(%q) = %d, want %d", c.s, got, c.width)
		}
	}
}

// Official Docker Hub images live under library/, even though they are pulled
// by a bare name. Without the prefix the API answers 404 for every one of them,
// which is why postgres, redis, mysql and etcd never showed a Latest value.
func TestDockerHubRepositoryPath(t *testing.T) {
	cases := map[string]string{
		"postgres":                  "library/postgres",
		"redis":                     "library/redis",
		"cloudbaristaorg/cm-ant":    "cloudbaristaorg/cm-ant",
		"rustfs/rustfs":             "rustfs/rustfs",
		"gcr.io/etcd-development/e": "gcr.io/etcd-development/e",
	}

	for image, want := range cases {
		if got := dockerHubRepositoryPath(image); got != want {
			t.Errorf("dockerHubRepositoryPath(%q) = %q, want %q", image, got, want)
		}
	}
}

// The OpenBao consistency verdict belongs in a report that covers OpenBao. It
// used to print unconditionally, so `info -s cm-ant` ended with a verdict about
// a component the user had not asked about.
func TestOpenbaoRelevant(t *testing.T) {
	if !openbaoRelevant(nil) {
		t.Error("openbaoRelevant(nil) = false, want true: no filter means the whole environment")
	}
	if !openbaoRelevant([]string{"cm-ant", "openbao-unseal"}) {
		t.Error("openbaoRelevant with openbao-unseal = false, want true")
	}
	if openbaoRelevant([]string{"cm-ant", "ant-postgres"}) {
		t.Error("openbaoRelevant without any OpenBao service = true, want false")
	}
}

// The real compose file must parse, and every service in it must be reachable
// through both the display list and -s resolution.
func TestRealComposeFileParses(t *testing.T) {
	prev := DockerFilePath
	DockerFilePath = filepath.Join("..", "..", "conf", "docker", "docker-compose.yaml")
	defer func() { DockerFilePath = prev }()

	parsed, err := loadComposeFile()
	if err != nil {
		t.Fatalf("failed to parse the real compose file: %v", err)
	}
	if len(parsed.Order) == 0 {
		t.Fatal("the real compose file parsed to zero services")
	}

	for _, name := range parsed.Order {
		svc := parsed.Services[name]
		if svc.Image == "" {
			t.Errorf("service %q has no image; check whether that is intended", name)
		}
		if svc.Tag == "" {
			t.Errorf("service %q has an image with no tag: %q", name, svc.Image)
		}
	}
}
