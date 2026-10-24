package main

import (
	"errors"
	"github.com/bspeelm/bothy/internal/install"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	// On macOS and Windows the question is whether this is a remote shell, so
	// an ssh session running the tests is ambient input to the thing measured.
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
}

func TestForceFlagsWin(t *testing.T) {
	clearDisplay(t)
	p := platform.Info{Terminal: "gnome-terminal"}

	if m := decideLaunch(p, "ghostty", "here"); m.Spawn {
		t.Error("--in-place must never spawn")
	}
	if m := decideLaunch(p, "ghostty", "window"); !m.Spawn {
		t.Error("--window must spawn even with no display, so the failure is visible")
	}
}

// The spawned bothy must not spawn again. This is the guard against opening
// windows forever, and it has to beat every other reason to spawn.
func TestInTerminalMarkerStopsRecursion(t *testing.T) {
	clearDisplay(t)
	t.Setenv("DISPLAY", ":0")
	t.Setenv(InTerminalEnv, "1")

	m := decideLaunch(platform.Info{Terminal: "gnome-terminal"}, "ghostty", "")
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
	m := decideLaunch(platform.Info{Terminal: "gnome-terminal"}, "ghostty", "")
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
		if m := decideLaunch(platform.Info{Terminal: "ghostty"}, "ghostty", mode); m.Reason == "" {
			t.Errorf("mode=%q produced no reason", mode)
		}
	}
}

// A container has no ghostty of its own; the terminal is a host application.
func TestGhosttyCommandUsesTheHostFromAContainer(t *testing.T) {
	p := platform.Info{Container: platform.Toolbx, ContainerName: "test"}
	cmd, err := terminalCommand(p, "ghostty")
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
	err := launch(p, config.Default(), home, "cockpit", "", remoteOpts{})
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

// #33. $DISPLAY and $WAYLAND_DISPLAY are X and Wayland. Aqua sets neither, so
// keying on them meant bothy would never open a window on a Mac however
// capable the machine was — and would report "no graphical display" on one
// with a display.
func TestDarwinHasADisplayUnlessYouAreSshedIn(t *testing.T) {
	mac := platform.Info{OS: "darwin", Home: t.TempDir()}
	clearDisplay(t)

	if !hasDisplay(mac) {
		t.Error("a Mac at its own keyboard was reported as having no display")
	}

	t.Setenv("SSH_CONNECTION", "10.0.0.1 22 10.0.0.2 22")
	if hasDisplay(mac) {
		t.Error("a Mac over ssh has no window server to open into")
	}
}

// Linux keeps the X and Wayland answer, which is the one that is true there.
func TestLinuxStillAsksTheDisplayServer(t *testing.T) {
	linux := platform.Info{OS: "linux", Home: t.TempDir()}
	clearDisplay(t)
	if hasDisplay(linux) {
		t.Error("no DISPLAY and no WAYLAND_DISPLAY should mean no display")
	}
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if !hasDisplay(linux) {
		t.Error("WAYLAND_DISPLAY set and still no display")
	}
}

// macOS installs applications as bundles and Ghostty's installer puts nothing
// on PATH, so a Mac with Ghostty installed looked to bothy like a Mac without.
func TestGhosttyIsFoundInsideAnAppBundle(t *testing.T) {
	home := t.TempDir()
	bundle := filepath.Join(home, "Applications", "Ghostty.app", "Contents", "MacOS")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "ghostty"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	restore := systemApplications
	t.Cleanup(func() { systemApplications = restore })
	systemApplications = filepath.Join(t.TempDir(), "Applications")

	cmd, err := terminalCommand(platform.Info{OS: "darwin", Home: home}, "ghostty")
	if err != nil {
		t.Fatalf("ghostty in ~/Applications was not found: %v", err)
	}
	if want := filepath.Join(bundle, "ghostty"); cmd[0] != want {
		t.Errorf("ghosttyCommand = %v, want the binary inside the bundle at %s", cmd, want)
	}
}

// And a Linux machine does not go looking for .app bundles.
func TestAppBundlesAreNotConsultedOnLinux(t *testing.T) {
	home := t.TempDir()
	bundle := filepath.Join(home, "Applications", "Ghostty.app", "Contents", "MacOS")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "ghostty"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))

	if _, err := terminalCommand(platform.Info{OS: "linux", Home: home}, "ghostty"); err == nil {
		t.Error("a Linux machine found a macOS application bundle")
	}
}

// ghostty takes --config-file=X, wezterm takes it as two words and binds it
// to the top-level command rather than the subcommand. Getting that wrong
// means wezterm silently ignores the file and falls back to its own defaults,
// so the theme vanishes with no error anywhere.
func TestEachTerminalGetsItsOwnArgvShape(t *testing.T) {
	for _, tc := range []struct {
		term string
		want []string
	}{
		{"ghostty", []string{"--config-file={config}", "-e"}},
		{"wezterm", []string{"--config-file", "{config}", "start", "--always-new-process", "--"}},
	} {
		sp, err := install.TerminalSpawn(tc.term)
		if err != nil {
			t.Errorf("%s: %v", tc.term, err)
			continue
		}
		if !slices.Equal(sp.Args, tc.want) {
			t.Errorf("%s args = %v, want %v", tc.term, sp.Args, tc.want)
		}
		if sp.Config == "" {
			t.Errorf("%s names no config file, so bothy cannot check one exists", tc.term)
		}
	}
	if _, err := install.TerminalSpawn("none"); err == nil {
		t.Error("a terminal of none should not resolve to a window to open")
	}
	if _, err := install.TerminalSpawn("nosuchterm"); err == nil {
		t.Error("an unknown terminal should be reported, not spawned")
	}
}

// A terminal that is already open was serving bothy its own defaults.
// `wezterm start` asks a running GUI instance to run the command, and that
// instance keeps the config it was launched with, so --config-file was read
// and discarded: zellij inside the window was themed, the window around it was
// not. The flag is the whole fix, so it is pinned by name rather than left to
// the argv comparison above to notice.
func TestWezTermIsNotHandedToAWindowThatIsAlreadyOpen(t *testing.T) {
	sp, err := install.TerminalSpawn("wezterm")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(sp.Args, "--always-new-process") {
		t.Errorf("wezterm args = %v, missing --always-new-process: a window already "+
			"open will run bothy with its own config and no theme", sp.Args)
	}
	cfg := slices.Index(sp.Args, "--config-file")
	start := slices.Index(sp.Args, "start")
	if cfg == -1 || start == -1 || cfg > start {
		t.Errorf("wezterm args = %v: --config-file is a global option and must "+
			"precede the start subcommand", sp.Args)
	}
	if sp.Args[len(sp.Args)-1] != "--" {
		t.Errorf("wezterm args = %v: the command bothy appends must follow --, or "+
			"its flags are parsed as wezterm's", sp.Args)
	}
}

// The branch that broke Windows. `bothy` in a WezTerm tab opened a new window
// every time, because isTerminal asked whether stdout was the null device and
// os.SameFile says yes to every console handle there (ADR-057). No test could
// see it: every other case in this file short-circuits at hasDisplay, and a
// test binary's own stdout is a pipe, so the answer was false for an
// uninteresting reason.
func TestATerminalThatCarriesTheKeysIsRunInRatherThanSpawnedFrom(t *testing.T) {
	clearDisplay(t)
	t.Setenv("DISPLAY", ":0")

	// A terminal to spawn has to exist, or the decision stops one branch short.
	dir := t.TempDir()
	spawnable := filepath.Join(dir, platform.ExeName(runtime.GOOS, "ghostty"))
	if err := os.WriteFile(spawnable, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	restore := stdoutIsTerminal
	t.Cleanup(func() { stdoutIsTerminal = restore })
	stdoutIsTerminal = func() bool { return true }

	for _, tc := range []struct {
		terminal  string
		wantSpawn bool
	}{
		{"wezterm", false},
		{"ghostty", false},
		{"gnome-terminal", true},
	} {
		m := decideLaunch(platform.Info{OS: runtime.GOOS, Terminal: tc.terminal}, "ghostty", "")
		if m.Spawn != tc.wantSpawn {
			t.Errorf("in %s: Spawn = %v, want %v (%s)", tc.terminal, m.Spawn, tc.wantSpawn, m.Reason)
		}
	}

	// And with no terminal on the other end, a window is the only way to show
	// anything -- which is what the branch is for.
	stdoutIsTerminal = func() bool { return false }
	if m := decideLaunch(platform.Info{OS: runtime.GOOS, Terminal: "wezterm"}, "ghostty", ""); !m.Spawn {
		t.Error("started without a terminal and bothy ran in place anyway")
	}
}
