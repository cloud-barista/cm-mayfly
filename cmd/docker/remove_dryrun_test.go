package docker

import (
	"path/filepath"
	"strings"
	"testing"
)

// setCleanFlags sets the two clean flags for one test and restores them after.
func setCleanFlags(t *testing.T, db, all bool) {
	t.Helper()
	prevDB, prevAll := cleanDBFlag, cleanAllFlag
	t.Cleanup(func() { cleanDBFlag, cleanAllFlag = prevDB, prevAll })
	cleanDBFlag, cleanAllFlag = db, all
}

// --dry-run must announce the .env edit that --clean-all performs.
//
// The real run clears VAULT_TOKEN as its final step, but the dry-run listed only
// the compose commands and the rm targets. A user reading it to decide whether
// the command was safe saw nothing about .env — the one file in the blast radius
// they may have hand-edited.
func TestDryRunPlanAnnouncesEnvTokenClear(t *testing.T) {
	setCleanFlags(t, false, true)

	plan := dryRunPlan([][]string{{"down", "--volumes", "--rmi", "all", "--remove-orphans"}}, nil, []string{"conf/docker/data/openbao"})

	absEnv, _ := filepath.Abs(envFilePath())
	if !strings.Contains(plan, "clear VAULT_TOKEN in "+absEnv) {
		t.Errorf("--clean-all dry-run does not announce the VAULT_TOKEN clear.\nplan:\n%s", plan)
	}
	// The rest of the plan is still there.
	if !strings.Contains(plan, "down") || !strings.Contains(plan, "sudo rm -rf") {
		t.Errorf("dry-run plan lost its compose/rm lines.\nplan:\n%s", plan)
	}
	if !strings.Contains(plan, "No changes were made.") {
		t.Errorf("dry-run plan should end by stating nothing happened.\nplan:\n%s", plan)
	}
}

// Without --clean-all there is no .env edit, so the line must not appear —
// otherwise the dry-run overstates the damage instead of understating it.
func TestDryRunPlanOmitsEnvTokenClearWhenNotCleanAll(t *testing.T) {
	for _, tc := range []struct {
		name   string
		db     bool
		all    bool
		expect bool
	}{
		{"default", false, false, false},
		{"clean-db", true, false, false},
		{"clean-all", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setCleanFlags(t, tc.db, tc.all)
			plan := dryRunPlan([][]string{{"down", "--remove-orphans"}}, nil, nil)
			got := strings.Contains(plan, "clear VAULT_TOKEN")
			if got != tc.expect {
				t.Errorf("VAULT_TOKEN line present = %v, want %v.\nplan:\n%s", got, tc.expect, plan)
			}
		})
	}
}

// Every host target the command would wipe has to be listed, as an absolute
// path — a relative one is ambiguous about what is actually about to be deleted.
func TestDryRunPlanListsEveryHostTargetAbsolute(t *testing.T) {
	setCleanFlags(t, true, false)

	targets := []string{"conf/docker/data/cb-spider", "conf/docker/data/cb-tumblebug"}
	plan := dryRunPlan(nil, nil, targets)

	for _, tgt := range targets {
		abs, _ := filepath.Abs(tgt)
		if !strings.Contains(plan, "sudo rm -rf "+abs) {
			t.Errorf("target %q missing from the plan (expected absolute %q).\nplan:\n%s", tgt, abs, plan)
		}
	}
}
