package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
)

// InTerminalEnv marks a bothy that was started by a terminal bothy spawned.
// Without it the inner process would open another window, and so on.
const InTerminalEnv = "BOTHY_IN_TERMINAL"

type launchMode struct {
	Spawn  bool
	Reason string
}

// decideLaunch works out whether to run here or open a terminal that can do
// the job. Inline image previews need a terminal that speaks the Kitty
// graphics protocol; inside GNOME Terminal, Yazi degrades silently to block
// art, so bothy opens a terminal that works instead.
//
// mode is workspace.launch, or the flag that overrode it for this run:
// "here" and "window" settle the question, "auto" and "" ask it.
//
// Every reason to stay put is checked before any reason to spawn.
func decideLaunch(p platform.Info, mode string) launchMode {
	switch mode {
	case "here":
		return launchMode{Reason: "asked to run in this terminal"}
	case "window":
		return launchMode{Spawn: true, Reason: "asked for a window"}
	}

	// We are the process the spawned terminal started; another would recurse.
	if os.Getenv(InTerminalEnv) != "" {
		return launchMode{Reason: "already inside the terminal bothy opened"}
	}

	// No graphical session to open a window into: SSH, a bare TTY, CI.
	//
	// $DISPLAY and $WAYLAND_DISPLAY are X and Wayland, so they say nothing
	// about a Mac. Aqua sets neither, and keying on them meant bothy would
	// never open a window on macOS however capable the machine was.
	if !hasDisplay(p) {
		return launchMode{Reason: "no graphical display; running here"}
	}

	if _, err := ghosttyCommand(p); err != nil {
		return launchMode{Reason: "ghostty is not installed; running here"}
	}

	// Launched from a desktop icon or a script: there is no terminal to run
	// in, so one has to be opened whatever the graphics situation.
	if !isTerminal(os.Stdout) {
		return launchMode{Spawn: true, Reason: "started without a terminal"}
	}

	if g := probe.CheckGraphics("", p.Terminal); !g.Supported {
		return launchMode{Spawn: true, Reason: g.Reason}
	}
	return launchMode{Reason: "this terminal can draw images; running here"}
}

// isTerminal reports whether a stream is an interactive terminal. /dev/null is
// also a character device, so it is excluded explicitly; that covers redirects,
// cron and systemd units without an ioctl (no x/term — PLAN.md §13).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}

// hasDisplay reports whether there is a session to open a window into.
//
// On darwin, an SSH login has no window server either, and $SSH_CONNECTION is
// how that shows: the alternative is asking launchd, which is a great deal of
// machinery to answer a question one environment variable already settles.
func hasDisplay(p platform.Info) bool {
	if p.OS == "darwin" {
		return os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == ""
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// ghosttyCommand returns how to run ghostty. Inside a container it is a host
// application, reached through flatpak-spawn --host.
func ghosttyCommand(p platform.Info) ([]string, error) {
	if path, err := exec.LookPath("ghostty"); err == nil {
		return []string{path}, nil
	}
	// macOS installs applications as bundles, and Ghostty's own installer puts
	// nothing on PATH. The binary inside the bundle is a normal executable and
	// takes the same arguments, so this is a path to find rather than a
	// different way to launch.
	if p.OS == "darwin" {
		for _, dir := range []string{"/Applications", filepath.Join(p.Home, "Applications")} {
			bin := filepath.Join(dir, "Ghostty.app", "Contents", "MacOS", "ghostty")
			if fi, err := os.Stat(bin); err == nil && !fi.IsDir() {
				return []string{bin}, nil
			}
		}
	}
	if p.InContainer() {
		if fs, err := exec.LookPath("flatpak-spawn"); err == nil {
			if err := exec.Command(fs, "--host", "sh", "-c", "command -v ghostty").Run(); err == nil {
				return []string{fs, "--host", "ghostty"}, nil
			}
		}
	}
	return nil, fmt.Errorf("ghostty not found")
}

// hostBothyLookup asks the host where its bothy is. A package variable so a
// test can answer without a container and a host to spawn into.
var hostBothyLookup = func() (string, error) {
	fs, err := exec.LookPath("flatpak-spawn")
	if err != nil {
		return "", err
	}
	out, err := exec.Command(fs, "--host", "sh", "-c", "command -v bothy").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hostBothy is the path the host should run.
//
// Asking the host beats assuming: a dnf or deb install puts its copy in
// /usr/bin, which is nowhere under home, and both are supported install paths.
//
// The fallback covers a host PATH that omits ~/.local/bin in a non-login
// shell, which is common enough that reporting "not found" would usually be
// wrong. It is checkable from in here because home is shared.
func hostBothy(p platform.Info) (string, error) {
	if path, err := hostBothyLookup(); err == nil && path != "" {
		return path, nil
	}
	fallback := filepath.Join(p.LocalBin, "bothy")
	if _, err := os.Stat(fallback); err == nil {
		return fallback, nil
	}
	return "", fmt.Errorf("the host has no bothy on its PATH, and none at %s\n"+
		"      install it on the host too, or run bothy with --in-place", fallback)
}

// spawnTerminal opens Ghostty on bothy's own config and runs bothy inside it.
// The palette is inlined because Ghostty theme lookup paths are not relocatable.
func spawnTerminal(p platform.Info, dir, profileName string) error {
	term, err := ghosttyCommand(p)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find the bothy binary to run: %w", err)
	}
	// Inside a container the host's ghostty runs the host's copy of bothy,
	// which then hops back in. The container's own os.Executable() is not that
	// copy: home is shared, so the path may name a different file on the other
	// side, or none.
	if len(term) > 1 && term[1] == "--host" {
		host, err := hostBothy(p)
		if err != nil {
			return err
		}
		self = host
	}

	conf := install.GhosttyConf(p)
	if _, err := os.Stat(conf); err != nil {
		return fmt.Errorf("%s does not exist — run 'bothy install' first", conf)
	}

	args := append([]string{}, term[1:]...)
	args = append(args,
		"--config-file="+conf,
		"-e", self, "--dir", dir, "--profile", profileName,
	)

	cmd := exec.Command(term[0], args...)
	cmd.Env = append(os.Environ(), InTerminalEnv+"=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start, not Run: the shell that typed `bothy` gets its prompt back instead
	// of blocking on the window the user is now working in.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open a terminal: %w", err)
	}
	fmt.Println("opened a Ghostty window")
	return cmd.Process.Release()
}
