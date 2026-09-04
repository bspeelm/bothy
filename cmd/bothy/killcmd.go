package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
)

// cmdKill ends a session. The only other way is to attach and press Ctrl-q,
// which means entering a workspace in order to leave it -- and a session whose
// terminal is unreachable could not be ended at all.
func cmdKill(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	dir, _ := os.Getwd()
	env := install.SessionEnv(p, cfg)

	session, err := killPlan(backend, bin, env, dir, args)
	if err != nil {
		return err
	}
	if err := backend.Kill(bin, env, session); err != nil {
		return err
	}
	fmt.Printf("ended %s\n", session)
	return nil
}

// killPlan names the session to end, or says why there is none to end. Every
// refusal points at the command that does the thing being asked for.
func killPlan(backend mux.Backend, bin string, env []string, dir string, args []string) (string, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("kill takes one session name")
	}
	session := backend.SessionName(dir)
	if len(args) == 1 {
		session = args[0]
	}
	if session == "" {
		return "", fmt.Errorf("no session named, and this directory has none")
	}
	// Ending the session you are reading this in would take the terminal with
	// it, and the thing that does that deliberately is one keystroke away.
	if session == backend.CurrentSession() {
		return "", fmt.Errorf("%s is the session you are in\n"+
			"      press Ctrl-q to quit it from here", session)
	}
	if slices.Contains(backend.Stopped(bin, env), session) {
		return "", fmt.Errorf("%s has already stopped\n"+
			"      clear it with 'bothy ls --prune'", session)
	}
	if !slices.Contains(backend.Live(bin, env), session) {
		return "", fmt.Errorf("no session called %s\n"+
			"      'bothy ls' lists them", session)
	}
	return session, nil
}
