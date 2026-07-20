package rest

import (
	"os"
	"strings"
	"testing"
)

// captureStdout collects everything fn prints.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	out := <-done
	r.Close()
	return out
}

const (
	testPassword = "sup3r-secret-password"
	testToken    = "eyJhbGciOiJIUzI1NiJ9.payload.signature"
)

// -v must not write the password into the terminal.
//
// `mayfly rest -v` is run to see how a call was assembled, and its output ends
// up in scrollback, CI job logs and redirected files — all of which outlive the
// debugging session by a long way. The username stays visible because it is not
// a secret and it is half of what the user is trying to confirm.
func TestSetBasicAuthVerboseDoesNotPrintPassword(t *testing.T) {
	prevUser, prevPass, prevVerbose := username, password, isVerbose
	t.Cleanup(func() { username, password, isVerbose = prevUser, prevPass, prevVerbose })

	username, password, isVerbose = "admin", testPassword, true

	out := captureStdout(t, SetBasicAuth)

	if strings.Contains(out, testPassword) {
		t.Errorf("-v printed the password verbatim:\n%s", out)
	}
	if !strings.Contains(out, "admin") {
		t.Errorf("-v should still show the username, got:\n%s", out)
	}
	// A masked value is still shown, so the user can tell a password was picked
	// up at all — hiding it entirely would make -v useless for this.
	if !strings.Contains(out, "password :") {
		t.Errorf("-v should still report the password field, got:\n%s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("-v should show a masked value, got:\n%s", out)
	}
}

// The same applies to the bearer token, which is if anything more sensitive —
// it is a ready-to-use credential rather than one factor of a login.
func TestSetAuthTokenVerboseDoesNotPrintToken(t *testing.T) {
	prevToken, prevScheme, prevVerbose := authToken, authScheme, isVerbose
	t.Cleanup(func() { authToken, authScheme, isVerbose = prevToken, prevScheme, prevVerbose })

	authToken, authScheme, isVerbose = testToken, "Bearer", true

	out := captureStdout(t, SetAuthToken)

	if strings.Contains(out, testToken) {
		t.Errorf("-v printed the auth token verbatim:\n%s", out)
	}
	// The signature is the part that must never appear, even if a prefix does.
	if strings.Contains(out, "signature") {
		t.Errorf("-v leaked the token signature:\n%s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("-v should show a masked token, got:\n%s", out)
	}
	// The scheme is not a secret and is worth seeing.
	if !strings.Contains(out, "Bearer") {
		t.Errorf("-v should still show the auth scheme, got:\n%s", out)
	}
}

// Without -v nothing is printed at all.
func TestQuietModePrintsNothing(t *testing.T) {
	prevUser, prevPass, prevToken, prevVerbose := username, password, authToken, isVerbose
	t.Cleanup(func() {
		username, password, authToken, isVerbose = prevUser, prevPass, prevToken, prevVerbose
	})

	username, password, authToken, isVerbose = "admin", testPassword, testToken, false

	out := captureStdout(t, func() {
		SetBasicAuth()
		SetAuthToken()
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("without -v nothing should be printed, got:\n%s", out)
	}
}
