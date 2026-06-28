package docker

import (
	"os"
	"path/filepath"
	"testing"
)

// clearEnvKey must blank only the target key and preserve every other line
// (other user settings, comments, ordering) — verifying the --clean-all
// VAULT_TOKEN cleanup does not disturb DB passwords etc.
func TestClearEnvKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	in := "# header\nVAULT_TOKEN=s.abc123\nTUMBLEBUG_DB_PASSWORD=tumblebug\nBEETLE_API_PASSWORD=default\n"
	if err := os.WriteFile(p, []byte(in), 0644); err != nil {
		t.Fatal(err)
	}
	if err := clearEnvKey(p, "VAULT_TOKEN"); err != nil {
		t.Fatalf("clearEnvKey: %v", err)
	}
	got, _ := os.ReadFile(p)
	want := "# header\nVAULT_TOKEN=\nTUMBLEBUG_DB_PASSWORD=tumblebug\nBEETLE_API_PASSWORD=default\n"
	if string(got) != want {
		t.Fatalf("clearEnvKey result mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// Absent key → file unchanged.
func TestClearEnvKeyAbsent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	in := "ANT_DB_PASSWORD=cm-ant-secret\n"
	os.WriteFile(p, []byte(in), 0644)
	if err := clearEnvKey(p, "VAULT_TOKEN"); err != nil {
		t.Fatalf("clearEnvKey: %v", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != in {
		t.Fatalf("absent key should leave file unchanged. got: %q", got)
	}
}
