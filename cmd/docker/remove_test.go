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
	found, err := clearEnvKey(p, "VAULT_TOKEN")
	if err != nil {
		t.Fatalf("clearEnvKey: %v", err)
	}
	if !found {
		t.Error("clearEnvKey reported the key as absent although it was present")
	}
	got, _ := os.ReadFile(p)
	want := "# header\nVAULT_TOKEN=\nTUMBLEBUG_DB_PASSWORD=tumblebug\nBEETLE_API_PASSWORD=default\n"
	if string(got) != want {
		t.Fatalf("clearEnvKey result mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// Absent key → file unchanged, and found is false so the caller can say so
// instead of announcing a clear that never happened.
func TestClearEnvKeyAbsent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	in := "ANT_DB_PASSWORD=cm-ant-secret\n"
	if err := os.WriteFile(p, []byte(in), 0644); err != nil {
		t.Fatal(err)
	}
	found, err := clearEnvKey(p, "VAULT_TOKEN")
	if err != nil {
		t.Fatalf("clearEnvKey: %v", err)
	}
	if found {
		t.Error("clearEnvKey reported the key as found although the file has no VAULT_TOKEN line")
	}
	got, _ := os.ReadFile(p)
	if string(got) != in {
		t.Fatalf("absent key should leave file unchanged. got: %q", got)
	}
}

// A missing .env is an error, not a quiet "nothing to clear" — the two are
// different situations and the caller prints different things for them.
func TestClearEnvKeyMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "does-not-exist.env")
	found, err := clearEnvKey(p, "VAULT_TOKEN")
	if err == nil {
		t.Fatal("clearEnvKey on a missing file should return an error")
	}
	if found {
		t.Error("found should be false when the file could not be read")
	}
}
