package main

import (
	"strings"
	"testing"

	"github.com/bothy-dev/bothy/internal/platform"
)

// clearDisplay removes any graphical session from the environment, so a test
// exercises a branch rather than whatever machine it happens to run on.
func clearDisplay(t *testing.T) {
	t.Helper()
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv(InTerminalEnv, "")
}

func TestForceFlagsWin(t *testing.T) {
	clearDisplay(t)
	p := platform.Info{Terminal: "gnome-terminal"}

	if m := decideLaunch(p, "in-place"); m.Spawn {
		t.Error("--in-place must never spawn")
	}
	if m := decideLaunch(p, "window"); !m.Spawn {
		t.Error("--window must spawn even with no display, so the failure is visible")
	}
}

// The spawned bothy must not spawn again. This is the guard against opening
// windows forever, and it has to beat every other reason to spawn.
func TestInTerminalMarkerStopsRecursion(t *testing.T) {
	clearDisplay(t)
	t.Setenv("DISPLAY", ":0")
	t.Setenv(InTerminalEnv, "1")

	m := decideLaunch(platform.Info{Terminal: "gnome-terminal"}, "")
	if m.Spawn {
		t.Fatal("a bothy started by its own terminal must not open another")
	}
	if !strings.Contains(m.Reason, "already inside") {
		t.Errorf("reason = %q", m.Reason)
	}
}

// On SSH or a bare TTY there is nothing to open a window into. Running with
// degraded previews beats failing to launch at all.
func TestNoDisplayRunsInPlace(t *testing.T) {
	clearDisplay(t)
	m := decideLaunch(platform.Info{Terminal: "gnome-terminal"}, "")
	if m.Spawn {
		t.Fatal("spawned a window with no graphical display")
	}
	if !strings.Contains(m.Reason, "display") {
		t.Errorf("reason = %q, should say why", m.Reason)
	}
}

// Every decision must explain itself: this is what the user reads when bothy
// opens a window they did not ask for.
func TestEveryDecisionCarriesAReason(t *testing.T) {
	clearDisplay(t)
	for _, force := range []string{"", "window", "in-place"} {
		if m := decideLaunch(platform.Info{Terminal: "ghostty"}, force); m.Reason == "" {
			t.Errorf("force=%q produced no reason", force)
		}
	}
}

// A container has no ghostty of its own; the terminal is a host application.
func TestGhosttyCommandUsesTheHostFromAContainer(t *testing.T) {
	p := platform.Info{Container: platform.Toolbx, ContainerName: "test"}
	cmd, err := ghosttyCommand(p)
	if err != nil {
		t.Skip("no ghostty reachable from here")
	}
	if len(cmd) > 1 && cmd[1] != "--host" {
		t.Errorf("got %v; from a container the launch should go to the host", cmd)
	}
}
