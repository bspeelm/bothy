package main

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

// The nesting guard and the agent list used to be two lists that disagreed:
// aider had a guard and no provider, codex and opencode had neither. Driving
// the guard from the providers is only worth anything if every variable a
// provider declares actually reaches it.
func TestEveryAgentsDetectVariableReachesTheGuard(t *testing.T) {
	all, err := slots.All()
	if err != nil {
		t.Fatal(err)
	}
	// The real environment already carries some of these -- this test suite
	// may well be running inside an agent -- so clear them all first.
	for _, pr := range all {
		for _, name := range pr.Detect {
			t.Setenv(name, "")
		}
	}
	agents := 0
	for _, pr := range all {
		if pr.Slot != "agent" {
			continue
		}
		agents++
		if len(pr.Detect) == 0 {
			t.Errorf("%s declares no detect variables, so bothy would open a "+
				"workspace inside it and start a second copy", pr.Name)
		}
		for _, name := range pr.Detect {
			t.Setenv(name, "1")
			got, nested := nestedAgent()
			if !nested || got != pr.Name {
				t.Errorf("%s=1 gave (%q, %v), want (%q, true)", name, got, nested, pr.Name)
			}
			t.Setenv(name, "")
		}
	}
	if agents < 3 {
		t.Errorf("found %d agent providers, expected at least claude-code, gemini-cli, aider", agents)
	}
}

// The bug, named. A client with no terminal behind it must not shut the
// project: closing the window is the ordinary way to stop working, and the
// next launch has to get back in.
func TestClosingTheWindowDoesNotLockTheProjectOut(t *testing.T) {
	if got := clientVerdict(1, true, false); got != reclaim {
		t.Errorf("a lone unowned client gives %v, want reclaim", got)
	}
}

// And the guarantee that came with the refusal is untouched: a session
// somebody is watching is never taken from them.
func TestAClientSomeoneIsHoldingIsNeverReclaimed(t *testing.T) {
	if got := clientVerdict(2, true, true); got != refuse {
		t.Errorf("an owned session gives %v, want refuse", got)
	}
}

// A probe that could not answer is not a reason to block a launch, and
// certainly not a reason to kill anything.
func TestALaunchIsNeverBlockedByAProbeThatFailed(t *testing.T) {
	for _, tc := range []struct {
		name           string
		n              int
		counted, owned bool
	}{
		{"the probe failed", 2, false, false},
		{"nobody is attached", 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientVerdict(tc.n, tc.counted, tc.owned); got != proceed {
				t.Errorf("got %v, want proceed", got)
			}
		})
	}
}

// The copy of bothy inside the container outlives the window, so a record it
// wrote would never go stale and the project would be shut permanently. This
// guards the guard: without it the fix becomes a worse version of the bug.
func TestABothyInsideTheContainerDoesNotClaimTheSession(t *testing.T) {
	cfg := config.Default()

	outside := sandbox(t, true)
	// The name is derived by the caller now, so deriving it here keeps that
	// step in the test rather than hard-coding what it produces.
	forget := ownSession(outside, cfg, sessionNameFor(outside, cfg, "/w/proj"))
	if !claimed(t, outside, "bothy-proj") {
		t.Fatal("the terminal on the host did not claim the session")
	}
	forget()
	if claimed(t, outside, "bothy-proj") {
		t.Error("the claim outlived the terminal that made it")
	}

	inside := sandbox(t, true)
	inside.Container = platform.Toolbx
	release := ownSession(inside, cfg, sessionNameFor(inside, cfg, "/w/proj"))
	defer release()
	if claimed(t, inside, "bothy-proj") {
		t.Error("a bothy inside the container claimed a session it can never release")
	}
}

// claimed reports whether a record was written at all, which is what the
// container guard is about -- not whether it is still live.
func claimed(t *testing.T, p platform.Info, session string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(p.StateDir(), "sessions", session))
	return err == nil
}

// hangups catches SIGHUP for the whole test binary.
//
// These tests raise a hangup at themselves, and onHangup re-raises it so a
// real exit looks like what it is. That re-raise is asynchronous, and two
// things went wrong with it. With no handler registered when it lands, Go
// restores the default disposition -- which is to die, and it killed the test
// binary. And landing inside a later test, it looks to that test like a
// hangup nobody sent.
//
// One registration for the binary fixes the first. Each test that causes a
// hangup waits for it here before returning, which fixes the second.
var hangups = make(chan os.Signal, 8)

func TestMain(m *testing.M) {
	signal.Notify(hangups, syscall.SIGHUP)
	os.Exit(m.Run())
}

// awaitHangups takes n hangups off the binary's handler, so none is still in
// flight when the next test starts.
func awaitHangups(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-hangups:
		case <-time.After(3 * time.Second):
			t.Fatalf("%d of %d hangups arrived", i, n)
		}
	}
}

// The window closing has to end the client, or it holds the session open and
// the next launch has to clean up after it. A deferred call cannot do this:
// the default disposition for a hangup is to die.
func TestTheHangupEndsTheSession(t *testing.T) {
	ran := make(chan struct{})
	stop := onHangup(func() { close(ran) })
	defer stop()
	hangup(t)

	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("the terminal went away and nothing ended the session")
	}
	// The one sent above and the one onHangup re-raises.
	awaitHangups(t, 2)
}

// And once the launch is over it stops listening, so a hangup arriving later
// does not reach into a session this process no longer has anything to do with.
func TestAFinishedLaunchStopsListening(t *testing.T) {
	var ran atomic.Bool
	onHangup(func() { ran.Store(true) })()
	hangup(t)
	// Only the one sent here: a stopped listener re-raises nothing.
	awaitHangups(t, 1)

	if ran.Load() {
		t.Error("a hangup after the launch finished still ended a session")
	}
}

// Same guard as ownSession, for the same reason: the copy inside the container
// is not the terminal, and must not act as though the terminal had gone.
func TestABothyInsideTheContainerEndsNothing(t *testing.T) {
	cfg := config.Default()
	inside := sandbox(t, true)
	inside.Container = platform.Toolbx

	if err := os.MkdirAll(filepath.Join(inside.StateDir(), "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(inside.StateDir(), "sessions", "bothy-proj")
	if err := os.WriteFile(record, []byte("1 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	endTheSession(inside, cfg, "/w/proj")()

	if _, err := os.Stat(record); err != nil {
		t.Error("a bothy inside the container tore down a session it does not own")
	}
}

func hangup(t *testing.T) {
	t.Helper()
	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := self.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
}

// Registering half of the pair is the failure this exists to prevent: an owner
// with no hangup handler leaves the session running when the window closes, and
// a handler with no owner cannot tell an abandoned client from a live one. Every
// command that opens a workspace must register both.
func TestEveryCommandThatOpensAWorkspaceWatchesIt(t *testing.T) {
	for _, f := range []string{"dev.go", "confinecmd.go", "connectcmd.go", "towercmd.go"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "watching(p, cfg,") {
			t.Errorf("%s opens a workspace and does not register a watcher for it", f)
		}
	}
	// And nothing registers half of it directly, which is what the helper is for.
	for _, f := range []string{"confinecmd.go", "connectcmd.go", "towercmd.go"} {
		body, _ := os.ReadFile(f)
		for _, half := range []string{"ownSession(", "onHangup("} {
			if strings.Contains(string(body), half) {
				t.Errorf("%s calls %s directly rather than watching()", f, half)
			}
		}
	}
}

// Closing a window ends the session, which is what most people mean by closing
// it. What is recorded on disk is untouched: the agent's transcript is what
// /resume reads and no multiplexer call reaches it.
func TestClosingTheWindowEndsTheSession(t *testing.T) {
	body, err := os.ReadFile("dev.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	end := strings.Index(src, "func endTheSession")
	if end < 0 {
		t.Fatal("endTheSession is gone")
	}
	after := src[end:]
	if stop := strings.Index(after, "\nfunc "); stop > 0 {
		after = after[:stop]
	}
	if !strings.Contains(after, "backend.Kill(bin, env, session)") {
		t.Error("the hangup no longer ends the session, only its clients")
	}
	if !strings.Contains(after, "mux.Reclaim(") {
		t.Error("the reclaim is gone; it is still the net for a crash or a reboot")
	}
}

// A session nobody is looking at reads differently from the one being worked in.
// Silence when the multiplexer will not answer: "could not ask" is not "nobody
// is looking", and reporting the second when you mean the first is what makes a
// listing worth ignoring.
func TestASessionWithNoWindowSaysSo(t *testing.T) {
	for _, tt := range []struct {
		clients    int
		counted    bool
		note, want string
	}{
		{0, true, "", "detached"},
		{0, true, "this directory", ", detached"},
		{1, true, "", ""},
		{0, false, "", ""},
	} {
		if got := lonely(tt.clients, tt.counted, tt.note); got != tt.want {
			t.Errorf("lonely(%d, %v, %q) = %q, want %q", tt.clients, tt.counted, tt.note, got, tt.want)
		}
	}
}
