package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestCompose creates a minimal compose file declaring the given services
// and points DockerFilePath at it for the duration of the test.
func writeTestCompose(t *testing.T, services ...string) string {
	t.Helper()

	var b strings.Builder
	b.WriteString("services:\n")
	for _, s := range services {
		b.WriteString("  " + s + ":\n")
		b.WriteString("    image: example/" + s + ":1.0.0\n")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yaml")
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}

	prev := DockerFilePath
	DockerFilePath = path
	t.Cleanup(func() { DockerFilePath = prev })
	return path
}

// Comma, space and mixed separators must all produce the same list, with blanks
// dropped and duplicates collapsed in first-seen order. Each command used to
// split -s its own way, so the same value meant different things depending on
// which subcommand read it.
func TestSplitServiceNames(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"single", "cb-spider", []string{"cb-spider"}},
		{"comma", "cb-spider,cb-tumblebug", []string{"cb-spider", "cb-tumblebug"}},
		{"space", "cb-spider cb-tumblebug", []string{"cb-spider", "cb-tumblebug"}},
		{"comma and space", "cb-spider, cb-tumblebug", []string{"cb-spider", "cb-tumblebug"}},
		{"mixed separators", "cb-spider, cb-tumblebug openbao", []string{"cb-spider", "cb-tumblebug", "openbao"}},
		{"surrounding whitespace", "  cb-spider ,cb-tumblebug  ", []string{"cb-spider", "cb-tumblebug"}},
		{"empty fields dropped", "cb-spider,,,cb-tumblebug", []string{"cb-spider", "cb-tumblebug"}},
		{"duplicates removed, order kept", "cb-tumblebug,cb-spider,cb-tumblebug", []string{"cb-tumblebug", "cb-spider"}},
		{"separators only", ",", []string{}},
		{"whitespace only", "   ", []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitServiceNames(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("splitServiceNames(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("splitServiceNames(%q) = %v, want %v", tc.raw, got, tc.want)
				}
			}
		})
	}
}

// The whole environment may be selected only by omitting -s. This is the
// distinction every destructive command depends on.
func TestResolveServicesEmptyMeansAll(t *testing.T) {
	writeTestCompose(t, "cb-spider", "openbao")

	got, err := resolveServices("")
	if err != nil {
		t.Fatalf("resolveServices(\"\") returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("resolveServices(\"\") = %v, want empty (all services)", got)
	}
}

// A -s value that holds nothing but separators must be an error, never "all
// services". `remove -s ","` used to fall through to the whole-environment
// branch and wipe every service's host data.
func TestResolveServicesSeparatorOnlyIsAnError(t *testing.T) {
	writeTestCompose(t, "cb-spider", "cb-tumblebug", "openbao")

	for _, raw := range []string{",", " ", "  ,  ", ",,,", "\t"} {
		got, err := resolveServices(raw)
		if err == nil {
			t.Fatalf("resolveServices(%q) = %v, want an error — a separator-only value must never mean 'all services'", raw, got)
		}
		if got != nil {
			t.Fatalf("resolveServices(%q) returned %v alongside an error; it must return no targets", raw, got)
		}
		if !strings.Contains(err.Error(), "no service name found") {
			t.Fatalf("resolveServices(%q) error = %q, want it to explain that no service was named", raw, err)
		}
	}
}

// Backward compatibility: a single valid name keeps working unchanged.
func TestResolveServicesSingleName(t *testing.T) {
	writeTestCompose(t, "cb-spider", "cb-tumblebug")

	got, err := resolveServices("cb-spider")
	if err != nil {
		t.Fatalf("resolveServices: %v", err)
	}
	if len(got) != 1 || got[0] != "cb-spider" {
		t.Fatalf("resolveServices(\"cb-spider\") = %v, want [cb-spider]", got)
	}
}

// Comma, space and mixed input must resolve identically once validated.
func TestResolveServicesSeparatorsAreEquivalent(t *testing.T) {
	writeTestCompose(t, "cb-spider", "cb-tumblebug", "openbao")

	want := []string{"cb-spider", "cb-tumblebug"}
	for _, raw := range []string{
		"cb-spider,cb-tumblebug",
		"cb-spider cb-tumblebug",
		"cb-spider, cb-tumblebug",
		" cb-spider ,  cb-tumblebug ",
	} {
		got, err := resolveServices(raw)
		if err != nil {
			t.Fatalf("resolveServices(%q): %v", raw, err)
		}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("resolveServices(%q) = %v, want %v", raw, got, want)
		}
	}
}

// An unknown name must be named on its own. Reporting the whole -s string left
// the user to guess which of several entries was wrong.
func TestResolveServicesNamesTheUnknownEntryOnly(t *testing.T) {
	writeTestCompose(t, "cb-spider", "cb-tumblebug", "openbao")

	_, err := resolveServices("cb-spider,cb-tumbelbug,openbao")
	if err == nil {
		t.Fatal("resolveServices accepted a misspelled service name")
	}
	msg := err.Error()
	if !strings.Contains(msg, "cb-tumbelbug") {
		t.Fatalf("error must point at the offending name; got %q", msg)
	}
	if strings.Contains(msg, "'cb-spider'") || strings.Contains(msg, "'openbao'") {
		t.Fatalf("error must not blame the valid names; got %q", msg)
	}
	if !strings.Contains(msg, "Available services:") {
		t.Fatalf("error must list the available services; got %q", msg)
	}
	for _, svc := range []string{"cb-spider", "cb-tumblebug", "openbao"} {
		if !strings.Contains(msg, "  - "+svc) {
			t.Fatalf("available list is missing %q; got %q", svc, msg)
		}
	}
}

// Several unknown names are all reported at once, so the user fixes the value
// in one pass rather than one name per run.
func TestResolveServicesReportsEveryUnknownName(t *testing.T) {
	writeTestCompose(t, "cb-spider", "openbao")

	_, err := resolveServices("nope-one nope-two")
	if err == nil {
		t.Fatal("resolveServices accepted two unknown service names")
	}
	for _, name := range []string{"nope-one", "nope-two"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error must mention %q; got %q", name, err)
		}
	}
}

// Shell metacharacters and path traversal cannot name a compose service, so
// they are rejected before any command or removal path is built. This is what
// makes `remove -s 'openbao;id'` and `remove -s '../../..'` structurally
// impossible rather than merely quoted.
func TestResolveServicesRejectsInjectionAndTraversal(t *testing.T) {
	writeTestCompose(t, "openbao", "cb-spider")

	for _, raw := range []string{
		"openbao;id",
		"openbao|id",
		"openbao$(id)",
		"openbao`id`",
		"../../..",
		"../../../etc",
		"openbao && rm -rf /",
	} {
		got, err := resolveServices(raw)
		if err == nil {
			t.Fatalf("resolveServices(%q) = %v, want an error — the value does not name a compose service", raw, got)
		}
	}
}

// validateServiceNames is the pure half of the resolver; a fully valid list
// passes through untouched.
func TestValidateServiceNamesPassesValidInput(t *testing.T) {
	available := map[string]ServiceInfo{
		"cb-spider": {Name: "cb-spider"},
		"openbao":   {Name: "openbao"},
	}
	got, err := validateServiceNames([]string{"openbao", "cb-spider"}, available)
	if err != nil {
		t.Fatalf("validateServiceNames: %v", err)
	}
	if len(got) != 2 || got[0] != "openbao" || got[1] != "cb-spider" {
		t.Fatalf("validateServiceNames = %v, want the input order preserved", got)
	}
}

// Every service the human-readable table knows about must be resolvable by -s.
// The two lists are maintained separately today, so a name present in one and
// missing from the other would make `-s <name>` reject a service that `info`
// happily displays.
func TestComposeFileCoversDisplayedServices(t *testing.T) {
	prev := DockerFilePath
	DockerFilePath = filepath.Join("..", "..", "conf", "docker", "docker-compose.yaml")
	defer func() { DockerFilePath = prev }()

	parsed, err := parseDockerComposeImages()
	if err != nil {
		t.Fatalf("failed to parse the real compose file: %v", err)
	}

	for _, svc := range getServicesFromCompose() {
		if _, ok := parsed[svc]; !ok {
			t.Errorf("service %q is displayed by `infra info` but cannot be resolved by -s", svc)
		}
	}
}

// The removal targets must stay inside conf/docker/data. resolveServices already
// prevents a traversal value from reaching here, so this guards the path itself.
func TestAssertUnderDataRoot(t *testing.T) {
	root := filepath.Join("conf", "docker", "data")

	ok := []string{
		filepath.Join(root, "openbao"),
		filepath.Join(root, "cb-spider"),
		filepath.Join(root, "a", "b"),
	}
	for _, target := range ok {
		if err := assertUnderDataRoot(root, target); err != nil {
			t.Errorf("assertUnderDataRoot(%q) = %v, want it accepted", target, err)
		}
	}

	bad := []string{
		root,                                  // the data root itself
		filepath.Join(root, ".."),             // conf/docker
		filepath.Join(root, "..", ".."),       // conf
		filepath.Join(root, "..", "..", ".."), // the repository
		"/etc",
		filepath.Join(root, "..", "..", "..", "..", "etc"),
	}
	for _, target := range bad {
		if err := assertUnderDataRoot(root, target); err == nil {
			t.Errorf("assertUnderDataRoot(%q) accepted a target outside the data directory", target)
		}
	}
}

// buildComposeCommands must emit argument vectors, with each service as its own
// element. A name is never concatenated into a string that a shell would parse.
func TestBuildComposeCommandsUsesArgumentVectors(t *testing.T) {
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	defer func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll }()

	// Whole environment, default flags → a single `down`.
	cleanDBFlag, cleanAllFlag = false, false
	cmds := buildComposeCommands(nil)
	if len(cmds) != 1 {
		t.Fatalf("whole-environment removal should be one command, got %d", len(cmds))
	}
	if cmds[0][0] != "down" {
		t.Fatalf("whole-environment removal should use `down`, got %v", cmds[0])
	}

	// Service-scoped → stop + rm, each service its own argument.
	cleanDBFlag, cleanAllFlag = true, false
	cmds = buildComposeCommands([]string{"cb-spider", "openbao"})
	if len(cmds) != 2 {
		t.Fatalf("service-scoped removal should be stop + rm, got %d commands", len(cmds))
	}
	if cmds[0][0] != "stop" || cmds[1][0] != "rm" {
		t.Fatalf("expected stop then rm, got %v and %v", cmds[0], cmds[1])
	}
	for _, c := range cmds {
		var found int
		for _, arg := range c {
			if arg == "cb-spider" || arg == "openbao" {
				found++
			}
			if strings.Contains(arg, " ") {
				t.Errorf("argument %q contains a space; service names must be separate arguments", arg)
			}
		}
		if found != 2 {
			t.Errorf("command %v should carry both service names as separate arguments", c)
		}
	}
}

// A service-scoped --clean-db wipes only the named services' directories.
func TestHostDataTargetsServiceScoped(t *testing.T) {
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	defer func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll }()
	cleanDBFlag, cleanAllFlag = true, false

	writeTestCompose(t, "cb-spider", "openbao")

	targets, err := hostDataTargets([]string{"cb-spider"})
	if err != nil {
		t.Fatalf("hostDataTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("hostDataTargets = %v, want exactly one target", targets)
	}
	if filepath.Base(targets[0]) != "cb-spider" {
		t.Fatalf("hostDataTargets = %v, want the cb-spider data directory", targets)
	}
}

// Without --clean-db/--clean-all nothing on the host is touched.
func TestHostDataTargetsDefaultWipesNothing(t *testing.T) {
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	defer func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll }()
	cleanDBFlag, cleanAllFlag = false, false

	targets, err := hostDataTargets([]string{"cb-spider"})
	if err != nil {
		t.Fatalf("hostDataTargets: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("hostDataTargets = %v, want nothing wiped without --clean-db", targets)
	}
}

// setServiceNames points the -s flag values at the given occurrences for the
// duration of the test.
func setServiceNames(t *testing.T, values ...string) {
	t.Helper()
	prev := ServiceNames
	ServiceNames = values
	t.Cleanup(func() { ServiceNames = prev })
}

// -s is repeatable and each occurrence may itself list several names, so
// repeating the flag, separating with commas, separating with spaces, and mixing
// the two must all select the same services. Before -s became repeatable only
// the last occurrence survived, so a user who wrote `-s a -s b` silently got
// just b — a targeted remove could then hit a narrower set than intended without
// any error.
func TestResolveSelectedServicesFormsAreEquivalent(t *testing.T) {
	writeTestCompose(t, "cb-spider", "cb-tumblebug", "openbao")

	cases := []struct {
		name   string
		values []string
		want   []string
	}{
		{"repeated flag", []string{"cb-spider", "cb-tumblebug"}, []string{"cb-spider", "cb-tumblebug"}},
		{"comma separated", []string{"cb-spider,cb-tumblebug"}, []string{"cb-spider", "cb-tumblebug"}},
		{"space separated", []string{"cb-spider cb-tumblebug"}, []string{"cb-spider", "cb-tumblebug"}},
		{"repeat mixed with comma", []string{"cb-spider,cb-tumblebug", "openbao"}, []string{"cb-spider", "cb-tumblebug", "openbao"}},
		{"duplicates across occurrences collapse", []string{"cb-spider", "cb-spider,cb-tumblebug"}, []string{"cb-spider", "cb-tumblebug"}},
		{"single value unchanged", []string{"cb-spider"}, []string{"cb-spider"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setServiceNames(t, tc.values...)

			got, err := resolveSelectedServices()
			if err != nil {
				t.Fatalf("resolveSelectedServices() with -s %v returned error: %v", tc.values, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("resolveSelectedServices() with -s %v = %v, want %v", tc.values, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("resolveSelectedServices() with -s %v = %v, want %v", tc.values, got, tc.want)
				}
			}
		})
	}
}

// Omitting -s is still the only way to mean "every service"; the safety rules
// that guard destructive commands must survive the switch to a repeatable flag.
func TestResolveSelectedServicesSafetyRules(t *testing.T) {
	writeTestCompose(t, "cb-spider", "openbao")

	t.Run("omitted means all", func(t *testing.T) {
		setServiceNames(t)

		got, err := resolveSelectedServices()
		if err != nil {
			t.Fatalf("resolveSelectedServices() with no -s returned error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("resolveSelectedServices() with no -s = %v, want empty (all services)", got)
		}
	})

	t.Run("separators only is an error, never all", func(t *testing.T) {
		setServiceNames(t, ",", " ")

		got, err := resolveSelectedServices()
		if err == nil {
			t.Fatalf("resolveSelectedServices() with separator-only -s = %v, want an error", got)
		}
		if got != nil {
			t.Fatalf("resolveSelectedServices() returned %v alongside an error; it must return no targets", got)
		}
	})

	t.Run("unknown name is reported", func(t *testing.T) {
		setServiceNames(t, "cb-spider", "nope-one")

		if _, err := resolveSelectedServices(); err == nil {
			t.Fatal("resolveSelectedServices() with an unknown name returned no error")
		} else if !strings.Contains(err.Error(), "nope-one") {
			t.Fatalf("error %q does not name the offending service", err)
		}
	})
}
