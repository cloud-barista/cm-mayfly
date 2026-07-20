package docker

import (
	"os"
	"path/filepath"
	"testing"
)

// .env holds DB credentials and VAULT_TOKEN. Every rewrite must leave it
// readable by its owner only — including a rewrite of a file that was created
// world-readable, which is what copying .env.example under the usual umask
// produces. os.WriteFile does not do this: its mode argument only applies when
// it creates the file, so the old code left a 0644 .env at 0644.
func TestWriteEnvFileNarrowsMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte("A=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeEnvFile(p, []byte("A=2\n")); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0600 {
		t.Errorf("mode = %04o, want 0600", got)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "A=2\n" {
		t.Errorf("content = %q, want %q", got, "A=2\n")
	}
}

// The same guarantee has to hold for a file that does not exist yet.
func TestWriteEnvFileCreatesOwnerOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := writeEnvFile(p, []byte("A=1\n")); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0600 {
		t.Errorf("mode = %04o, want 0600", got)
	}
}

// A write that fails must leave the previous .env intact. The old code
// truncated first, so an interrupted write destroyed the credentials with
// nothing to fall back on; writing to a temporary file and renaming means a
// failure before the rename cannot touch the original.
//
// The failure is provoked by making the directory unwritable, so the temporary
// file cannot be created.
func TestWriteEnvFilePreservesOriginalOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permissions")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	const original = "TUMBLEBUG_DB_PASSWORD=keepme\n"
	if err := os.WriteFile(p, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	if err := writeEnvFile(p, []byte("TUMBLEBUG_DB_PASSWORD=clobbered\n")); err == nil {
		t.Fatal("writeEnvFile succeeded on an unwritable directory, want an error")
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("original .env was modified by a failed write.\n got: %q\nwant: %q", got, original)
	}
}

// No temporary file may be left behind next to .env — docker compose reads
// every file in that directory's vicinity and a stray .env.tmp-* holding
// credentials is exactly what the atomic write is meant to avoid.
func TestWriteEnvFileLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := writeEnvFile(p, []byte("A=1\n")); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".env" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only .env", names)
	}
}

// setEnvKey and clearEnvKey both go through writeEnvFile, so the mode
// guarantee has to survive the path an actual command takes.
func TestEnvKeyWritersNarrowMode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(path string) error
	}{
		{"setEnvKey", func(p string) error { return setEnvKey(p, "AIRFLOW_JWT_SECRET", "s3cret") }},
		{"clearEnvKey", func(p string) error { _, err := clearEnvKey(p, "VAULT_TOKEN"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(p, []byte("VAULT_TOKEN=abc\nAIRFLOW_JWT_SECRET=\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := tc.write(p); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != 0600 {
				t.Errorf("mode after %s = %04o, want 0600", tc.name, got)
			}
		})
	}
}
