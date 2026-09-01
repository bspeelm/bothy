package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
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

// ghosttyCommand returns how to run ghostty. Inside a container it is a host
// application, reached through flatpak-spawn --host.
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

	// Start, not Run: the shell that typed `bothy` gets its prompt back instead
	// of blocking on the window the user is now working in.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open a terminal: %w", err)
	}
	fmt.Println("opened a Ghostty window")
	return cmd.Process.Release()
}
