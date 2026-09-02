//go:build unix

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

// TerminalSize asks the terminal how big it is, in columns and rows.
//
// ok is false when none of these is a terminal -- a pipe, a CI runner, a
// desktop launcher -- which is a fact about the situation rather than a
// failure, and has no fix.
//
// The ioctl is the only way to ask. $COLUMNS and $LINES are what a shell
// happens to have exported and may be stale; this is the terminal's own
// answer, which is the point of calling it at the moment the size is wanted.
func TerminalSize() (cols, rows int, ok bool) {
	var ws struct{ Row, Col, X, Y uint16 }
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(),
			uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
		if errno == 0 && ws.Col > 0 && ws.Row > 0 {
			return int(ws.Col), int(ws.Row), true
		}
	}
	return 0, 0, false
}
