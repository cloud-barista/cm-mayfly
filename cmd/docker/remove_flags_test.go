package docker

import (
	"strings"
	"testing"
)

// --clean-db and --clean-all must not be accepted together.
//
// They differ in exactly one thing — whether the OpenBao credentials survive —
// and the combination used to run --clean-all in silence. A user who typed
// --clean-db asked to keep those credentials and lost them with no message.
func TestRemoveRejectsCleanDBWithCleanAll(t *testing.T) {
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	t.Cleanup(func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll })

	// ParseFlags + ValidateFlagGroups is the pair cobra itself runs before a
	// command's Run. Calling Execute here would walk up to the root command and
	// run the real thing, which would touch containers.
	if err := removeCmd.ParseFlags([]string{"--clean-db", "--clean-all"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	err := removeCmd.ValidateFlagGroups()
	if err == nil {
		t.Fatal("--clean-db --clean-all was accepted; the two must be mutually exclusive")
	}
	// Cobra names both flags in the message, which is what tells the user which
	// pair conflicted.
	msg := err.Error()
	if !strings.Contains(msg, "clean-db") || !strings.Contains(msg, "clean-all") {
		t.Errorf("error should name both flags, got: %v", err)
	}
}

// Each flag on its own still parses — the exclusion must not have made the
// command unusable.
func TestRemoveAcceptsEachCleanFlagAlone(t *testing.T) {
	prevDB, prevAll, prevDry := cleanDBFlag, cleanAllFlag, dryRunFlag
	t.Cleanup(func() {
		cleanDBFlag, cleanAllFlag, dryRunFlag = prevDB, prevAll, prevDry
		removeCmd.SetArgs(nil)
	})

	for _, flag := range []string{"--clean-db", "--clean-all"} {
		t.Run(flag, func(t *testing.T) {
			cleanDBFlag, cleanAllFlag = false, false
			// Parse only: ParseFlags stops short of running the command, so no
			// containers are touched.
			if err := removeCmd.ParseFlags([]string{flag}); err != nil {
				t.Fatalf("%s alone should parse, got: %v", flag, err)
			}
		})
	}
}

// No paragraph may mention volumes.
//
// The compose file declares none — not one named volume, not one anonymous one;
// every service keeps its data in a bind mount under conf/docker/data/<service>.
// A message that lists volumes among the things it removes is describing work
// that has nothing to act on, and it does it in the sentence a user reads to
// decide whether their data is about to disappear.
func TestRemoveMessagesDoNotMentionVolumes(t *testing.T) {
	for name, msg := range map[string]string{
		"default":              msgRemoveDefault,
		"default -s":           msgRemoveDefaultService,
		"--clean-db":           msgRemoveCleanDB,
		"--clean-db -s":        msgRemoveCleanDBService,
		"--clean-all":          msgRemoveCleanAll,
		"--help (Long)":        removeCmd.Long,
		"--clean-db flag help": removeCmd.PersistentFlags().Lookup("clean-db").Usage,
	} {
		if strings.Contains(strings.ToLower(msg), "volume") {
			t.Errorf("%s mentions volumes, but the compose file declares none:\n%s", name, msg)
		}
	}
}

// Whether the project network survives is the clearest difference between a
// whole-system removal and a service-scoped one — `down` takes it away and
// `stop` + `rm` leaves it — and it used to appear in none of the paragraphs.
func TestRemoveMessagesStateNetworkHandling(t *testing.T) {
	for name, msg := range map[string]string{
		"default":       msgRemoveDefault,
		"default -s":    msgRemoveDefaultService,
		"--clean-db":    msgRemoveCleanDB,
		"--clean-db -s": msgRemoveCleanDBService,
		"--clean-all":   msgRemoveCleanAll,
	} {
		if !strings.Contains(strings.ToLower(msg), "network") {
			t.Errorf("%s does not say what happens to the project network:\n%s", name, msg)
		}
	}
}

// --clean-all no longer asks for a manual OpenBao initialization.
//
// It clears VAULT_TOKEN from .env, and `infra run` initializes OpenBao by
// itself when that key is empty — so telling the user to run the
// initialization again asks for work the command already arranged. What does
// need redoing by hand is tumblebug-init, whose credentials and assets the wipe
// really does destroy.
func TestCleanAllMessageDoesNotDemandManualOpenBaoInit(t *testing.T) {
	lower := strings.ToLower(msgRemoveCleanAll)
	if strings.Contains(lower, "must run the openbao initialization") {
		t.Error("--clean-all still demands a manual OpenBao initialization, which 'infra run' performs automatically")
	}
	if !strings.Contains(lower, "tumblebug-init") {
		t.Errorf("--clean-all should point at tumblebug-init, which does have to be redone:\n%s", msgRemoveCleanAll)
	}
}

// The default paths remove no data, and their paragraphs have to say where to
// go when removing data is what the user actually wanted. Without this the only
// route to --clean-db/--clean-all is --help, which is not where someone looks
// after a command they believe already deleted everything.
func TestDefaultMessagesPointAtTheDestructiveOptions(t *testing.T) {
	for name, msg := range map[string]string{
		"default":    msgRemoveDefault,
		"default -s": msgRemoveDefaultService,
	} {
		if !strings.Contains(msg, "--clean-db") {
			t.Errorf("%s does not mention --clean-db, so a user wanting the data gone has no way to find it:\n%s", name, msg)
		}
	}
	if !strings.Contains(msgRemoveDefault, "--clean-all") {
		t.Errorf("the default message should also name --clean-all:\n%s", msgRemoveDefault)
	}
}

// The service-scoped --clean-db paragraph says it removes the images, so the
// command has to actually remove them.
//
// It cannot do that through compose: `docker compose rm` has no --rmi, and the
// --rmi that exists belongs to `down`, which cannot be narrowed to a service.
// The images are therefore removed in a separate `docker rmi` step, and this
// test pins the pair together — the paragraph promising it, and the step that
// produces the targets.
func TestServiceScopedCleanDBRemovesImages(t *testing.T) {
	withComposeFile(t, `
services:
  cb-spider:
    image: cloudbaristaorg/cb-spider:0.12.35
  cm-butterfly-db:
    image: postgres:14-alpine
`)
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	t.Cleanup(func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll })
	cleanDBFlag, cleanAllFlag = true, false

	// The compose half stays as it was: stop + rm, no --rmi.
	cmds := buildComposeCommands([]string{"cb-spider"})
	var flat []string
	for _, c := range cmds {
		flat = append(flat, strings.Join(c, " "))
	}
	if joined := strings.Join(flat, " | "); strings.Contains(joined, "--rmi") {
		t.Fatalf("service-scoped removal should not pass --rmi to compose: %s", joined)
	}

	// The image half. The service name is not the image name, which is the
	// whole reason this has to be looked up rather than derived.
	got, err := imageRemovalTargets([]string{"cb-spider", "cm-butterfly-db"})
	if err != nil {
		t.Fatalf("imageRemovalTargets: %v", err)
	}
	want := []string{"cloudbaristaorg/cb-spider:0.12.35", "postgres:14-alpine"}
	if len(got) != len(want) {
		t.Fatalf("imageRemovalTargets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("imageRemovalTargets[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// And the paragraph must not have been left saying the opposite.
	lower := strings.ToLower(msgRemoveCleanDBService)
	if strings.Contains(lower, "images and shared networks are left in place") {
		t.Error("service-scoped message still says the images are kept, but they are now removed")
	}
	if !strings.Contains(lower, "their images") {
		t.Errorf("service-scoped message should state that the images are removed, got:\n%s", msgRemoveCleanDBService)
	}
}

// The paths that do not remove images must not produce any rmi targets.
//
// The whole-system paths already pass --rmi all to `docker compose down`;
// producing targets here as well would remove the same images twice, and the
// second attempt would report a failure for an image that is already gone.
func TestImageRemovalTargetsOnlyForServiceScopedCleanDB(t *testing.T) {
	withComposeFile(t, `
services:
  cb-spider:
    image: cloudbaristaorg/cb-spider:0.12.35
`)
	for _, tc := range []struct {
		name     string
		db, all  bool
		services []string
	}{
		{"whole system, no flags", false, false, nil},
		{"whole system --clean-db", true, false, nil},
		{"whole system --clean-all", false, true, nil},
		{"service scoped, no flags", false, false, []string{"cb-spider"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setCleanFlags(t, tc.db, tc.all)
			got, err := imageRemovalTargets(tc.services)
			if err != nil {
				t.Fatalf("imageRemovalTargets: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("imageRemovalTargets = %v, want nothing removed on this path", got)
			}
		})
	}
}

// An image left as ${VAR} cannot be removed, and `docker rmi` failures are
// deliberately non-fatal — so an unresolved reference would slip through as a
// warning and leave the image in place, which is the exact outcome removing
// images is meant to prevent. It has to stop the command instead.
func TestImageRemovalTargetsRejectsUnresolvedVariable(t *testing.T) {
	withComposeFile(t, `
services:
  cm-ant:
    image: cloudbaristaorg/cm-ant:${ANT_TAG}
`)
	setCleanFlags(t, true, false)

	_, err := imageRemovalTargets([]string{"cm-ant"})
	if err == nil {
		t.Fatal("an unresolved ${VAR} in the image should be reported, not passed to docker rmi")
	}
	if !strings.Contains(err.Error(), "cm-ant") || !strings.Contains(err.Error(), "${ANT_TAG}") {
		t.Errorf("the error should name the service and the unresolved image, got: %v", err)
	}
}
