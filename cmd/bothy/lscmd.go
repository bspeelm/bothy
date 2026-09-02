package main

import (
	"fmt"
	"os"

	"github.com/bspeelm/bothy/internal/install"
)

// cmdLs lists the multiplexer sessions that are running. `zellij
// list-sessions` typed directly reports none of them: bothy's sessions live
// under its own cache directory, which only its environment names.
func cmdLs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("ls takes no arguments")
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}
	mux := cfg.Slots.Mux
	if mux == "" {
		mux = "zellij"
	}
	bin, err := lookPathIn(p, mux)
	if err != nil {
		return fmt.Errorf("%s is not installed — run 'bothy install'", mux)
	}

	live := liveSessions(bin, install.SessionEnv(p, cfg))
	if len(live) == 0 {
		fmt.Println("no sessions running")
		return nil
	}

	dir, _ := os.Getwd()
	here := sessionName(dir)
	current := os.Getenv("ZELLIJ_SESSION_NAME")
	for _, s := range live {
		switch {
		case s == current:
			fmt.Printf("  %-28s the one you are in\n", s)
		case s == here:
			fmt.Printf("  %-28s this directory\n", s)
		default:
			fmt.Printf("  %s\n", s)
		}
	}
	return nil
}
