package main

import (
	"flag"
	"fmt"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/theme"
)

// `bothy layout` and `bothy theme` -- print what install would write.

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
	backend, _, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	out, err := backend.Preview(prof, install.Commands(cfg))
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// cmdTheme prints a blank palette. Filling in eleven colours and pointing bothy
// at the file is the whole mechanism for a custom palette; no theme is built in.
func cmdTheme(args []string) error {
	if len(args) == 0 || args[0] != "example" {
		return fmt.Errorf("usage: bothy theme example")
	}
	fmt.Print(theme.ExampleFile)
	return nil
}
