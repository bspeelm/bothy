package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/doctor"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy doctor` -- run the checks and render them, for a person or for CI.

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
		MuxBin:      install.ToolPath(p, cfg.Slots.Mux),
		SessionName: sessionNameHere(),
		RunsIn:      hopTarget(p, cfg),
		Version:     version(),
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
		printCapabilities(rep, cfg)
	}

	if rep.Failed() {
		os.Exit(1)
	}
	return nil
}

// printCapabilities answers the question the check list only implies: what
// does this stack actually give you. ADR-017 names five things, and a
// capability nothing checks says so rather than being quietly counted as
// working -- unless nothing in the stack claims to contribute to it, which
// bothy can say outright.
func printCapabilities(rep doctor.Report, cfg config.Config) {
	delivers := rep.Delivers()
	supplied := doctor.Supplied(cfg)
	var yes, no, unchecked []string
	for _, c := range doctor.Capabilities {
		switch {
		case !supplied[c]:
			// Asked before the results: the graphics check reads the emulator
			// bothy runs in, not the one the config names, so it passes on a
			// stack with nothing to do the work.
			no = append(no, string(c))
		case delivers[c] == doctor.Pass:
			yes = append(yes, string(c))
		case delivers[c] == doctor.Fail, delivers[c] == doctor.Warn:
			no = append(no, string(c))
		default:
			unchecked = append(unchecked, string(c))
		}
	}
	fmt.Println()
	for _, line := range []struct {
		label string
		items []string
	}{
		{"this stack gives you", yes},
		{"it cannot give you", no},
		{"nothing verifies", unchecked},
	} {
		if len(line.items) > 0 {
			fmt.Printf("%-21s %s\n", line.label+":", strings.Join(line.items, ", "))
		}
	}
}

// sessionNameHere is what bothy would call this directory's session, for the
// check that asks whether the running one matches.
func sessionNameHere() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return sessionName(dir)
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
