package common

import (
	"fmt"
	"os"
	"os/exec"
)

//var CommandStr string
//var TargetStr string

const (

	// NotDefined is a variable that holds the string "Not_Defined"
	NotDefined string = "Not_Defined"
)

// streamTo wires a child process to this process's own stdout so its output
// reaches the terminal without passing through a buffer we own.
//
// These helpers never parse what the child writes — they only forward it — so
// there is nothing to gain from reading it line by line, and a great deal to
// lose. A line scanner has to hold a whole line in memory before it can emit
// it, which means it must declare a maximum line length; past that limit it
// stops and reports the truncation as an ordinary end of input. Nothing then
// drains the pipe, so the child blocks writing while cmd.Wait() blocks on the
// child, and a `logs -f` session freezes mid-stream. Raising the limit only
// moves the line at which it happens.
//
// Handing os.Stdout to the child removes the pipe entirely: the file descriptor
// is inherited, the child writes to the terminal directly, and no length applies
// because no buffer of ours is involved. Output also stays byte-exact and
// unbuffered, which keeps colour codes, progress redraws and interactive prompts
// working as they do when the command is run by hand.
//
// stderr is merged into stdout rather than sent to fd 2. These helpers have
// always combined the two streams, and callers such as `infra logs` rely on the
// merge: docker writes container output to both, and splitting them now would
// reorder a running log and hide errors from anyone piping the command. Sharing
// one descriptor also keeps the two streams strictly ordered, which a split
// cannot guarantee.
func streamTo(cmd *exec.Cmd) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
}

// These three helpers deliberately run their argument through /bin/sh: the
// callers that remain need shell features (pipes, redirection, $VAR expansion
// inside `docker exec`). Everything that did not need a shell was moved to
// RunCommand*/exec.Command with an argument vector. Callers must therefore
// pass a fixed command string; do not build one out of flags or arguments.
//
// SysCall executes the given shell command, streaming its output.
func SysCall(cmdStr string) {
	cmd := exec.Command("/bin/sh", "-c", cmdStr) // #nosec G204 -- shell is intentional here; every call site passes a fixed command string
	streamTo(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println(err)
		//os.Exit(1)
	}

}

// SysCallWithError executes user-passed command via system call and returns error.
func SysCallWithError(cmdStr string) error {
	cmd := exec.Command("/bin/sh", "-c", cmdStr) // #nosec G204 -- shell is intentional here; every call site passes a fixed command string
	streamTo(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

// RunCommand executes a program directly, without going through a shell, and
// streams its combined output. Every element of args is handed to the program as
// a single argument, so a value that contains shell metacharacters (`;`, `|`,
// `$(…)`, backticks, quotes) is passed through literally instead of being
// interpreted. extraEnv entries ("KEY=VALUE") are appended to the current
// environment.
//
// The error returned carries the process exit status, so a caller can tell a
// failed command from a successful one — unlike SysCall, which discards it.
func RunCommand(name string, args []string, extraEnv []string) error {
	return RunCommandInDir("", name, args, extraEnv)
}

// RunCommandInDir is RunCommand with a working directory. An empty dir runs the
// program in the caller's own working directory, exactly as RunCommand does.
//
// Setting the directory on the process is what lets a caller drop `cd <dir> &&
// …` from a shell string: the path stops being a fragment of a command line
// that a shell would re-parse, so a directory containing `$(…)`, a quote or a
// space is handed to the kernel as one literal argument.
func RunCommandInDir(dir, name string, args []string, extraEnv []string) error {
	cmd := exec.Command(name, args...) // #nosec G204 -- arguments are passed as a vector, never re-parsed by a shell
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	streamTo(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

// RunCommandOutput executes a program directly, without a shell, and returns its
// standard output. It is the capture-only counterpart of RunCommand.
func RunCommandOutput(name string, args []string, extraEnv []string) ([]byte, error) {
	cmd := exec.Command(name, args...) // #nosec G204 -- arguments are passed as a vector, never re-parsed by a shell
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	return cmd.Output()
}

// SysCallWithOutput executes user-passed command via system call and returns output.
func SysCallWithOutput(cmdStr string) string {
	cmd := exec.Command("/bin/sh", "-c", cmdStr) // #nosec G204 -- shell is intentional here; the openbao call sites pass fixed docker/curl command strings

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return string(output)
}
