package docker

import (
	"fmt"
	"os"
	"path/filepath"
)

// envFileMode is the mode .env is kept at. It holds DB credentials and
// VAULT_TOKEN, so nobody but the owner has any business reading it.
const envFileMode os.FileMode = 0600

// writeEnvFile replaces the contents of the .env at path atomically, and leaves
// the file owner-readable only.
//
// Two things the previous os.WriteFile(path, data, 0600) did not do:
//
//   - Mode. os.WriteFile only applies the mode when it *creates* the file. An
//     .env copied from .env.example under a typical 0022 umask is 0644, and
//     every rewrite left it that way — the comment claimed owner-only while the
//     secrets stayed world-readable. The mode is therefore set explicitly.
//   - Atomicity. os.WriteFile truncates first and then writes. An interrupted or
//     out-of-space write leaves a half-written .env and no way back: the DB
//     credentials and VAULT_TOKEN that were in it are simply gone. Writing to a
//     temporary file in the same directory and renaming over the target means
//     the file a reader sees is always one of the two complete versions.
//
// The temporary file lives in the target's own directory so the rename stays
// within one filesystem (os.Rename cannot cross devices) and never widens the
// exposure to a shared directory such as /tmp.
func writeEnvFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".env.tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create a temporary file next to %s: %w", path, err)
	}
	tmpName := tmp.Name()
	// Clean up the temporary file on every path that does not rename it away.
	defer func() {
		if _, err := os.Stat(tmpName); err == nil {
			_ = os.Remove(tmpName)
		}
	}()

	// CreateTemp already makes the file 0600, but say so explicitly rather than
	// depending on that.
	if err := tmp.Chmod(envFileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to set permissions on the temporary file for %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	// Flush to disk before the rename, so a crash right after it cannot leave
	// the renamed file with unwritten contents.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to flush %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close the temporary file for %s: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	// The rename carries the temporary file's 0600 across, so the mode is
	// already correct here. Re-apply it anyway: it costs nothing and keeps the
	// guarantee true even if the file is replaced some other way later.
	if err := os.Chmod(path, envFileMode); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", path, err)
	}
	return nil
}
