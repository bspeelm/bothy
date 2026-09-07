package mux

import (
	"os"
	"runtime"
	"testing"
)

// The condition this fix was accepted under: Linux keeps the names it has.
// /tmp leaves room for 71 and every name bothy builds is shorter, so the
// shortening is macOS's long $TMPDIR and nothing else.
func TestLinuxSessionNamesAreNeverRewritten(t *testing.T) {
	room := socketRoom("linux", "/tmp/zellij-1000")
	for _, n := range []string{"bothy-abbey", "bothy-shadowthebearded-192-168-30-93"} {
		if len(n) > room {
			t.Errorf("%q (%d bytes) would be rewritten on Linux, which has room for %d", n, len(n), room)
		}
	}
	// Measured on macOS 25.6: zellij refused a 115-byte path, max 103.
	if r := socketRoom("darwin", "/var/folders/y2/2ld819k10yb0c4c11sq7gckm0000gn/T/zellij-501"); r != 24 {
		t.Errorf("socketRoom(darwin) = %d, want the measured 24", r)
	}
}

// zellij exited with "the IPC socket path is too long (115 bytes, max 103)"
// before the workspace opened, which read as bothy printing terminal garbage.
func TestAnOverlongSessionNameIsShortenedAndStaysUnique(t *testing.T) {
	t.Setenv("ZELLIJ_SOCKET_DIR", "/var/folders/y2/2ld819k10yb0c4c11sq7gckm0000gn/T/zellij-501")
	room := socketRoom(runtime.GOOS, os.Getenv("ZELLIJ_SOCKET_DIR"))
	long := "bothy-averylongaccountname-192-168-30-93-with-a-deep-project-directory"
	a, b := FitSocket(long), FitSocket(long+"4")
	if len(a) > room {
		t.Errorf("FitSocket = %q (%d bytes), over the %d it has", a, len(a), room)
	}
	if a == b {
		t.Errorf("two machines collapsed onto one session name %q -- ADR-046 keeps them apart", a)
	}
	if got := FitSocket("bothy-abbey"); got != "bothy-abbey" {
		t.Errorf("a name that already fits was rewritten to %q", got)
	}
}
