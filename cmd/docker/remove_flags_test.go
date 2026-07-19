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

// The service-scoped --clean-db paragraph must not promise the things a
// service-scoped removal does not do. buildComposeCommands issues stop + rm for
// the named services, so images and shared networks stay put.
func TestServiceScopedCleanDBMessageMatchesBehaviour(t *testing.T) {
	// What the command actually runs for a service-scoped --clean-db.
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	t.Cleanup(func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll })
	cleanDBFlag, cleanAllFlag = true, false

	cmds := buildComposeCommands([]string{"cb-spider"})
	var flat []string
	for _, c := range cmds {
		flat = append(flat, strings.Join(c, " "))
	}
	joined := strings.Join(flat, " | ")
	if strings.Contains(joined, "--rmi") {
		t.Fatalf("service-scoped removal unexpectedly removes images: %s", joined)
	}

	// So the message must not claim it removes images or networks.
	lower := strings.ToLower(msgRemoveCleanDBService)
	for _, promise := range []string{"removes containers, images", "networks, and db host data"} {
		if strings.Contains(lower, promise) {
			t.Errorf("service-scoped message still promises %q, which the command does not do", promise)
		}
	}
	// And it should say so explicitly, so the user knows a follow-up is needed.
	if !strings.Contains(lower, "images and shared networks are left in place") {
		t.Errorf("service-scoped message should state that images and networks are kept, got:\n%s", msgRemoveCleanDBService)
	}
}
