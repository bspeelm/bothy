//go:build windows

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

// kernel32 answers what the stdlib does not export: syscall has
// GetConsoleMode and no console geometry at all. Through a lazy DLL rather
// than golang.org/x/sys, which PLAN.md §13 rules out -- kernel32 is already
// loaded in every process, so the preloading caveat on LazyDLL does not bite.
var getConsoleScreenBufferInfo = syscall.NewLazyDLL("kernel32.dll").
	NewProc("GetConsoleScreenBufferInfo")

// consoleInfo is CONSOLE_SCREEN_BUFFER_INFO. Only the window rectangle is
// read: size is the scroll buffer, which is thousands of rows tall and is not
// what anything here means by the height of a terminal.
type consoleInfo struct {
	size, cursor             struct{ X, Y int16 }
	attributes               uint16
	left, top, right, bottom int16
	maxSize                  struct{ X, Y int16 }
}

// TerminalSize asks the console how big it is, in columns and rows, matching
// the unix answer. ok is false when none of these is a console.
func TerminalSize() (cols, rows int, ok bool) {
	var info consoleInfo
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		r, _, _ := getConsoleScreenBufferInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&info)))
		c, n := int(info.right-info.left)+1, int(info.bottom-info.top)+1
		if r != 0 && c > 0 && n > 0 {
			return c, n, true
		}
	}
	return 0, 0, false
}
