package setup

import (
	"os"
	"strings"
	"testing"
)

func captureStdoutSecret(t *testing.T, fn func()) string {
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

// `mayfly setup credential -v` is the command someone runs while registering CSP
// credentials, so its output is the last place a cb-tumblebug API password
// should be written in the clear.
func TestSetupSetBasicAuthVerboseDoesNotPrintPassword(t *testing.T) {
	const secret = "tumblebug-api-password"

	prevInfo, prevVerbose := serviceInfo, isVerbose
	t.Cleanup(func() { serviceInfo, isVerbose = prevInfo, prevVerbose })

	serviceInfo.Auth.Username = "default"
	serviceInfo.Auth.Password = secret
	isVerbose = true

	out := captureStdoutSecret(t, SetBasicAuth)

	if strings.Contains(out, secret) {
		t.Errorf("-v printed the password verbatim:\n%s", out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("-v should still show the username, got:\n%s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("-v should show a masked password, got:\n%s", out)
	}
}
