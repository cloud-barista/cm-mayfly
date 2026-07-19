package common

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// captureStdout collects everything fn writes to stdout. These helpers stream a
// child's output rather than returning it, so the only way to observe what they
// carried is to read the stream they print to.
//
// The reader runs in its own goroutine and drains continuously. That matters:
// the child now inherits this pipe directly, so a reader that waited until fn
// returned would fill the pipe and deadlock the very hang these tests exist to
// rule out.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	out, _ := captureStdoutTimed(t, fn, "")
	return out
}

// captureStdoutTimed is captureStdout with one extra observation: how long after
// fn started the given marker first appeared in the stream. It answers whether
// output reaches the terminal while the child is still running or only once it
// exits. An empty marker skips the timing.
func captureStdoutTimed(t *testing.T, fn func(), marker string) (string, time.Duration) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	type result struct {
		out       string
		firstSeen time.Duration
	}
	done := make(chan result, 1)
	start := time.Now()

	go func() {
		var sb strings.Builder
		var firstSeen time.Duration
		buf := make([]byte, 32*1024)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
				if marker != "" && firstSeen == 0 && strings.Contains(sb.String(), marker) {
					firstSeen = time.Since(start)
				}
			}
			if readErr != nil {
				break
			}
		}
		done <- result{sb.String(), firstSeen}
	}()

	fn()
	w.Close()
	os.Stdout = orig
	res := <-done
	r.Close()
	return res.out, res.firstSeen
}

// oneLongLine builds an awk program that emits a single line of n bytes followed
// by a short marker line, so both the line itself and whatever came after it can
// be checked.
//
// The line is generated inside the child rather than passed as an argument: an
// argv entry this large exceeds the per-argument limit and the exec would fail
// before any of this is exercised.
func oneLongLine(n int) (awkProg string, shellCmd string) {
	awkProg = fmt.Sprintf(
		`BEGIN{s="x";while(length(s)<%d){s=s s}print substr(s,1,%d);print "END"}`, n, n)
	return awkProg, "awk '" + awkProg + "'"
}

// No single line, at any size, may stall the stream.
//
// The old implementation read the child through a bufio.Scanner, which has to
// hold a whole line before emitting it and therefore caps line length. Past the
// cap it stopped and reported the truncation as an ordinary end of input; with
// nothing left draining the pipe, the child blocked on write and cmd.Wait()
// blocked on the child. Against that implementation this test does not fail, it
// hangs until the go test timeout — exactly what a user saw mid-`logs -f`.
//
// 10MB is far past any limit the scanner version could have been configured
// with, so passing it shows the ceiling is gone rather than merely raised.
func TestStreamingHelpersCarryLongLines(t *testing.T) {
	sizes := []struct {
		name string
		n    int
	}{
		{"1MB", 1024 * 1024},
		{"1MB+1", 1024*1024 + 1},
		{"10MB", 10 * 1024 * 1024},
	}

	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			awkProg, shellCmd := oneLongLine(size.n)
			args := []string{awkProg}

			t.Run("RunCommand", func(t *testing.T) {
				out := captureStdout(t, func() {
					if err := RunCommand("awk", args, nil); err != nil {
						t.Errorf("RunCommand: %v", err)
					}
				})
				assertCarriedLongLine(t, out, size.n)
			})

			t.Run("SysCallWithError", func(t *testing.T) {
				out := captureStdout(t, func() {
					if err := SysCallWithError(shellCmd); err != nil {
						t.Errorf("SysCallWithError: %v", err)
					}
				})
				assertCarriedLongLine(t, out, size.n)
			})

			t.Run("SysCall", func(t *testing.T) {
				out := captureStdout(t, func() { SysCall(shellCmd) })
				assertCarriedLongLine(t, out, size.n)
			})
		})
	}
}

// assertCarriedLongLine checks both halves of the failure: the long line itself
// has to arrive whole, and the output that followed it must not have been cut
// off — a stalled stream loses everything after the oversized line, not just
// that line.
func assertCarriedLongLine(t *testing.T, out string, want int) {
	t.Helper()

	var longest int
	for _, line := range strings.Split(out, "\n") {
		if len(line) > longest {
			longest = len(line)
		}
	}
	if longest < want {
		t.Errorf("longest line carried was %d bytes, want at least %d — the line was truncated", longest, want)
	}
	if !strings.Contains(out, "END") {
		t.Error("output after the long line was lost; the stream stopped early")
	}
}

// A long run of ordinary lines has to arrive complete. Line length is one way to
// stall a stream; sheer volume through a pipe is another, and `logs --tail all`
// produces exactly this shape.
func TestStreamingHelpersCarryManyLines(t *testing.T) {
	const lines = 50000
	awkProg := fmt.Sprintf(`BEGIN{for(i=1;i<=%d;i++)print i}`, lines)

	out := captureStdout(t, func() {
		if err := RunCommand("awk", []string{awkProg}, nil); err != nil {
			t.Errorf("RunCommand: %v", err)
		}
	})

	got := strings.Count(out, "\n")
	if got != lines {
		t.Errorf("carried %d lines, want %d", got, lines)
	}
	if !strings.Contains(out, fmt.Sprintf("\n%d\n", lines)) {
		t.Errorf("the last line (%d) never arrived", lines)
	}
}

// Output must reach the terminal as the child produces it, not in one burst when
// the child exits. A user watching `logs -f` is reading a live stream; anything
// that batches it makes the command look frozen even when nothing is wrong.
func TestStreamingHelpersAreNotBuffered(t *testing.T) {
	const delay = 1500 * time.Millisecond

	out, firstSeen := captureStdoutTimed(t, func() {
		if err := RunCommand("sh", []string{"-c", "echo EARLY; sleep 1.5; echo LATE"}, nil); err != nil {
			t.Errorf("RunCommand: %v", err)
		}
	}, "EARLY")

	if !strings.Contains(out, "EARLY") || !strings.Contains(out, "LATE") {
		t.Fatalf("both markers should have arrived, got %q", out)
	}
	if firstSeen == 0 {
		t.Fatal("the first marker was never observed arriving")
	}
	// The child sleeps 1.5s after the first line. Seeing that line well before
	// the sleep ends proves it was not held until exit.
	if firstSeen > delay/2 {
		t.Errorf("first line took %v to appear; the child slept %v afterwards, so the output was buffered until exit", firstSeen, delay)
	}
}

// A failing command still has to report its exit status. Streaming straight to
// the terminal must not cost the caller the one thing it uses to branch on.
func TestStreamingHelpersReportExitCode(t *testing.T) {
	const wantCode = 7

	t.Run("RunCommandInDir", func(t *testing.T) {
		var err error
		captureStdout(t, func() {
			err = RunCommandInDir("", "sh", []string{"-c", "echo before-exit; exit 7"}, nil)
		})
		assertExitCode(t, err, wantCode)
	})

	t.Run("SysCallWithError", func(t *testing.T) {
		var err error
		captureStdout(t, func() {
			err = SysCallWithError("echo before-exit; exit 7")
		})
		assertExitCode(t, err, wantCode)
	})

	t.Run("success is nil", func(t *testing.T) {
		var err error
		captureStdout(t, func() {
			err = RunCommandInDir("", "sh", []string{"-c", "echo ok"}, nil)
		})
		if err != nil {
			t.Errorf("a command that succeeded returned %v", err)
		}
	})
}

func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("a command that exited %d returned no error", want)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error was %T (%v), want *exec.ExitError carrying the exit status", err, err)
	}
	if got := exitErr.ExitCode(); got != want {
		t.Errorf("exit code %d, want %d", got, want)
	}
}

// stderr has to keep arriving on stdout, interleaved in the order the child
// wrote it. These helpers have always merged the two streams and callers read
// them as one; splitting them would reorder a live log and drop errors for
// anyone piping the command.
func TestStreamingHelpersMergeStderrIntoStdout(t *testing.T) {
	t.Run("RunCommand", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := RunCommand("sh", []string{"-c", "echo OUT-1; echo ERR-1 >&2; echo OUT-2"}, nil); err != nil {
				t.Errorf("RunCommand: %v", err)
			}
		})
		assertMergedInOrder(t, out)
	})

	t.Run("SysCall", func(t *testing.T) {
		out := captureStdout(t, func() {
			SysCall("echo OUT-1; echo ERR-1 >&2; echo OUT-2")
		})
		assertMergedInOrder(t, out)
	})
}

func assertMergedInOrder(t *testing.T, out string) {
	t.Helper()
	for _, want := range []string{"OUT-1", "ERR-1", "OUT-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from stdout; got %q", want, out)
		}
	}
	first, mid, last := strings.Index(out, "OUT-1"), strings.Index(out, "ERR-1"), strings.Index(out, "OUT-2")
	if first > mid || mid > last {
		t.Errorf("stdout and stderr arrived out of order: %q", out)
	}
}
