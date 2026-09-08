package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy tower` -- the effects half: ask the multiplexer what is running, and
// open one window that watches all of it.

func cmdTower(args []string) error {
	fs := flag.NewFlagSet("tower", flag.ExitOnError)
	one := fs.String("mirror", "", "watch one session's agent pane and nothing else")
	every := fs.Duration("every", 2*time.Second, "how often to refresh")
	if err := fs.Parse(args); err != nil {
		return err
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

	if *one != "" {
		return runMirror(backend, bin, env, cfg, *one, *every)
	}
	return openTower(p, cfg, backend, bin, env)
}

// runMirror prints one session's agent pane until the window closes. The pane
// is looked up every pass, not once: an agent restarted lands in a new pane.
func runMirror(backend mux.Backend, bin string, env []string, cfg config.Config,
	session string, every time.Duration) error {

	agent := install.AgentBinary(cfg.Slots.Agent)
	for {
		screen, err := mirrorOnce(backend, bin, env, session, agent)
		paint(os.Stdout, screen)
		if err != nil {
			// Reported in the pane, not returned: one session ending must not
			// close a pane the other mirrors share a window with.
			fmt.Printf("\n%s: %v\n", session, err)
		}
		time.Sleep(every)
	}
}

// mirrorOnce is one refresh: find the agent's pane, read it.
func mirrorOnce(backend mux.Backend, bin string, env []string, session, agent string) (string, error) {
	panes, ok := backend.PanesOf(bin, session, env)
	if !ok {
		return "", fmt.Errorf("gone, or the multiplexer stopped answering")
	}
	pane, found := agentPane(panes, agent)
	if !found {
		return "", fmt.Errorf("no %s pane in this session", agent)
	}
	return backend.Screen(bin, session, pane.Addr(), env)
}

// openTower builds the window and launches it.
func openTower(p platform.Info, cfg config.Config, backend mux.Backend, bin string, env []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("tower: cannot find this binary to run it again: %w", err)
	}
	live := backend.Live(bin, env)
	panesOf := func(session string) ([]mux.PaneRef, bool) {
		return backend.PanesOf(bin, session, env)
	}
	mirrors := watchable(panesOf, install.AgentBinary(cfg.Slots.Agent), live)
	if len(mirrors) == 0 {
		return fmt.Errorf("no sessions with an agent to watch\n" +
			"      the tower shows agent panes of running sessions; 'bothy ls' lists them")
	}
	prof := towerProfile(mirrors, self, envInt(env, "COLUMNS"))
	// Which arrangement it chose. A mirror cannot be made wider than the pane
	// it watches, so stacking is what a window too narrow to hold them all
	// looks like, and saying so beats leaving it to be guessed at.
	shape := "stacked"
	if len(prof.Rows) == 1 {
		shape = "side by side"
	}
	fmt.Printf("watching %d agent(s), %s\n", len(mirrors), shape)
	return backend.Open(mux.Request{
		Platform: p, Bin: bin, Session: towerSession, Dir: p.Home,
		Profile: prof, Commands: install.Commands(cfg), Env: env, Live: live,
	})
}

// envInt reads a numeric variable out of the session environment, 0 when it is
// absent. COLUMNS is unset rather than guessed when nothing could be measured
// (ADR-022's neighbour in SessionEnv), and 0 is what the layout treats as
// "do not assume it fits".
func envInt(env []string, key string) int {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}
