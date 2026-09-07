//go:build unix

package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// Mounted reports whether something is mounted at path.
//
// The mount table is a different thing on every unix -- /proc/self/mounts on
// Linux, a program to run on macOS -- but the reason a mount is visible is the
// same everywhere: the directory sits on a different device from the one
// holding it. Asking the device is the portable question, so the platform
// split is this file rather than a branch at the call site.
func Mounted(path string) bool {
	dev, ok := device(path)
	parent, ok2 := device(filepath.Dir(path))
	return ok && ok2 && dev != parent
}

func device(path string) (uint64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}
