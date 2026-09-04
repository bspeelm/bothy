package main

import (
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
	forget := ownSession(outside, cfg, "/w/proj")
	if !claimed(t, outside, "bothy-proj") {
		t.Fatal("the terminal on the host did not claim the session")
	}
	forget()
	if claimed(t, outside, "bothy-proj") {
		t.Error("the claim outlived the terminal that made it")
	}

	inside := sandbox(t, true)
	inside.Container = platform.Toolbx
	release := ownSession(inside, cfg, "/w/proj")
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

// The window closing has to end the client, or it holds the session open and
// the next launch has to clean up after it. A deferred call cannot do this:
// the default disposition for a hangup is to die.
func TestTheHangupEndsTheSession(t *testing.T) {
	// Caught here too, or the re-raise would end the test binary.
	guard := make(chan os.Signal, 2)
	signal.Notify(guard, syscall.SIGHUP)
	defer signal.Stop(guard)

	ran := make(chan struct{})
	stop := onHangup(func() { close(ran) })
	defer stop()
	hangup(t)

	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("the terminal went away and nothing ended the session")
	}
}

// And once the launch is over it stops listening, so a hangup arriving later
// does not reach into a session this process no longer has anything to do with.
func TestAFinishedLaunchStopsListening(t *testing.T) {
	guard := make(chan os.Signal, 2)
	signal.Notify(guard, syscall.SIGHUP)
	defer signal.Stop(guard)

	var ran atomic.Bool
	onHangup(func() { ran.Store(true) })()
	hangup(t)
	time.Sleep(300 * time.Millisecond)

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
