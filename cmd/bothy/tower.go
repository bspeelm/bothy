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
// which is also the width of the mirror; Fullscreen is whether that pane
// already fills its tab.
type mirror struct {
	Session    string
	Pane       string
	Label      string
	Cols       int
	Fullscreen bool
}

// agentPaneName is what the profiles call the agent's pane, and what the
// multiplexer reports as its title.
const agentPaneName = "agent"

// agentPane picks out the pane an agent is running in, by command rather than
// position: the agent is terminal_1 only because of how the cockpit profile
// splits. An exited pane is skipped, since the session outlives the agent.
//
// The title is the fallback, because the command is not always the agent's.
// `bothy confine` runs it inside a container, so the pane reports podman and a
// confined agent was invisible here; the pane is still named agent, which is
// true of any wrapper rather than of that one.
func agentPane(panes []mux.PaneRef, agentBin string) (mux.PaneRef, bool) {
	want := filepath.Base(agentBin)
	var titled mux.PaneRef
	found := false
	for _, p := range panes {
		if p.Plugin || p.Exited {
			continue
		}
		if p.Command != "" && filepath.Base(p.Command) == want {
			return p, true
		}
		if p.Title == agentPaneName {
			titled, found = p, true
		}
	}
	return titled, found
}

// label names a row, from the working directory rather than the session name.
func label(session, dir string) string {
	if dir != "" {
		return filepath.Base(dir)
	}
	return strings.TrimPrefix(session, "bothy-")
}

// mirrorCommand is what one tower pane runs: this binary, in the mode that
// prints another pane repeatedly. By absolute path, because the copy on PATH
// may not be the one that built this window.
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

// replyRows is how much of the pane the mirror keeps its hands off: a blank
// line and the line being typed on.
const replyRows = 2

// paint writes a screen into the top of the pane and leaves the bottom alone.
//
// The bottom rows carry the reply being typed, so the cursor is saved and put
// back, rows are cleared one at a time rather than to the end of the screen,
// and nothing is written below them. Reaching them ate keystrokes mid-word.
//
// The bottom of the screen is what is kept when it does not fit: that is where
// an agent says what it is waiting for.
func paint(w io.Writer, screen string, rows int) {
	lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	area := len(lines)
	if rows > replyRows+1 {
		area = rows - replyRows
	}
	if len(lines) > area {
		lines = lines[len(lines)-area:]
	}
	fmt.Fprint(w, "\0337\033[H")
	for i := 0; i < area; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		if i > 0 {
			fmt.Fprint(w, "\r\n")
		}
		fmt.Fprint(w, line, "\033[K")
	}
	fmt.Fprint(w, "\0338")
}

// continues reports whether a line asks for the next one, and returns it with
// the continuation backslash removed. A trailing backslash is how a message
// spans lines, because the terminal cannot offer anything better: the tower
// reads stdin in canonical mode, where the line discipline ends a line on CR or
// LF, so Shift+Enter cannot mean "insert a newline and keep reading" whatever
// the terminal sends for it.
//
// Backslashes pair off, so a message that has to end in one doubles it. Odd
// means the last is the marker; even means they are all literal.
func continues(line string) (string, bool) {
	n := 0
	for n < len(line) && line[len(line)-1-n] == '\\' {
		n++
	}
	if n == 0 {
		return line, false
	}
	return line[:len(line)-n] + strings.Repeat(`\`, n/2), n%2 == 1
}

// replyPrompt puts the cursor on the reply line and marks it, so there is
// somewhere obvious to type and the mirror above never reaches it.
func replyPrompt(w io.Writer, rows int) {
	if rows <= replyRows {
		return
	}
	fmt.Fprintf(w, "\033[%d;1H\033[K> ", rows)
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
			Session: s, Pane: pane.Addr(), Label: label(s, pane.Dir),
			Cols: pane.Cols, Fullscreen: pane.Fullscreen,
		})
	}
	return out
}

// expandable is the mirrors whose pane does not already fill its tab.
//
// A pane sharing its window with a browser and a shell holds a quarter of what
// it holds alone -- 57x23 against 191x46, measured -- and a pane shows exactly
// what can be read out of it. Already-expanded panes are excluded: Expand
// toggles.
func expandable(mirrors []mirror) []mirror {
	var out []mirror
	for _, m := range mirrors {
		if !m.Fullscreen {
			out = append(out, m)
		}
	}
	return out
}

// collapsible is what to put back when the tower closes: panes the tower
// expanded that are still expanded now.
//
// Read again rather than remembered: fullscreen is Ctrl+P then f, so a pane can
// be collapsed by hand while the tower runs, and toggling it again on the way
// out would expand it.
func collapsible(expanded []mirror, panesOf func(string) ([]mux.PaneRef, bool), agent string) []mirror {
	var out []mirror
	for _, m := range expanded {
		panes, ok := panesOf(m.Session)
		if !ok {
			continue
		}
		pane, found := agentPane(panes, agent)
		if !found || !pane.Fullscreen {
			continue
		}
		out = append(out, mirror{Session: m.Session, Pane: pane.Addr(), Label: m.Label})
	}
	return out
}
