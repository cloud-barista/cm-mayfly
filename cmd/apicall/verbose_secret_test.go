package apicall

import (
	"os"
	"strings"
	"testing"

	"github.com/cm-mayfly/cm-mayfly/common"
)

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

// `mayfly api -v` resolves credentials out of api.yaml, including ${VAR}
// references, so it is exactly the command someone runs when a credential is not
// working — and exactly the output they then paste into a ticket.
func TestSetBasicAuthVerboseDoesNotPrintPassword(t *testing.T) {
	prevInfo, prevVerbose := serviceInfo, isVerbose
	t.Cleanup(func() { serviceInfo, isVerbose = prevInfo, prevVerbose })

	serviceInfo.Auth.Username = "default"
	serviceInfo.Auth.Password = testPassword
	isVerbose = true

	out := captureStdout(t, SetBasicAuth)

	if strings.Contains(out, testPassword) {
		t.Errorf("-v printed the password verbatim:\n%s", out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("-v should still show the username, got:\n%s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("-v should show a masked password, got:\n%s", out)
	}
}

// The bearer token path prints through the same helper. Asserting on it here
// keeps the property attached to this package: whatever else changes about how
// the token is obtained, it must not reach stdout intact.
func TestBearerTokenIsMaskedBeforePrinting(t *testing.T) {
	got := common.MaskSecret(testToken)
	if strings.Contains(got, "signature") || got == testToken {
		t.Errorf("token was not masked: %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Errorf("expected a masked value, got %q", got)
	}
}
