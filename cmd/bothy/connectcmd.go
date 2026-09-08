package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
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

// mounted is platform.Mounted behind a seam, so a test can say what is mounted
// without one.
var mounted = platform.Mounted

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	dirFlag := fs.String("dir", "", "the directory on that machine (default: /, the whole machine)")
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
	if mounted(mount) {
		unmount(p.OS, mount)
	}
	// Straight through rather than captured: ssh asks about an unknown host key
	// and asks for a password on the terminal, and a prompt nobody can see is
	// the difference between a question and a hang. Its own message is also
	// better than any paraphrase of it.
	user := sshUser(host)
	fmt.Printf("connecting to %s as %s\n", host, orLocalUser(user))
	m := exec.Command(sshfs, mountArgs(host, rec.Identity, mount)...)
	m.Stdin, m.Stdout, m.Stderr = os.Stdin, os.Stdout, os.Stderr
	// sshfs forks and the parent exits 0 even when the child failed to mount,
	// so the mount point is the only honest answer about whether this worked.
	if err := m.Run(); err != nil || !mounted(mount) {
		return fmt.Errorf("could not mount %s -- sshfs said why above\n"+
			"      it tried to log in as %s. If the account there is different:\n"+
			"        bothy connect <account>@%s\n"+
			"      if it needs a particular key:  bothy connect edit %s",
			host, orLocalUser(user), host, host)
	}
	// Released however this returns. A workspace that exits leaving the far
	// machine mounted is a directory that looks local and is not.
	defer unmount(p.OS, mount)

	if err := writeShim(p); err != nil {
		return err
	}
	fmt.Printf("%s mounted at %s\n", host, tilde(mount, p.Home))

	defer watching(p, cfg, sessionFor(host, rec.Dir))()
	return launch(p, cfg, localPath(mount, rec.Dir), cfg.Profile,
		agentWithNote(cfg, host, rec.Dir, mount), remoteOpts{
			session: sessionFor(host, rec.Dir),
			shell:   shellCommand(host, rec.Identity, rec.Dir),
			env:     connectEnv(host, rec.Dir, mount),
		})
}

// settle decides where on that machine the work lives, asking at most once.
//
// The default is the whole machine. A server's home directory usually holds
// nothing but dotfiles, so landing there looks like a connection that did not
// work -- measured on a real host, ten entries and every one of them hidden.
// The root always has something in it, and anywhere below it is a few
// keystrokes away in the browser.
func settle(p platform.Info, cfg config.Config, host string, rec state.Remote,
	dirFlag string, known, edit bool) (state.Remote, error) {

	if dirFlag != "" {
		rec.Dir = dirFlag
	}
	if !edit && known && rec.Dir != "" {
		return rec, nil
	}
	if _, declared := cfg.Remotes[host]; declared && !edit {
		return rec, nil // config said so; do not second-guess it
	}
	if rec.Dir == "" {
		rec.Dir = "/"
	}
	if edit || !known {
		if answer := askLine(fmt.Sprintf("directory on %s [%s]: ", host, rec.Dir)); answer != "" {
			rec.Dir = answer
		}
		if id := askLine("ssh key, if that host needs one named [none]: "); id != "" {
			rec.Identity = id
		}
	}
	dir, err := expandRemote(host, rec.Identity, rec.Dir)
	if err != nil {
		return rec, err
	}
	rec.Dir = dir
	return rec, install.RecordRemote(p, host, rec)
}

// expandRemote resolves a leading ~ by asking the machine it belongs to.
//
// Only that shell can expand it, and this is the wrong shell -- so a "~" typed
// here is a question for over there rather than a guess made with this
// machine's home. Nothing else needs asking, which is why the default of "/"
// costs no round trip at all.
func expandRemote(host, identity, dir string) (string, error) {
	if dir != "~" && !strings.HasPrefix(dir, "~/") {
		return dir, nil
	}
	home, err := remoteHome(host, identity)
	if err != nil {
		return "", err
	}
	return path.Join(home, strings.TrimPrefix(strings.TrimPrefix(dir, "~"), "/")), nil
}

// remoteHome asks the machine what your home is there. A question, not a
// change: it runs printf and leaves nothing behind. A package variable so a
// test can answer without a machine to reach.
var remoteHome = func(host, identity string) (string, error) {
	argv := []string{}
	if identity != "" {
		argv = append(argv, "-i", identity)
	}
	argv = append(argv, "--", host, `printf %s "$HOME"`)
	out, err := exec.Command("ssh", argv...).Output()
	home := strings.TrimSpace(string(out))
	if err != nil || home == "" {
		return "", fmt.Errorf("could not ask %s where your home is; name the directory in full", host)
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
func unmount(goos, mount string) {
	plain, forced := unmountArgv(goos, mount)
	if exec.Command(plain[0], plain[1:]...).Run() == nil {
		return
	}
	_ = exec.Command(forced[0], forced[1:]...).Run()
}

// orLocalUser names the account for a message, falling back to this machine's
// when ssh declines to say -- which is also the account it would use.
func orLocalUser(user string) string {
	if user != "" {
		return user
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "your local username"
}
