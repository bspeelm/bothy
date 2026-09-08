package main

import (
	"fmt"
	"io"
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

// mirror is one agent being watched. Cols is the width of the pane it watches,
// which is also the width of the mirror.
type mirror struct {
	Session string
	Pane    string
	Label   string
	Cols    int
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

// towerProfile lays the mirrors out to fit what they are being mirrored from.
//
// A mirror is exactly as wide as the pane it watches: dump-screen returns that
// pane's grid with the text already wrapped, so a 57-column agent pane stays 57
// columns however large the tower window is. Side by side is therefore the
// arrangement that uses the space -- three of them fit a 200-column window,
// where stacked they leave two thirds of it empty and fullscreen adds nothing.
// They stack only when the window is too narrow to hold them abreast, or when
// its width could not be measured.
func towerProfile(mirrors []mirror, self string, width int) layout.Profile {
	prof := layout.Profile{
		Name:        "tower",
		Description: "every agent at work, in one window",
	}
	if abreast(mirrors, width) {
		panes := make([]layout.Pane, 0, len(mirrors))
		for _, m := range mirrors {
			panes = append(panes, layout.Pane{Name: m.Label, Command: mirrorCommand(self, m)})
		}
		prof.Rows = []layout.Row{{Panes: panes}}
		return prof
	}
	for _, m := range mirrors {
		prof.Rows = append(prof.Rows, layout.Row{
			Panes: []layout.Pane{{Name: m.Label, Command: mirrorCommand(self, m)}},
		})
	}
	return prof
}

// abreast reports whether every mirror can sit side by side at the width it
// will be drawn at. Two columns per mirror are added for the pane frame; a
// window one column short wraps every line of every mirror, which is worse
// than stacking.
func abreast(mirrors []mirror, width int) bool {
	if width <= 0 {
		return false
	}
	total := 0
	for _, m := range mirrors {
		if m.Cols <= 0 {
			return false
		}
		total += m.Cols + 2
	}
	return total <= width
}

// paint writes a screen without scrolling.
//
// Homing the cursor and clearing each line as it is written, rather than
// printing the lines plainly: a plain print pushes the previous frame into
// scrollback, which reached 1,382 lines of history after a few minutes of
// watching. No newline follows the last line, so a screen that fits the pane
// never scrolls it at all.
func paint(w io.Writer, screen string) {
	fmt.Fprint(w, "\033[H")
	lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	for i, line := range lines {
		if i > 0 {
			fmt.Fprint(w, "\r\n")
		}
		fmt.Fprint(w, line, "\033[K")
	}
	fmt.Fprint(w, "\033[J")
}

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
		out = append(out, mirror{
			Session: s, Pane: pane.Addr(), Label: label(s, pane.Dir), Cols: pane.Cols,
		})
	}
	return out
}
