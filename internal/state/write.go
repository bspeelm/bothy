package state

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile writes via a temporary file and renames, so an interrupted write
// cannot leave a tool -- or bothy's next run -- reading a truncated file.
func WriteFile(dest string, content []byte, mode fs.FileMode) error {
	// Unique temp name, so concurrent writers cannot rename each other's
	// half-written file.
	f, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".bothy-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename below succeeds

	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// CreateTemp makes the file 0600; the caller's mode is the one that
	// matters, and it must be set before the rename so nothing observes the
	// file at the wrong permissions.
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
