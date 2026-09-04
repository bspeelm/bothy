package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
)

// cmdLs lists the multiplexer sessions that are running. `zellij
// list-sessions` typed directly reports none of them: bothy's sessions live
// under its own cache directory, which only its environment names.
func cmdLs(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	prune := fs.Bool("prune", false, "delete the stopped sessions listed below")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("ls takes no arguments")
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return err
	}

	env := install.SessionEnv(p, cfg)
	stopped := backend.Stopped(bin, env)
	if *prune {
		return pruneSessions(backend, bin, env, stopped)
	}

	live := backend.Live(bin, env)
	if len(live) == 0 && len(stopped) == 0 {
		fmt.Println("no sessions running")
		return nil
	}

	dir, _ := os.Getwd()
	here := backend.SessionName(dir)
	current := backend.CurrentSession()
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

	// Stopped sessions are not junk: attaching brings the layout back as it
	// was. They are listed because nothing else lists them -- `zellij
	// list-sessions --short` hides the distinction, and typed outside bothy's
	// environment it finds none of them at all.
	if len(stopped) > 0 {
		fmt.Printf("\n%d stopped, kept so they can be resurrected:\n", len(stopped))
		for _, s := range stopped {
			fmt.Printf("  %s\n", s)
		}
		fmt.Println("Clear them with 'bothy ls --prune'.")
	}
	return nil
}

// pruneSessions deletes the stopped ones. A live session is refused by the
// multiplexer rather than killed, so a list that went stale between reading
// and acting costs nothing.
func pruneSessions(backend mux.Backend, bin string, env []string, stopped []string) error {
	if len(stopped) == 0 {
		fmt.Println("no stopped sessions")
		return nil
	}
	for _, s := range stopped {
		if err := backend.Discard(bin, env, s); err != nil {
			fmt.Printf("  ! %v\n", err)
			continue
		}
		fmt.Printf("  removed %s\n", s)
	}
	return nil
}
