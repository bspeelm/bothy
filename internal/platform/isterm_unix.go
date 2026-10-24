//go:build unix

package platform

import "os"

// IsTerminal reports whether a stream is an interactive terminal. /dev/null is
// a character device too, so it is excluded explicitly; that covers redirects,
// cron and systemd units without an ioctl (no x/term -- PLAN.md §13).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}
