package mux

import (
	"os"
	"runtime"
	"testing"
)

// zellij binds under $XDG_RUNTIME_DIR/zellij when it is set, which it is on
// every desktop Linux. The budget was measured against /tmp/zellij-<uid>
// instead -- five bytes shorter -- so FitSocket handed back names too long to
// bind, and `bothy connect` builds the longest ones.
func TestTheSocketDirectoryIsTheOneZellijWillUse(t *testing.T) {
	if got := socketDir("", "/run/user/1000", "/tmp", 1000); got != "/run/user/1000/zellij" {
		t.Errorf("socketDir = %q; zellij binds under the runtime directory", got)
	}
	// Its own override wins, as zellij's does.
	if got := socketDir("/somewhere/else", "/run/user/1000", "/tmp", 1000); got != "/somewhere/else" {
		t.Errorf("socketDir = %q, want the override", got)
	}
	// Only with no runtime directory does the temporary one apply.
	if got := socketDir("", "", "/tmp", 1000); got != "/tmp/zellij-1000" {
		t.Errorf("socketDir = %q, want the temporary fallback", got)
	}
	// The budget must shrink accordingly: the runtime path is the longer one,
	// and measuring against the shorter is what allowed unbindable names.
	runtime := socketRoom("linux", socketDir("", "/run/user/1000", "/tmp", 1000))
	temp := socketRoom("linux", socketDir("", "", "/tmp", 1000))
	if runtime >= temp {
		t.Errorf("room under the runtime dir (%d) is not less than under /tmp (%d)", runtime, temp)
	}
}

// The condition this fix was accepted under: Linux keeps the names it has.
// The runtime directory leaves room for 66 and every name bothy builds is
// shorter, so the shortening is macOS's long $TMPDIR and nothing else.
func TestLinuxSessionNamesAreNeverRewritten(t *testing.T) {
	room := socketRoom("linux", "/run/user/1000/zellij")
	for _, n := range []string{"bothy-<client>", "bothy-longishaccountname-203-0-113-42"} {
		if len(n) > room {
			t.Errorf("%q (%d bytes) would be rewritten on Linux, which has room for %d", n, len(n), room)
		}
	}
	// Measured on macOS 25.6: zellij refused a 115-byte path, max 103.
	if r := socketRoom("darwin", "/var/folders/y2/x1y2z3a4b5c6d7e8f9g0h1i2j3k000/T/zellij-501"); r != 24 {
		t.Errorf("socketRoom(darwin) = %d, want the measured 24", r)
	}
}

// zellij exited with "the IPC socket path is too long (115 bytes, max 103)"
// before the workspace opened, which read as bothy printing terminal garbage.
func TestAnOverlongSessionNameIsShortenedAndStaysUnique(t *testing.T) {
	t.Setenv("ZELLIJ_SOCKET_DIR", "/var/folders/y2/x1y2z3a4b5c6d7e8f9g0h1i2j3k000/T/zellij-501")
	room := socketRoom(runtime.GOOS, os.Getenv("ZELLIJ_SOCKET_DIR"))
	long := "bothy-averylongaccountname-192-168-30-93-with-a-deep-project-directory"
	a, b := FitSocket(long), FitSocket(long+"4")
	if len(a) > room {
		t.Errorf("FitSocket = %q (%d bytes), over the %d it has", a, len(a), room)
	}
	if a == b {
		t.Errorf("two machines collapsed onto one session name %q -- ADR-046 keeps them apart", a)
	}
	if got := FitSocket("bothy-<client>"); got != "bothy-<client>" {
		t.Errorf("a name that already fits was rewritten to %q", got)
	}
}
