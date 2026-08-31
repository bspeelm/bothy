// Command bothy sets up and launches a terminal workspace.
//
// Subcommand dispatch is a plain switch rather than a CLI framework. PLAN.md §8
// puts the dependency ceiling at go-toml plus the standard library, and the
// switch below is smaller than the flag definitions a framework would need.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
)

// Version is stamped by the release build's -ldflags.
//
// It must stay a plain constant string: `-X` silently does nothing to a
// variable initialised by a function call, so folding the fallback below into
// this declaration disables the stamping it was meant to complement. Found by
// checking that -X still worked, which it had quietly stopped doing.
var Version = "dev"

// version reports the build's version.
//
// A `go install` binary gets no ldflags, so without this it would report "dev"
// and every bug report from that install path would name a version that does
// not exist. Go embeds the module version in the build info of every binary,
// which covers exactly that case.
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
  bothy attach                  reattach to a running session
  bothy install [--dry-run]     write the configs, then check them
  bothy doctor  [--json]        report what is broken and how to fix it
  bothy config  [get|set|edit|path]
  bothy layout  [--profile P]   print the layout that would be launched
  bothy theme   example         print a blank palette file to fill in
  bothy tools                   show which tools are used and where they came from
  bothy desktop-entry           print a .desktop launcher (--install to write it)
  bothy uninstall [--dry-run]   remove bothy's directory and its binary
  bothy lock    [--tool T]      re-pin the tools in bothy.lock (maintainers)
  bothy version

Every generated file says it is bothy's and names where to put your own
changes. Everything bothy writes lives under ~/.local/share/bothy, and
'bothy uninstall' removes that one directory.
`

func main() {
	// Bare `bothy` launches the workspace. That is the command people type
	// every day, so it is the one that costs nothing to type; `bothy help`
	// is there for everything else.
	if len(os.Args) < 2 {
		if err := cmdDev(nil); err != nil {
			fmt.Fprintf(os.Stderr, "bothy: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "install":
		err = cmdInstall(args)
	case "doctor":
		err = cmdDoctor(args)
	case "dev":
		// Retained so `bothy dev` keeps working; bare `bothy` is the same thing.
		err = cmdDev(args)
	case "attach":
		err = cmdAttach(args)
	case "config":
		err = cmdConfig(args)
	case "layout":
		err = cmdLayout(args)
	case "theme":
		err = cmdTheme(args)
	case "lock":
		err = cmdLock(args)
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
		fmt.Fprintf(os.Stderr, "bothy: %v\n", err)
		os.Exit(1)
	}
}

// load is the common preamble: detect the machine, read the config.
func load() (platform.Info, config.Config, error) {
	p := platform.Detect()
	cfg, err := config.Load(p)
	return p, cfg, err
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
