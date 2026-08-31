package main

import (
	"flag"
	"fmt"

	"github.com/bspeelm/bothy/internal/install"
)

// `bothy uninstall` -- remove bothy's tree, which under ADR-009 is all of it.

func cmdUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report what would be removed")
	keepBinary := fs.Bool("keep-binary", false, "leave the bothy binary in place")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, _, err := load()
	if err != nil {
		return err
	}

	rep, err := install.Uninstall(p, *dryRun, *keepBinary)
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
		fmt.Println("nothing left to remove — bothy is not installed here")
	}
	for _, f := range rep.Kept {
		fmt.Printf("  kept %s\n", tilde(f, p.Home))
	}
	if len(rep.Orphaned) > 0 {
		fmt.Printf("\n%d process(es) are still running binaries this removed:\n", len(rep.Orphaned))
		for _, r := range rep.Orphaned {
			fmt.Printf("  pid %d  %s\n", r.PID, r.Cmdline)
		}
		fmt.Println("they keep working but cannot be reattached, and their memory is")
		fmt.Println("held until they exit. Close them, or: kill " + pids(rep.Orphaned))
	}
	// The desktop entry is the one thing bothy writes outside its tree, so it
	// is the one thing uninstall has to mention rather than silently leave.
	if entry := desktopEntryPath(p.DataDir); fileExists(entry) {
		fmt.Printf("  kept %s — remove with 'bothy desktop-entry --remove'\n", tilde(entry, p.Home))
	}
	return nil
}
