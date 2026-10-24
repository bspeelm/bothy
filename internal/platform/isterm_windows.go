//go:build windows

package platform

import (
	"os"
	"syscall"
)

// IsTerminal asks the console API, because the Unix test answers wrongly here
// rather than failing to compile. Measured in a WezTerm tab: a console handle
// is a character device, and os.Stat("NUL") returns a stat with no file
// identity -- no path, no volume, no index -- as does the handle's own. So
// SameFile compares zero with zero, says the console is the null device, and
// every stream on Windows reads as "not a terminal". GetConsoleMode succeeds
// only on a real console, which is the question being asked.
func IsTerminal(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}
