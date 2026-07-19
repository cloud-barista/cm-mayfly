package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Arguments carrying shell metacharacters must reach the program literally
// instead of being interpreted. The infra commands used to build a string and
// hand it to `/bin/sh -c`, which made `logs --tail '1; echo INJECTED'` and
// `remove -s 'openbao;id'` execute the injected part.
func TestRunCommandOutputDoesNotInterpretMetacharacters(t *testing.T) {
	payloads := []string{
		"1; echo INJECTED",
		"1h'; echo INJECTED; :'",
		"$(echo INJECTED)",
		"`echo INJECTED`",
		"a | echo INJECTED",
		"a && echo INJECTED",
		"openbao;id",
	}

	for _, payload := range payloads {
		out, err := RunCommandOutput("printf", []string{"%s", payload}, nil)
		if err != nil {
			t.Fatalf("RunCommandOutput(%q): %v", payload, err)
		}
		got := string(out)
		if got != payload {
			t.Errorf("argument %q was altered on the way to the program: got %q", payload, got)
		}
		// `printf %s` echoes its argument verbatim, so the literal text is
		// expected. What must never appear is INJECTED on its own — the mark of
		// a shell having run the payload as a second command.
		for _, line := range strings.Split(got, "\n") {
			if strings.TrimSpace(line) == "INJECTED" {
				t.Errorf("payload %q was executed by a shell; output: %q", payload, got)
			}
		}
	}
}

// A failing command must surface as an error. SysCall printed the failure and
// returned nothing, so callers carried on and reported success.
func TestRunCommandReturnsExitStatus(t *testing.T) {
	if err := RunCommand("false", nil, nil); err == nil {
		t.Error("RunCommand must return an error when the program exits non-zero")
	}
	if err := RunCommand("true", nil, nil); err != nil {
		t.Errorf("RunCommand must return nil on success, got %v", err)
	}
	if err := RunCommand("cm-mayfly-no-such-program", nil, nil); err == nil {
		t.Error("RunCommand must return an error when the program does not exist")
	}
}

// extraEnv reaches the child process, so COMPOSE_PROJECT_NAME no longer has to
// be prefixed onto a shell command string.
func TestRunCommandPassesExtraEnv(t *testing.T) {
	out, err := RunCommandOutput("sh", []string{"-c", "printf %s \"$COMPOSE_PROJECT_NAME\""},
		[]string{"COMPOSE_PROJECT_NAME=cm-test-project"})
	if err != nil {
		t.Fatalf("RunCommandOutput: %v", err)
	}
	if string(out) != "cm-test-project" {
		t.Errorf("extraEnv did not reach the child process: got %q", out)
	}
}

// The child must inherit the parent environment alongside extraEnv, otherwise
// docker would lose PATH, HOME, DOCKER_HOST and friends.
func TestRunCommandInheritsParentEnv(t *testing.T) {
	t.Setenv("CM_MAYFLY_TEST_INHERITED", "yes")

	out, err := RunCommandOutput("sh", []string{"-c", "printf %s \"$CM_MAYFLY_TEST_INHERITED\""},
		[]string{"COMPOSE_PROJECT_NAME=cm-test-project"})
	if err != nil {
		t.Fatalf("RunCommandOutput: %v", err)
	}
	if string(out) != "yes" {
		t.Errorf("parent environment was not inherited: got %q", out)
	}
}

// A path holding shell metacharacters must be removed as a literal path, not
// re-parsed. This is the `sudo rm -rf .../openbao;id` case.
func TestRunCommandTreatsPathsLiterally(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "keep-me")
	if err := os.WriteFile(victim, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	// "odd;name" is a single path component, not a command separator.
	odd := filepath.Join(dir, "odd;name")
	if err := os.WriteFile(odd, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := RunCommand("rm", []string{"-rf", odd}, nil); err != nil {
		t.Fatalf("RunCommand rm: %v", err)
	}
	if _, err := os.Stat(odd); !os.IsNotExist(err) {
		t.Errorf("the literal path %q should have been removed", odd)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("an unrelated file was affected: %v", err)
	}
}
