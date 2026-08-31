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
	"github.com/bothy-dev/bothy/internal/theme"
	"github.com/bothy-dev/bothy/internal/tools"
)

// Version is set at build time by the release process.
var Version = "dev"

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
  bothy uninstall [--dry-run]   put the machine back the way it was
  bothy version

Every generated file says it is bothy's and names where to put your own
changes. Nothing is written without first backing up what was there.
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
	offline := fs.Bool("offline", false, "do not fetch any tool; use only what is installed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}

	// Tools first: the graphics probe that decides how yazi.toml is written
	// asks the multiplexer its version, so a zellij fetched now is the one the
	// config is rendered for.
	if !*dryRun {
		treport, err := install.EnsureTools(p, cfg, *offline)
		if err != nil {
			return err
		}
		printTools(treport)
	}

	res, err := install.Run(p, cfg, install.Options{DryRun: *dryRun, Offline: *offline})
	if err != nil {
		return err
	}

	verb := "wrote"
	if *dryRun {
		verb = "would write"
	}
	fmt.Printf("%s %d file(s), %d already current, under %s\n",
		verb, len(res.Written), len(res.Unchanged), tilde(res.Root, p.Home))
	for _, f := range res.Written {
		fmt.Printf("  + %s\n", tilde(f, p.Home))
	}

	if pr := res.Plugins; pr != nil {
		for _, pl := range pr.Installed {
			fmt.Printf("  + yazi plugin %s — %s\n", pl.Name, pl.Gives)
		}
		for _, f := range pr.Failed {
			fmt.Printf("  ! yazi plugin %s unavailable (%v) — %s is off\n",
				f.Plugin.Name, f.Err, f.Plugin.Gives)
		}
	}

	fmt.Printf("\nimage previews: %v — %s\n", res.Data.ImagePreviews, res.Data.GraphicsReason)
	fmt.Println("nothing outside that directory was touched.")

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
	env := doctor.Env{
		Platform: p, Config: cfg, ProfileName: cfg.Profile,
		MuxBin: install.ToolPath(p, cfg.Slots.Mux),
		// Check the tools the way bothy will actually run them.
		ToolEnv: install.SessionEnv(p, cfg),
	}
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

	rep, err := install.Uninstall(p, *dryRun)
	if err != nil {
		return err
	}
	verb := "removed"
	if *dryRun {
		verb = "would remove"
	}
	for _, f := range rep.Removed {
		fmt.Printf("%s %s\n", verb, tilde(f, p.Home))
	}
	if len(rep.Removed) == 0 {
		fmt.Println("nothing to remove")
	}
	for _, f := range rep.Kept {
		fmt.Printf("  kept %s\n", tilde(f, p.Home))
	}
	// The desktop entry is the one thing bothy writes outside its tree, so it
	// is the one thing uninstall has to mention rather than silently leave.
	if entry := desktopEntryPath(p.DataDir); fileExists(entry) {
		fmt.Printf("  kept %s — remove with 'bothy desktop-entry --remove'\n", tilde(entry, p.Home))
	}
	return nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
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

// printTools reports the fill-gaps decisions. Saying *why* a tool was fetched
// matters more than saying that it was: "zellij 0.42.2 is below 0.45.1" is
// actionable, "downloading zellij" is noise.
func printTools(r *install.ToolReport) {
	if r.Skipped {
		fmt.Println("offline: using only the tools already installed")
	}
	for _, d := range r.Fetched {
		fmt.Printf("  ↓ %s — %s\n", d.Tool.Name, d.Reason)
	}
	for _, f := range r.Failed {
		fmt.Printf("  ! %s could not be supplied: %v\n", f.Decision.Tool.Name, f.Err)
	}
	if n := len(r.Used); n > 0 {
		fmt.Printf("  ✓ using %d tool(s) already on your system\n", n)
	}
	if len(r.Fetched) > 0 || len(r.Failed) > 0 {
		fmt.Println()
	}
}

// cmdTools shows where each tool comes from, without changing anything.
func cmdTools(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	names, err := tools.Required(cfg.Slots.Mux, cfg.Slots.Browser, cfg.Extras)
	if err != nil {
		return err
	}
	decisions, err := tools.ResolveAll(names)
	if err != nil {
		return err
	}
	for _, d := range decisions {
		mark := "✓"
		if d.Action == tools.Fetch {
			mark = "↓"
		}
		if own, ok := install.InstalledBinary(p, d.Tool.Binary); ok && d.Path == own {
			fmt.Printf("%s %-9s %-9s supplied by bothy\n", mark, d.Tool.Name, d.Version)
			continue
		}
		fmt.Printf("%s %-9s %s\n", mark, d.Tool.Name, d.Reason)
	}
	return nil
}
