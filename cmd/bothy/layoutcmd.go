package main

import (
	"flag"
	"fmt"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
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
