package platform

import (
	"os"
	"strings"
	"testing"
)

// A pipe is not a terminal, on either platform. This is the half a test can
// assert anywhere: the other half, that a real console answers true, needs a
// console and is measured on the Windows box instead (ADR-057).
func TestAPipeIsNotATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(w) {
		t.Error("a pipe reported as a terminal; every prompt would wait on nobody")
	}

	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no %s here: %v", os.DevNull, err)
	}
	defer null.Close()
	if IsTerminal(null) {
		t.Errorf("%s reported as a terminal", os.DevNull)
	}
}

// The Windows arm may not ask the question the Unix one asks. os.SameFile
// against NUL is true for every handle there -- a pipe, a file, a console --
// so the guard that is correct on Unix answers "not a terminal" for
// everything, and bothy opened a window on every launch for five releases.
func TestTheWindowsArmDoesNotCompareAgainstTheNullDevice(t *testing.T) {
	src, err := os.ReadFile("isterm_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "SameFile(") {
		t.Error("isterm_windows.go compares against the null device again; on Windows " +
			"that comparison is true for every handle (ADR-057)")
	}
	if !strings.Contains(string(src), "GetConsoleMode") {
		t.Error("isterm_windows.go no longer asks the console API, which is the only " +
			"answer Windows gives to this question")
	}
}
