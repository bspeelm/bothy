// Command bothy sets up and launches a terminal workspace. Dispatch is a
// plain switch; PLAN.md §8 caps dependencies at go-toml.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
)

// Version is stamped by the release build's -ldflags, and must stay a plain
// string literal: `-X` silently does nothing to a variable initialised by a
// function call.
var Version = "dev"

// version reports the build's version. A `go install` binary gets no ldflags,
// so it falls back to the module version Go embeds in build info.
func version() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return Version
}

const usage = `bothy — a small, unlocked terminal workspace

Usage:
  bothy                         launch the workspace
  bothy attach [session]        reattach to this project's session
  bothy ls [--prune]            which sessions are running, and which stopped
  bothy kill [session]          end a session without attaching to it
  bothy box     [ls|use|stop|create|rm]  the toolbox this project uses, and the rest
  bothy connect [edit] <host>   open the workspace against another machine
  bothy keys                    the bindings worth knowing
  bothy confine                 run the agent walled off from the rest of $HOME
  bothy install [--dry-run]     write the configs, then check them
  bothy doctor  [--json]        report what is broken and how to fix it
  bothy config  [get|set|edit|path]
  bothy layout  [--profile P]   print the layout that would be launched
  bothy theme   example         print a blank palette file to fill in
  bothy tools                   show which tools are used and where they came from
  bothy tower                   watch every running agent in one window
  bothy completion <bash|zsh>   the completion script (--install to write it)
  bothy desktop-entry           print a .desktop launcher (--install to write it)
  bothy uninstall [--dry-run]   remove bothy's directory and its binary
  bothy upgrade                 how to upgrade this copy of bothy
  bothy outdated [--json]       which pinned tools have newer releases
  bothy version

Every generated file says it is bothy's and names where to put your own
changes. The workspace lives under ~/.local/share/bothy and your settings
under ~/.config/bothy; 'bothy uninstall' removes the first and names the rest.
`

// isHelpFlag names the flags main answers itself rather than handing on.
func isHelpFlag(s string) bool {
	switch s {
	case "--version", "-v", "--help", "-h":
		return true
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		if err := cmdDev(nil); err != nil {
			fail(err)
		}
		return
	}

	// A leading flag means the launcher, so `bothy --in-place` works. Without
	// this the flags could only be reached through `bothy dev`, which is why
	// that alias outlived the shell function it was kept for.
	if strings.HasPrefix(os.Args[1], "-") && !isHelpFlag(os.Args[1]) {
		if err := cmdDev(os.Args[1:]); err != nil {
			fail(err)
		}
		return
	}

	cmd, args := os.Args[1], os.Args[2:]
	if lines, ok := helpFor(cmd, args); ok {
		fmt.Print(lines)
		return
	}
	var err error
	switch cmd {
	case "install":
		err = cmdInstall(args)
	case "doctor":
		err = cmdDoctor(args)
	case "attach":
		err = cmdAttach(args)
	case "ls":
		err = cmdLs(args)
	case "box":
		err = cmdBox(args)
	case "connect":
		err = cmdConnect(args)
	case "keys":
		err = cmdKeys(args)
	case "confine":
		err = cmdConfine(args)
	case "config":
		err = cmdConfig(args)
	case "layout":
		err = cmdLayout(args)
	case "theme":
		err = cmdTheme(args)
	case "upgrade":
		err = cmdUpgrade(args)
	case "outdated":
		err = cmdOutdated(args)
	case "kill":
		err = cmdKill(args)
	case "lock":
		err = cmdLock(args)
	case "completion":
		err = cmdCompletion(args)
	case "tower":
		err = cmdTower(args)
	case "tools":
		err = cmdTools(args)
	case "desktop-entry":
		err = cmdDesktop(args)
	case "uninstall":
		err = cmdUninstall(args)
	case "version", "--version", "-v":
		fmt.Printf("bothy %s\n", version())
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "bothy: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		fail(err)
	}
}

// parsesFlags are the commands whose flag set answers -h itself, listing the
// flags as well. The rest take positional words, and without this a --help
// is taken for one: kill looks for a session called "--help", attach hands it
// to zellij.
var parsesFlags = map[string]bool{
	"completion": true, "confine": true, "connect": true, "desktop-entry": true,
	"doctor": true, "install": true, "layout": true, "lock": true, "ls": true,
	"outdated": true, "tower": true, "uninstall": true, "upgrade": true,
}

// helpFor answers -h or --help for a command that takes only positional words.
func helpFor(cmd string, args []string) (string, bool) {
	if len(args) == 0 || (args[0] != "-h" && args[0] != "--help") || parsesFlags[cmd] {
		return "", false
	}
	lines := commandUsage(cmd)
	return lines, lines != ""
}

// commandUsage is cmd's lines from the usage text, "" for a command it does
// not list.
func commandUsage(cmd string) string {
	var out strings.Builder
	for _, line := range strings.Split(usage, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "bothy" && f[1] == cmd {
			out.WriteString(line + "\n")
		}
	}
	return out.String()
}

// fail exits for err. A child's exit status passes through as the child's own
// code with nothing printed: the child has already said what it had to.
func fail(err error) {
	var status mux.ExitStatus
	if errors.As(err, &status) {
		os.Exit(status.Code)
	}
	fmt.Fprintf(os.Stderr, "bothy: %v\n", err)
	os.Exit(1)
}

func load() (platform.Info, config.Config, error) {
	p := platform.Detect()
	cfg, err := config.Load(p)
	warnUnknownKeys(cfg)
	return p, cfg, err
}

// warnUnknownKeys reports unrecognised config keys once, on stderr so it
// cannot corrupt --json output or a piped layout.
func warnUnknownKeys(cfg config.Config) {
	for _, k := range cfg.Unreadable {
		fmt.Fprintf(os.Stderr, "bothy: config.toml: %s has a value bothy cannot read, "+
			"so it is being ignored; 'bothy config set %s ...' rewrites the file\n", k, k)
	}
	for _, k := range cfg.Unknown {
		if near := config.Nearest(k); near != "" {
			fmt.Fprintf(os.Stderr, "bothy: config.toml: unknown key %q — did you mean %q?\n", k, near)
			continue
		}
		fmt.Fprintf(os.Stderr, "bothy: config.toml: unknown key %q\n", k)
	}
}

func cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report what would change without writing anything")
	offline := fs.Bool("offline", false, "do not fetch any tool; use only what is installed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}
	if err := setup(p, cfg, setupOptions{DryRun: *dryRun, Offline: *offline}); err != nil {
		return err
	}
	if *dryRun {
		return nil
	}
	fmt.Println("\nchecking:")
	return runDoctor(p, cfg, false)
}
