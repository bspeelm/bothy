package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/mux"
)

// The half of `bothy tower` that starts nothing: which pane holds an agent,
// what a watching pane runs, and how the window is divided.
//
// Every pane mirrors a pane elsewhere by asking the multiplexer what it shows,
// so nothing attaches to a watched session and no second client resizes it.
// Nothing here may send anything to an agent (ADR-048).

// towerSession is the tower's own session name. Fixed rather than derived: a
// second tower would watch the first.
const towerSession = "bothy-tower"

// mirror is one agent being watched.
type mirror struct {
	Session string
	Pane    string
	Label   string
}

// agentPane picks out the pane an agent is running in, by command rather than
// position: the agent is terminal_1 only because of how the cockpit profile
// splits. An exited pane is skipped, since the session outlives the agent.
func agentPane(panes []mux.PaneRef, agentBin string) (mux.PaneRef, bool) {
	want := filepath.Base(agentBin)
	for _, p := range panes {
		if p.Plugin || p.Exited || p.Command == "" {
			continue
		}
		if filepath.Base(p.Command) == want {
			return p, true
		}
	}
	return mux.PaneRef{}, false
}

// label names a row. The working directory rather than the session name, which
// says the same thing twice for a local project.
func label(session, dir string) string {
	if dir != "" {
		return filepath.Base(dir)
	}
	return strings.TrimPrefix(session, "bothy-")
}

// mirrorCommand is what one tower pane runs: this binary, in the mode that
// prints another pane repeatedly. By absolute path, because the copy on PATH
// may not be the copy that built this window.
func mirrorCommand(self string, m mirror) string {
	return fmt.Sprintf("%s tower --mirror %s", self, m.Session)
}

// towerProfile stacks one mirror per row. Rows rather than columns: an agent
// pane is about 55 columns, and three side by side wrap into noise. Height is
// the cheap axis, because a screen printed into a shorter pane scrolls to its
// own bottom, which is the part worth seeing.
func towerProfile(mirrors []mirror, self string) layout.Profile {
	prof := layout.Profile{
		Name:        "tower",
		Description: "every agent at work, in one window",
	}
	for _, m := range mirrors {
		prof.Rows = append(prof.Rows, layout.Row{
			Panes: []layout.Pane{{Name: m.Label, Command: mirrorCommand(self, m)}},
		})
	}
	return prof
}

// clearScreen puts the cursor home and wipes what was there. The source is a
// whole screen every time, so reprinting beats diffing.
const clearScreen = "\033[H\033[2J"

// watchable is the sessions worth a row: running, holding an agent, and not the
// tower itself. It takes a lookup rather than a backend so the decision is
// testable without a multiplexer. Only live sessions are considered: nothing
// prunes the records under state/sessions (ADR-042), so ghosts are on disk.
func watchable(panesOf func(string) ([]mux.PaneRef, bool), agent string, live []string) []mirror {
	var out []mirror
	for _, s := range live {
		if s == towerSession {
			continue
		}
		panes, ok := panesOf(s)
		if !ok {
			continue
		}
		pane, found := agentPane(panes, agent)
		if !found {
			continue
		}
		out = append(out, mirror{Session: s, Pane: pane.Addr(), Label: label(s, pane.Dir)})
	}
	return out
}
