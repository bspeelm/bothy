package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
		MuxBin: install.ToolPath(p, cfg.Slots.Mux),
		RunsIn: hopTarget(p, cfg),
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
