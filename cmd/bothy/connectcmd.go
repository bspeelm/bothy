package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/advice"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/state"
)

// `bothy connect` -- open the workspace against another machine.
//
// The workspace runs here. sshfs mounts the far machine so yazi can read it,
// the shell pane is a login session on it, and the agent is a local process
// that can reach it the same way you can. Nothing is placed over there
// (ADR-046), and the only thing run over there is a shell you asked for.

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	dirFlag := fs.String("dir", "", "the directory on that machine (default: your home there)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	edit := len(rest) > 0 && rest[0] == "edit"
	if edit {
		rest = rest[1:]
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: bothy connect [edit] <host> [--dir path]")
	}
	name, err := hostArg(rest[0])
	if err != nil {
		return err
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}
	host, rec, known := install.RemoteFor(p, cfg, name)

	// sshfs before anything else: refusing early beats a half-built workspace,
	// and a machine without it is the common first-run case.
	sshfs, err := exec.LookPath("sshfs")
	if err != nil {
		return fmt.Errorf("sshfs is not installed, and it is what lets the file browser\n"+
			"      see %s. bothy does not install it:\n"+
			"        %s", host, sshfsCommand(p))
	}

	rec, err = settle(p, cfg, host, rec, *dirFlag, known, edit)
	if err != nil {
		return err
	}

	mount := mountPoint(p.CacheDir(), host)
	if err := os.MkdirAll(mount, 0o755); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	// A workspace that was killed never ran its unmount, so clear a leftover
	// before adding to it: sshfs stacks a second mount on the same directory
	// happily, and then neither can be released by name.
	if alreadyMounted(mount) {
		unmount(mount)
	}
	if out, err := exec.Command(sshfs, mountArgs(host, rec.Identity, mount)...).CombinedOutput(); err != nil {
		return fmt.Errorf("could not reach %s: %s", host, strings.TrimSpace(string(out)))
	}
	// Released however this returns. A workspace that exits leaving the far
	// machine mounted is a directory that looks local and is not.
	defer unmount(mount)

	if err := writeShim(p); err != nil {
		return err
	}
	fmt.Printf("%s mounted at %s\n", host, tilde(mount, p.Home))

	return launch(p, cfg, localPath(mount, rec.Dir), cfg.Profile, "", remoteOpts{
		session: sessionFor(host, rec.Dir),
		shell:   shellCommand(host, rec.Identity, rec.Dir),
		env:     connectEnv(host, rec.Dir, mount),
	})
}

// settle decides where the work lives on that machine, asking at most once.
//
// The default is your home over there, which only that machine can expand --
// so it is asked for rather than guessed, and the absolute answer is recorded.
// Everything else depends on the recorded path being one the far machine
// actually uses.
func settle(p platform.Info, cfg config.Config, host string, rec state.Remote,
	dirFlag string, known, edit bool) (state.Remote, error) {

	if dirFlag != "" {
		rec.Dir = dirFlag
	}
	if edit || !known || rec.Dir == "" {
		if _, declared := cfg.Remotes[host]; declared && !edit {
			return rec, nil // config said so; do not second-guess it
		}
		if rec.Dir == "" {
			home, err := remoteHome(host, rec.Identity)
			if err != nil {
				return rec, err
			}
			rec.Dir = home
		}
		if edit || !known {
			if answer := askLine(fmt.Sprintf("directory on %s [%s]: ", host, rec.Dir)); answer != "" {
				rec.Dir = answer
			}
			if id := askLine("ssh key, if that host needs one named [none]: "); id != "" {
				rec.Identity = id
			}
		}
		if err := install.RecordRemote(p, host, rec); err != nil {
			return rec, err
		}
	}
	return rec, nil
}

// remoteHome asks the machine what your home is there, because "~" is only
// meaningful to the shell that expands it and this one is the wrong shell.
// A question, not a change: it runs `printf` and leaves nothing behind.
var remoteHome = func(host, identity string) (string, error) {
	argv := []string{}
	if identity != "" {
		argv = append(argv, "-i", identity)
	}
	argv = append(argv, "--", host, `printf %s "$HOME"`)
	out, err := exec.Command("ssh", argv...).Output()
	home := strings.TrimSpace(string(out))
	if err != nil || home == "" {
		return "", fmt.Errorf("could not ask %s where your home is; name it with --dir", host)
	}
	return home, nil
}

// writeShim puts `on` in bothy's own bin, on this machine. The agent's paths
// and the far machine's do not match, and this is what spares it the
// arithmetic -- see ADR-046.
func writeShim(p platform.Info) error {
	if err := os.MkdirAll(p.BinDir(), 0o755); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return os.WriteFile(filepath.Join(p.BinDir(), "on"), []byte(shimScript()), 0o755)
}

// sshfsCommand is the install line for this machine, from the provider file
// rather than from a switch here: adding a distribution should need no Go.
func sshfsCommand(p platform.Info) string {
	a, err := advice.Get("sshfs")
	if err != nil {
		return "install sshfs with your package manager"
	}
	return a.Command(p)
}

// askLine reads one answer, or "" when there is nobody to ask.
func askLine(prompt string) string {
	if !isTerminal(os.Stdin) {
		return ""
	}
	fmt.Print(prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// unmount releases the mount, lazily only if it has to. Errors are not
// reported: this runs on the way out of a workspace, and a complaint about a
// mount arriving after the window has gone helps nobody.
func unmount(mount string) {
	if exec.Command("fusermount3", unmountArgs(mount)...).Run() == nil {
		return
	}
	_ = exec.Command("fusermount3", unmountLazyArgs(mount)...).Run()
}
