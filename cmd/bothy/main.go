// Command bothy sets up and launches a terminal workspace.
//
// Subcommand dispatch is a plain switch rather than a CLI framework. PLAN.md §8
// puts the dependency ceiling at go-toml plus the standard library, and the
// switch below is smaller than the flag definitions a framework would need.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/doctor"
	"github.com/bothy-dev/bothy/internal/install"
	"github.com/bothy-dev/bothy/internal/layout"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/state"
	"github.com/bothy-dev/bothy/internal/theme"
)

// Version is set at build time by the release process.
var Version = "dev"

const usage = `bothy — a small, unlocked terminal workspace

Usage:
  bothy install [--dry-run]     install tools, write configs, then check them
  bothy doctor  [--json]        report what is broken and how to fix it
  bothy dev     [--dir DIR]     launch the workspace  (usually run as: dev)
  bothy config  [get|set|edit|path]
  bothy layout  [--profile P]   print the layout that would be launched
  bothy theme   example         print a blank palette file to fill in
  bothy uninstall [--dry-run]   put the machine back the way it was
  bothy version

Every generated file says it is bothy's and names where to put your own
changes. Nothing is written without first backing up what was there.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "install":
		err = cmdInstall(args)
	case "doctor":
		err = cmdDoctor(args)
	case "dev":
		err = cmdDev(args)
	case "config":
		err = cmdConfig(args)
	case "layout":
		err = cmdLayout(args)
	case "theme":
		err = cmdTheme(args)
	case "uninstall":
		err = cmdUninstall(args)
	case "version", "--version", "-v":
		fmt.Printf("bothy %s\n", Version)
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
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}

	res, err := install.Run(p, cfg, install.Options{DryRun: *dryRun})
	if err != nil {
		return err
	}

	verb := "wrote"
	if *dryRun {
		verb = "would write"
	}
	fmt.Printf("%s %d file(s), %d already current\n", verb, len(res.Written), len(res.Unchanged))
	for _, f := range res.Written {
		fmt.Printf("  + %s\n", tilde(f, p.Home))
	}
	if len(res.Skipped) > 0 {
		fmt.Printf("\nleft alone (edited by hand since bothy wrote them):\n")
		for _, f := range res.Skipped {
			fmt.Printf("  ! %s\n", tilde(f, p.Home))
		}
		fmt.Println("  move your changes into ~/.config/bothy/overrides/ to keep them across installs")
	}

	// delta as git's pager is part of the setup being ported. Previous values
	// are recorded first so uninstall can put them back — including putting a
	// key back to unset, which is not the same as putting it back to empty.
	if contains(cfg.Extras, "delta") {
		m, err := state.Load(p.StateDir)
		if err != nil {
			return err
		}
		if err := install.ApplyGitSettings(m, *dryRun); err != nil {
			return err
		}
		if !*dryRun {
			if err := m.Save(p.StateDir); err != nil {
				return err
			}
		}
	}

	fmt.Printf("\nimage previews: %v — %s\n", res.Data.ImagePreviews, res.Data.GraphicsReason)

	if *dryRun {
		return nil
	}
	fmt.Println("\nchecking:")
	return runDoctor(p, cfg, false)
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}
	return runDoctor(p, cfg, *asJSON)
}

func runDoctor(p platform.Info, cfg config.Config, asJSON bool) error {
	env := doctor.Env{Platform: p, Config: cfg, ProfileName: cfg.Profile}
	if prof, err := install.LoadProfile(p, cfg.Profile); err == nil {
		env.PaneCount = prof.PaneCount()
	}

	rep := doctor.Run(env)

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		for _, r := range rep.Results {
			if r.Severity == doctor.Skip {
				continue
			}
			fmt.Printf("%s %s\n", mark(r.Severity), r.Summary)
			if r.Severity == doctor.Pass {
				continue
			}
			if r.Detail != "" {
				fmt.Printf("    %s\n", r.Detail)
			}
			if r.Fix != "" {
				fmt.Printf("    fix: %s\n", r.Fix)
			}
		}
		pass, warn, fail, skip := rep.Counts()
		fmt.Printf("\n%d passed, %d warning(s), %d failure(s), %d not applicable\n",
			pass, warn, fail, skip)
	}

	if rep.Failed() {
		os.Exit(1)
	}
	return nil
}

func mark(s doctor.Severity) string {
	switch s {
	case doctor.Pass:
		return "✓"
	case doctor.Warn:
		return "!"
	case doctor.Fail:
		return "✗"
	}
	return "-"
}

func cmdLayout(args []string) error {
	fs := flag.NewFlagSet("layout", flag.ExitOnError)
	profile := fs.String("profile", "", "profile name (default: the configured one)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}
	name := *profile
	if name == "" {
		name = cfg.Profile
	}
	prof, err := install.LoadProfile(p, name)
	if err != nil {
		return err
	}
	kdl, err := layout.Render(prof, install.Commands(cfg))
	if err != nil {
		return err
	}
	fmt.Print(kdl)
	return nil
}

// cmdTheme prints a blank palette. This is the whole mechanism for using a
// palette other than the built-in one: fill in eleven colours, point bothy at
// the file. Nothing about any particular theme is built in, and a palette you
// have licensed never leaves your machine.
func cmdTheme(args []string) error {
	if len(args) == 0 || args[0] != "example" {
		return fmt.Errorf("usage: bothy theme example")
	}
	fmt.Print(theme.ExampleFile)
	return nil
}

func cmdConfig(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"get"}
	}

	switch args[0] {
	case "path":
		fmt.Println(config.Path(p))
	case "get":
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	case "set":
		if len(args) != 3 {
			return fmt.Errorf("usage: bothy config set <key> <value>")
		}
		if err := cfg.Set(args[1], args[2]); err != nil {
			return err
		}
		if err := config.Save(p, cfg); err != nil {
			return err
		}
		fmt.Printf("set %s = %s\n", args[1], args[2])
		if err := cfg.Incomplete(); err != nil {
			fmt.Printf("\nnot ready yet:\n%v\n", err)
			return nil
		}
		fmt.Println("run 'bothy install' to apply it")
	case "edit":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		if err := config.Save(p, cfg); err != nil { // ensure the file exists
			return err
		}
		return runInteractive(editor, config.Path(p))
	default:
		return fmt.Errorf("usage: bothy config [get|set|edit|path]")
	}
	return nil
}

func cmdUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report what would be removed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, _, err := load()
	if err != nil {
		return err
	}
	m, err := state.Load(p.StateDir)
	if err != nil {
		return err
	}
	if len(m.Files) == 0 && len(m.Binaries) == 0 {
		fmt.Println("nothing recorded — bothy has not installed anything here")
		return nil
	}

	rep, err := install.Uninstall(p, m, *dryRun)
	if err != nil {
		return err
	}
	verb := "removed"
	if *dryRun {
		verb = "would remove"
	}
	fmt.Printf("%s %d file(s), restored %d backup(s)\n", verb, len(rep.Removed), len(rep.Restored))
	for _, f := range rep.Removed {
		fmt.Printf("  - %s\n", tilde(f, p.Home))
	}
	for _, f := range rep.Restored {
		fmt.Printf("  ↩ %s\n", tilde(f, p.Home))
	}
	if len(rep.Kept) > 0 {
		fmt.Println("\nkept (edited since bothy wrote them):")
		for _, f := range rep.Kept {
			fmt.Printf("  ! %s\n", tilde(f, p.Home))
		}
	}
	return nil
}

func tilde(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func expandDir(dir, home string) string {
	if dir == "~" {
		return home
	}
	if strings.HasPrefix(dir, "~/") {
		return filepath.Join(home, dir[2:])
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// newFlagSet keeps flag handling uniform across subcommands.
func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ExitOnError)
}

// contains reports whether a slice holds a value.
func contains(hay []string, needle string) bool {
	for _, v := range hay {
		if v == needle {
			return true
		}
	}
	return false
}
