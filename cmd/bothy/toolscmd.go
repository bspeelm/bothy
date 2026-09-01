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
	names, err := tools.Required(cfg.Slots.Mux, cfg.Slots.Browser, cfg.Extras)
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

	for _, d := range decisions {
		if d.Action == tools.UseSystem {
			fmt.Printf("✓ %-9s %s\n", d.Tool.Name, d.Reason)
			continue
		}
		have := ""
		if mErr == nil {
			have = install.SuppliedVersion(p, m, d.Tool.Name)
		}
		if have == "" {
			fmt.Printf("↓ %-9s %s\n", d.Tool.Name, d.Reason)
			continue
		}
		pinned := ""
		if lockErr == nil {
			if entry, ok := lock.Get(d.Tool.Name); ok {
				pinned = entry.Version
			}
		}
		if pinned != "" && pinned != have {
			fmt.Printf("↑ %-9s %-9s supplied by bothy, pinned at %s\n", d.Tool.Name, have, pinned)
			continue
		}
		fmt.Printf("✓ %-9s %-9s supplied by bothy\n", d.Tool.Name, have)
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
