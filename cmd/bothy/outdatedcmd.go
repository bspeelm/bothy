package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bspeelm/bothy/internal/fetch"
	"github.com/bspeelm/bothy/internal/tools"
)

// `bothy outdated` -- which pinned tools have newer releases upstream.

func cmdOutdated(args []string) error {
	fs := flag.NewFlagSet("outdated", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ts, err := tools.Load()
	if err != nil {
		return err
	}
	lock, err := fetch.LoadLock()
	if err != nil {
		return err
	}
	updates := fetch.CheckOutdated(ts, lock)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Updates []fetch.Update `json:"updates"`
		}{updates})
	}

	var stale, unknown int
	for _, u := range updates {
		switch {
		case u.Outdated():
			stale++
			fmt.Printf("  %-9s %-9s -> %s\n", u.Name, u.Pinned, u.Latest)
		case u.Reason != "":
			unknown++
		}
	}
	for _, u := range updates {
		if u.Reason != "" {
			fmt.Printf("  %-9s %-9s ?  %s\n", u.Name, u.Pinned, u.Reason)
		}
	}

	fmt.Println()
	switch {
	case stale == 0 && unknown == 0:
		fmt.Printf("all %d tools are at their latest release.\n", len(updates))
	case stale == 0:
		fmt.Printf("no updates found, but %d could not be checked.\n", unknown)
	default:
		fmt.Printf("%d of %d tools have newer releases.\n", stale, len(updates))
		fmt.Println("Run 'bothy lock' to take them, then review the diff.")
	}

	// Being out of date is a fact, not a failure: exiting non-zero here would
	// make every scheduled run look broken.
	return nil
}
