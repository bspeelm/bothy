package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bothy-dev/bothy/internal/install"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/probe"
)

// InTerminalEnv marks a bothy that was started by a terminal bothy spawned.
// Without it the inner process would open another window, and so on.
const InTerminalEnv = "BOTHY_IN_TERMINAL"

// launchMode is the decision about where the workspace runs.
type launchMode struct {
	Spawn  bool
	Reason string
}

// decideLaunch works out whether to run here or open a terminal that can do
// the job.
//
// This is not about aesthetics. Inline image previews need a terminal that
// speaks the Kitty graphics protocol; run bothy inside GNOME Terminal and Yazi
// silently degrades to block art. Silent degradation is the class of failure
// this project exists to remove, so bothy would rather open a terminal that
// works than pretend the one you have is fine.
//
// The order matters: every reason to stay put is checked before any reason to
// spawn, because a spawn that cannot work is worse than a degraded workspace.
func decideLaunch(p platform.Info, force string) launchMode {
	switch force {
	case "in-place":
		return launchMode{Reason: "--in-place was given"}
	case "window":
		return launchMode{Spawn: true, Reason: "--window was given"}
	}

	// We are the process the spawned terminal started. Opening another would
	// recurse forever.
	if os.Getenv(InTerminalEnv) != "" {
		return launchMode{Reason: "already inside the terminal bothy opened"}
	}

	// No graphical session to open a window into: SSH, a bare TTY, CI.
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return launchMode{Reason: "no graphical display; running here"}
	}

	if _, err := ghosttyCommand(p); err != nil {
		return launchMode{Reason: "ghostty is not installed; running here"}
	}

	// Launched from a desktop icon or a script: there is no terminal to run
	// in, so one has to be opened whatever the graphics situation.
	if !stdoutIsTerminal() {
		return launchMode{Spawn: true, Reason: "started without a terminal"}
	}

	if g := probe.CheckGraphics("", p.Terminal); !g.Supported {
		return launchMode{Spawn: true, Reason: g.Reason}
	}
	return launchMode{Reason: "this terminal can draw images; running here"}
}

// stdoutIsTerminal reports whether stdout is a character device. This is the
// stdlib-only way to ask, and PLAN.md §13 caps dependencies at go-toml.
func stdoutIsTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ghosttyCommand returns how to run ghostty, which is not always directly.
//
// Inside a container there is no ghostty and no desktop: the terminal is a
// host application. flatpak-spawn hands the launch to the host, exactly as the
// xdg-open shim hands over file opening.
func ghosttyCommand(p platform.Info) ([]string, error) {
	if path, err := exec.LookPath("ghostty"); err == nil {
		return []string{path}, nil
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

// spawnTerminal opens Ghostty with bothy's own config and runs bothy inside it.
//
// --config-file is what keeps this isolated: Ghostty reads bothy's file and
// never looks at ~/.config/ghostty. The palette is written into that file
// rather than named as a theme, because theme lookup paths are not relocatable
// (ADR-009).
func spawnTerminal(p platform.Info, dir, profileName string) error {
	term, err := ghosttyCommand(p)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find the bothy binary to run: %w", err)
	}
	// Inside a container the host's ghostty runs the host's copy of bothy —
	// the same file, because home is shared. That copy hops back in.
	if len(term) > 1 && term[1] == "--host" {
		self = filepath.Join(p.LocalBin, "bothy")
	}

	conf := install.GhosttyConf(p)
	if _, err := os.Stat(conf); err != nil {
		return fmt.Errorf("%s does not exist — run 'bothy install' first", conf)
	}

	args := append([]string{}, term[1:]...)
	args = append(args,
		"--config-file="+conf,
		"-e", self, "dev", "--dir", dir, "--profile", profileName,
	)

	cmd := exec.Command(term[0], args...)
	cmd.Env = append(os.Environ(), InTerminalEnv+"=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start rather than Run: the point is a new window, so the shell that
	// typed `bothy` gets its prompt back instead of blocking on a terminal the
	// user is now working in.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open a terminal: %w", err)
	}
	fmt.Println("opened a Ghostty window")
	return cmd.Process.Release()
}
