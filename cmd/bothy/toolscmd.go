package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bspeelm/bothy/internal/install"
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
