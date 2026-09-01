package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
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

	if m := decideLaunch(p, "here"); m.Spawn {
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
	for _, mode := range []string{"", "auto", "window", "here"} {
		if m := decideLaunch(platform.Info{Terminal: "ghostty"}, mode); m.Reason == "" {
			t.Errorf("mode=%q produced no reason", mode)
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

// A command that fails must not leave a directory tree behind. `bothy dev`
// used to render the layout before checking anything could use it, so running
// it on a machine that was never installed created a partial tree and then
// reported failure — debris on the machine of someone who ran one command and
// was told it did not work.
func TestDevLeavesNothingBehindWhenNotInstalled(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, "share")

	p := platform.Info{
		Home: home, DataDir: data,
		ConfigDir: filepath.Join(home, "config"),
		LocalBin:  filepath.Join(home, "bin"),
	}
	err := launch(p, config.Default(), home, "cockpit")
	if err == nil {
		t.Fatal("expected a refusal when bothy is not installed")
	}
	// The refusal has to name the fix. "no such file or directory" would be
	// technically true and useless.
	if !strings.Contains(err.Error(), "bothy install") {
		t.Errorf("the refusal does not say how to fix it: %v", err)
	}
	entries, err := os.ReadDir(data)
	if err == nil && len(entries) > 0 {
		t.Errorf("a failed launch created %d path(s) under %s", len(entries), data)
	}
}

// #50. Inside a container the host's ghostty runs the host's copy of bothy,
// and which file that is was assumed to be ~/.local/bin/bothy. dnf and deb
// both put it in /usr/bin, and both are supported install paths.
func TestHostBothyAsksTheHostRatherThanAssuming(t *testing.T) {
	p := platform.Info{Home: "/home/me", LocalBin: "/home/me/.local/bin"}

	restore := hostBothyLookup
	t.Cleanup(func() { hostBothyLookup = restore })

	hostBothyLookup = func() (string, error) { return "/usr/bin/bothy", nil }
	got, err := hostBothy(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/usr/bin/bothy" {
		t.Errorf("hostBothy = %q, want the path the host reported", got)
	}
}

// A host PATH without ~/.local/bin is common in a non-login shell, and home is
// shared, so the old assumption survives as a fallback rather than as the rule.
func TestHostBothyFallsBackToTheSharedHome(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "bothy"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := platform.Info{Home: home, LocalBin: bin}

	restore := hostBothyLookup
	t.Cleanup(func() { hostBothyLookup = restore })
	hostBothyLookup = func() (string, error) { return "", errors.New("no flatpak-spawn") }

	got, err := hostBothy(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(bin, "bothy"); got != want {
		t.Errorf("hostBothy = %q, want the shared-home copy at %q", got, want)
	}
}

// And when the host has neither, that is an error naming what to do about it
// rather than a path that does not exist.
func TestHostBothyReportsWhenTheHostHasNone(t *testing.T) {
	p := platform.Info{Home: "/home/me", LocalBin: filepath.Join(t.TempDir(), "empty")}

	restore := hostBothyLookup
	t.Cleanup(func() { hostBothyLookup = restore })
	hostBothyLookup = func() (string, error) { return "", errors.New("no flatpak-spawn") }

	if _, err := hostBothy(p); err == nil {
		t.Error("hostBothy invented a path for a host with no bothy")
	}
}
