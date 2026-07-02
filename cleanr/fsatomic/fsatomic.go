// Package fsatomic provides crash-safe file writes for cleanr's persisted
// artifacts (configs, profiles, snapshots, trend history, attestations).
// Data is written to a temporary file in the destination directory, synced,
// and renamed over the target so readers never observe a torn file and an
// interrupted write never destroys the previous content.
package fsatomic

import (
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data. The temporary file is created
// in path's directory so the final rename stays on one filesystem. The file is
// fsynced before the rename, and the directory afterwards (best-effort), so a
// power loss cannot persist the rename without the data.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	syncDir(dir)
	return nil
}

// syncDir flushes the directory entry for a completed rename. Errors are
// ignored: not every platform or filesystem supports fsync on directories, and
// the write itself has already succeeded.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
