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
// the job: image previews need the Kitty graphics protocol, and inside GNOME
// Terminal Yazi degrades to block art silently.
//
// mode is workspace.launch or the flag overriding it: "here" and "window" are
// decisions, "auto" and "" are questions. Reasons to stay put are checked
// before reasons to spawn.
func decideLaunch(p platform.Info, term, mode string) launchMode {
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

	// No graphical session to open a window into: SSH, a bare TTY, CI. Not
	// keyed on $DISPLAY/$WAYLAND_DISPLAY, which are X and Wayland: Aqua sets
	// neither, so bothy would never open a window on macOS.
	if !hasDisplay(p) {
		return launchMode{Reason: "no graphical display; running here"}
	}

	if _, err := terminalCommand(p, term); err != nil {
		return launchMode{Reason: err.Error() + "; running here"}
	}

	// Launched from a desktop icon or a script: there is no terminal to run
	// in, so one has to be opened whatever the graphics situation.
	if !isTerminal(os.Stdout) {
		return launchMode{Spawn: true, Reason: "started without a terminal"}
	}

	if g := probe.CheckGraphics(p.Terminal, probe.MuxGraphics{None: true}); !g.Supported {
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

// hasDisplay reports whether there is a session to open a window into. darwin
// and windows have a window server whenever anyone is logged in at all, so the
// question there is only whether this is a remote login -- $SSH_CONNECTION
// says so without the machinery of asking launchd or the session manager.
// Neither sets DISPLAY, so the X and Wayland test would answer no on every
// Mac and every Windows machine.
func hasDisplay(p platform.Info) bool {
	if p.OS == "darwin" || p.OS == "windows" {
		return os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == ""
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// terminalCommand returns how to run the configured terminal. Inside a
// container it is a host application, reached through flatpak-spawn --host.
func terminalCommand(p platform.Info, term string) ([]string, error) {
	sp, err := install.TerminalSpawn(term)
	if err != nil {
		return nil, err
	}
	bin := p.ExeName(sp.Binary)
	if path, err := exec.LookPath(bin); err == nil {
		return []string{path}, nil
	}
	// macOS installs applications as bundles and Ghostty's installer puts
	// nothing on PATH. The binary inside takes the same arguments, so this is
	// a path to find rather than a different way to launch.
	if p.OS == "darwin" {
		app := strings.ToUpper(sp.Binary[:1]) + sp.Binary[1:] + ".app"
		for _, dir := range []string{"/Applications", filepath.Join(p.Home, "Applications")} {
			inside := filepath.Join(dir, app, "Contents", "MacOS", sp.Binary)
			if fi, err := os.Stat(inside); err == nil && !fi.IsDir() {
				return []string{inside}, nil
			}
		}
	}
	if p.InContainer() {
		if fs, err := exec.LookPath("flatpak-spawn"); err == nil {
			if err := exec.Command(fs, "--host", "sh", "-c", "command -v "+sp.Binary).Run(); err == nil {
				return []string{fs, "--host", sp.Binary}, nil
			}
		}
	}
	return nil, fmt.Errorf("%s is not installed", sp.Binary)
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

// hostBothy is the path the host should run. Asking beats assuming: a dnf or
// deb install puts its copy in /usr/bin, nowhere under home. The fallback
// covers a host PATH omitting ~/.local/bin in a non-login shell, checkable
// from in here because home is shared.
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
func spawnTerminal(p platform.Info, termSlot, dir, profileName string) error {
	sp, err := install.TerminalSpawn(termSlot)
	if err != nil {
		return err
	}
	term, err := terminalCommand(p, termSlot)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find the bothy binary to run: %w", err)
	}
	// Inside a container the host's ghostty runs the host's copy of bothy,
	// which hops back in. The container's os.Executable() is not that copy:
	// home is shared, so the path may name a different file, or none.
	if len(term) > 1 && term[1] == "--host" {
		host, err := hostBothy(p)
		if err != nil {
			return err
		}
		self = host
	}

	conf, err := install.TerminalConf(p, termSlot)
	if err != nil {
		return err
	}
	// Checked rather than assumed: wezterm falls back to its built-in defaults
	// without a word when a config file is missing, so the theme would go
	// quietly rather than loudly.
	if _, err := os.Stat(conf); err != nil {
		return fmt.Errorf("%s does not exist — run 'bothy install' first", conf)
	}

	args := append([]string{}, term[1:]...)
	for _, a := range sp.Args {
		args = append(args, strings.ReplaceAll(a, "{config}", conf))
	}
	args = append(args, self, "--dir", dir, "--profile", profileName)

	cmd := exec.Command(term[0], args...)
	cmd.Env = append(os.Environ(), InTerminalEnv+"=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start, not Run: the shell that typed `bothy` gets its prompt back instead
	// of blocking on the window the user is now working in.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open a terminal: %w", err)
	}
	fmt.Printf("opened a %s window\n", sp.Binary)
	return cmd.Process.Release()
}
