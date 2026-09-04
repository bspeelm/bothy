package mux

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// Real argv, read off /proc on a machine showing the bug. The server is the
// one that matters: it carries the session name too, inside the socket path it
// was handed, and reclaiming it would take the whole workspace down.
func TestAClientArgvIsTheOneAttachedToThisSession(t *testing.T) {
	const bin = "/home/me/.local/share/bothy/bin/zellij"
	for _, tc := range []struct {
		name string
		argv []string
		want bool
	}{
		{"the launching client", []string{bin, "--layout", "/c/cockpit.kdl", "attach", "--create", "bothy-work"}, true},
		{"a plain client", []string{bin, "attach", "bothy-work"}, true},
		{"the server", []string{bin, "--server", "/run/user/1000/zellij/contract_version_1/bothy-work"}, false},
		{"another project", []string{bin, "attach", "bothy-other"}, false},
		{"a pane", []string{"/usr/bin/yazi"}, false},
		{"the wrapper, not the client", []string{"/usr/bin/bothy", "attach", "bothy-work"}, false},
		{"nothing at all", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isClientOf(tc.argv, "zellij", "bothy-work"); got != tc.want {
				t.Errorf("isClientOf(%v) = %v, want %v", tc.argv, got, tc.want)
			}
		})
	}
}

func TestOwningASessionAndForgettingIt(t *testing.T) {
	dir := t.TempDir()
	forget := Own(dir, "bothy-work")
	if !Owned(dir, "bothy-work") {
		t.Fatal("the process that just claimed the session does not own it")
	}
	forget()
	if Owned(dir, "bothy-work") {
		t.Error("the session is still owned after the terminal let it go")
	}
	if Owned(dir, "never-claimed") {
		t.Error("a session nobody claimed reports an owner")
	}
}

// The window closing kills the bothy that wrote the record, so it never runs
// the forget func. That is the whole signal: a record whose process is gone.
func TestAnOwnerRecordGoesStaleWhenItsProcessDoes(t *testing.T) {
	dir := t.TempDir()
	done := exec.Command("/bin/true")
	if err := done.Run(); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "bothy-work", strconv.Itoa(done.Process.Pid)+" 12345")
	if Owned(dir, "bothy-work") {
		t.Error("a record naming a process that has exited still reads as owned")
	}
}

// Pids are reused. Without the start time, a record would come back to life
// under whatever process next took its number.
func TestAReusedPidDoesNotLookLikeAnOwner(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bothy-work", strconv.Itoa(os.Getpid())+" 1")
	if Owned(dir, "bothy-work") {
		t.Error("a live pid with someone else's start time reads as the owner")
	}
}

// Not knowing must refuse, not reclaim: the caller kills what this reports as
// unowned. This is also the macOS and Windows answer, asserted on Linux.
func TestWithoutProcNothingIsKnownAndNothingIsReclaimed(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bothy-work", "1 1")

	restore := procRoot
	defer func() { procRoot = restore }()
	procRoot = filepath.Join(t.TempDir(), "no-proc")

	if !Owned(dir, "bothy-work") {
		t.Error("an unreadable /proc reported the session unowned, which invites a kill")
	}
	if n := Reclaim("zellij", "bothy-work"); n != 0 {
		t.Errorf("reclaimed %d clients with no /proc to read", n)
	}
}

// A client on this side of the container boundary dies with its terminal in
// milliseconds, so one still running is one somebody is using. Reclaim must
// never touch it, whatever its argv looks like.
func TestAClientOnThisSideOfTheBoundaryIsNeverReclaimed(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	session := "bothy-test-" + strconv.Itoa(os.Getpid())
	// "; true" keeps the shell alive as the parent. Without it sh execs sleep
	// for a lone command, the fake argv goes with it, and the test passes by
	// matching nothing at all.
	cmd := exec.Command("/bin/sh")
	cmd.Args = []string{"zellij", "-c", "sleep 30; true", "attach", session}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Reaped in the background: an unreaped child stays in /proc as a zombie,
	// and a test that cannot tell that from a survivor proves nothing.
	go func() { _ = cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill() }()
	// Start returns before the exec lands, and /proc/PID/cmdline is empty
	// until it does. Reclaiming nothing because nothing was there yet would
	// pass this test for the wrong reason.
	waitForArgv(t, cmd.Process.Pid)

	if n := Reclaim("zellij", session); n != 0 {
		t.Errorf("reclaimed %d clients that share this mount namespace", n)
	}
	if !alive(cmd.Process.Pid) {
		t.Error("the process was killed despite being on this side of the boundary")
	}
}

func waitForArgv(t *testing.T, pid int) {
	t.Helper()
	path := filepath.Join(procRoot, strconv.Itoa(pid), "cmdline")
	for i := 0; i < 100; i++ {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the process never showed an argv, so nothing would have matched it")
}

func write(t *testing.T, dir, session, body string) {
	t.Helper()
	if err := os.MkdirAll(ownerDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ownerDir(dir), session), []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
