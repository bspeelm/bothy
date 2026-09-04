package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bspeelm/bothy/internal/fetch"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/state"
	"github.com/bspeelm/bothy/internal/tools"
)

// cmdTools shows where each tool comes from, without changing anything.
func cmdTools(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	names, err := tools.Required(cfg.Providers(), cfg.Extras)
	if err != nil {
		return err
	}
	decisions, err := tools.ResolveAll(names, p.BinDir())
	if err != nil {
		return err
	}
	// The decisions describe what the *system* has. Provenance for the rest
	// comes from the manifest, which is the only record of what bothy itself
	// installed and at which version.
	m, mErr := state.Load(p.StateDir())
	lock, lockErr := fetch.LoadLock()
	downloadOnly := false

	for _, d := range decisions {
		if d.Action == tools.UseSystem {
			fmt.Printf("✓ %-9s %-20s %s\n", d.Tool.Name, d.Tool.What, d.Reason)
			continue
		}
		have := ""
		if mErr == nil {
			have = install.SuppliedVersion(p, m, d.Tool.Name)
		}
		if have == "" {
			fmt.Printf("↓ %-9s %-20s %s\n", d.Tool.Name, d.Tool.What, d.Reason)
			continue
		}
		pinned, pin := "", ""
		if lockErr == nil {
			if entry, ok := lock.Get(d.Tool.Name); ok {
				pinned = entry.Version
				pin = "download"
				if entry.CrossChecked(p) {
					pin = "upstream"
				}
				downloadOnly = downloadOnly || pin == "download"
			}
		}
		if pinned != "" && pinned != have {
			fmt.Printf("↑ %-9s %-20s %-9s supplied by bothy, pinned at %s  pin: %s\n",
				d.Tool.Name, d.Tool.What, have, pinned, pin)
			continue
		}
		fmt.Printf("✓ %-9s %-20s %-9s supplied by bothy  pin: %s\n", d.Tool.Name, d.Tool.What, have, pin)
	}

	// Only when there is something to explain. "upstream" needs no gloss;
	// "download" is the one a reader should know the shape of.
	if downloadOnly {
		fmt.Print("\n" +
			"pin: upstream — the pinned checksum matched one the project published. That\n" +
			"  rules out the release changing after publication; not a bad release.\n" +
			"pin: download — nothing was published to compare with, so the pin is the hash\n" +
			"  of what bothy downloaded on the day it was pinned.\n")
	}
	return nil
}

// printTools reports the fill-gaps decisions, with the reason each tool was fetched.
func printTools(r *install.ToolReport) {
	if r.Skipped {
		fmt.Println("offline: using only the tools already installed")
	}
	for _, f := range r.Failed {
		fmt.Printf("  ! %s could not be supplied: %v\n", f.Decision.Tool.Name, f.Err)
	}
	if n := len(r.Used); n > 0 {
		fmt.Printf("  ✓ using %d tool(s) already on your system\n", n)
	}
	if n := len(r.Current); n > 0 {
		fmt.Printf("  ✓ %d tool(s) bothy supplied are at the pinned version\n", n)
	}
	if len(r.Fetched) > 0 || len(r.Failed) > 0 {
		fmt.Println()
	}
}

// pids renders a kill-able list.
func pids(rs []install.Running) string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, strconv.Itoa(r.PID))
	}
	return strings.Join(out, " ")
}
