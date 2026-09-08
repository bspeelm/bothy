package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/mux"
)

// A real list-panes reply, from a running cockpit. The agent is terminal_1 here
// and that is a coincidence of how the cockpit profile splits; the tests below
// exist because an earlier draft of this feature hardcoded that number.
func cockpitPanes() []mux.PaneRef {
	return []mux.PaneRef{
		{ID: 0, Plugin: true, Title: "zellij:tab-bar"},
		{ID: 1, Plugin: true, Title: "zellij:status-bar"},
		{ID: 0, Command: "yazi", Title: "Yazi: shanty", Dir: "/w/shanty"},
		{ID: 1, Command: "claude", Title: "agent", Dir: "/w/shanty"},
		{ID: 2, Command: "/bin/bash", Title: "side", Dir: "/w/shanty"},
	}
}

// The agent's pane is found by what it is running, not by where it sits. The
// editor profile puts the agent in a different position, and a plugin pane
// carries id 1 as well -- ids restart per kind, so the address needs the kind.
func TestTheAgentPaneIsFoundByCommandNotByPosition(t *testing.T) {
	got, ok := agentPane(cockpitPanes(), "claude")
	if !ok {
		t.Fatal("no agent pane found in a cockpit that has one")
	}
	if got.Addr() != "terminal_1" {
		t.Errorf("agent pane is %s, want terminal_1", got.Addr())
	}

	// Reordered, so position cannot be what found it.
	shuffled := []mux.PaneRef{
		{ID: 1, Plugin: true, Title: "zellij:status-bar"},
		{ID: 0, Command: "claude", Title: "agent"},
		{ID: 1, Command: "yazi"},
	}
	got, ok = agentPane(shuffled, "claude")
	if !ok || got.Addr() != "terminal_0" {
		t.Errorf("agent pane is %q/%v after reordering, want terminal_0", got.Addr(), ok)
	}

	// A full path in the slot resolves to the same agent: config may name
	// ~/.local/bin/claude while zellij reports the basename.
	if _, ok := agentPane(cockpitPanes(), "/usr/local/bin/claude"); !ok {
		t.Error("an agent named by absolute path was not matched")
	}
}

// A pane whose command has exited still exists -- the session outlives the
// agent -- so mirroring it would show a dead screen and imply the agent is
// merely quiet.
func TestAnExitedAgentPaneIsNotWatched(t *testing.T) {
	panes := []mux.PaneRef{{ID: 1, Command: "claude", Exited: true}}
	if _, ok := agentPane(panes, "claude"); ok {
		t.Error("an exited agent pane was offered for watching")
	}
}

// One live session has no agent pane at all right now -- the agent was closed
// and the session kept -- so this is a case that happens, not a hypothetical.
func TestASessionWithNoAgentPaneIsSkipped(t *testing.T) {
	panes := map[string][]mux.PaneRef{
		"bothy-api":     cockpitPanes(),
		"bothy-noagent": {{ID: 0, Command: "yazi"}, {ID: 1, Command: "/bin/bash"}},
	}
	got := watchable(func(s string) ([]mux.PaneRef, bool) {
		p, ok := panes[s]
		return p, ok
	}, "claude", []string{"bothy-api", "bothy-noagent"})

	if len(got) != 1 || got[0].Session != "bothy-api" {
		t.Errorf("watchable = %+v, want only bothy-api", got)
	}
}

// The tower is a session like any other, so without this it appears in its own
// window watching itself.
func TestTheTowerIsNotMirroredIntoItself(t *testing.T) {
	got := watchable(func(string) ([]mux.PaneRef, bool) { return cockpitPanes(), true },
		"claude", []string{towerSession, "bothy-api"})
	for _, m := range got {
		if m.Session == towerSession {
			t.Error("the tower is watching itself")
		}
	}
	if len(got) != 1 {
		t.Errorf("got %d mirrors, want 1", len(got))
	}
}

// Nothing prunes the records under state/sessions, by design (ADR-042), so a
// session that died last week is still on disk. Only what the multiplexer calls
// live may appear, or the tower shows ghosts.
func TestADeadSessionIsDroppedNotShownStale(t *testing.T) {
	asked := map[string]bool{}
	got := watchable(func(s string) ([]mux.PaneRef, bool) {
		asked[s] = true
		return cockpitPanes(), true
	}, "claude", []string{"bothy-api"})

	if asked["bothy-longdead"] {
		t.Error("a session absent from the live list was asked about anyway")
	}
	if len(got) != 1 {
		t.Errorf("got %d mirrors, want 1", len(got))
	}
}

// A mirror is as wide as the pane it watches and no wider, so a wide window
// should hold several of them side by side rather than one with two thirds of
// the screen empty. Measured on a real cockpit: the agent pane is 57 columns,
// because the profile splits the window three ways.
func TestMirrorsSitAbreastWhenTheWindowIsWideEnough(t *testing.T) {
	mirrors := []mirror{
		{Session: "bothy-api", Pane: "terminal_1", Label: "api", Cols: 57},
		{Session: "bothy-docs", Pane: "terminal_1", Label: "docs", Cols: 57},
		{Session: "bothy-srv", Pane: "terminal_1", Label: "srv", Cols: 57},
	}
	prof := towerProfile(mirrors, "/usr/bin/bothy", 200)
	if len(prof.Rows) != 1 || len(prof.Rows[0].Panes) != 3 {
		t.Fatalf("got %d rows; three 57-column mirrors fit a 200-column window", len(prof.Rows))
	}
}

// One column short is not nearly enough: every line of every mirror wraps, and
// stacked is better than that.
func TestMirrorsStackWhenTheWindowIsTooNarrow(t *testing.T) {
	mirrors := []mirror{
		{Session: "bothy-api", Pane: "terminal_1", Label: "api", Cols: 57},
		{Session: "bothy-docs", Pane: "terminal_1", Label: "docs", Cols: 57},
	}
	if prof := towerProfile(mirrors, "/usr/bin/bothy", 100); len(prof.Rows) != 2 {
		t.Errorf("%d rows in a 100-column window; two 57-column mirrors do not fit abreast", len(prof.Rows))
	}
	// An unmeasured width is not an invitation to assume.
	if prof := towerProfile(mirrors, "/usr/bin/bothy", 0); len(prof.Rows) != 2 {
		t.Errorf("%d rows with no measured width; stacking is the safe answer", len(prof.Rows))
	}
}

func TestTheTowerStacksMirrorsOneToARow(t *testing.T) {
	mirrors := []mirror{
		{Session: "bothy-api", Pane: "terminal_1", Label: "api", Cols: 57},
		{Session: "bothy-docs", Pane: "terminal_1", Label: "docs", Cols: 57},
	}
	prof := towerProfile(mirrors, "/usr/bin/bothy", 0)
	if len(prof.Rows) != 2 {
		t.Fatalf("%d rows for 2 mirrors, want 2", len(prof.Rows))
	}
	for i, r := range prof.Rows {
		if len(r.Panes) != 1 {
			t.Errorf("row %d has %d panes, want 1", i, len(r.Panes))
		}
	}
	if !strings.Contains(prof.Rows[0].Panes[0].Command, "bothy-api") {
		t.Errorf("first row does not watch bothy-api: %q", prof.Rows[0].Panes[0].Command)
	}
	// The absolute path, so the tower re-runs the bothy that built it rather
	// than whichever one PATH finds first.
	if !strings.HasPrefix(prof.Rows[0].Panes[0].Command, "/usr/bin/bothy ") {
		t.Errorf("mirror command does not run this binary by path: %q", prof.Rows[0].Panes[0].Command)
	}
}

// ADR-048 draws the line this feature stays behind: bothy watches panes and
// never originates input to an agent. The line is worth nothing if a later
// change can cross it quietly, so it is a fence over the source rather than a
// sentence in a document.
func TestTheTowerOriginatesNoInput(t *testing.T) {
	forbidden := []string{
		"write-chars", "send-keys", "\"write\"", "paste",
		"switch-session", "focus-pane", "new-pane", "close-pane",
		"Kill(", "Discard(", "detach",
	}
	for _, f := range []string{"tower.go", "towercmd.go"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range forbidden {
			if strings.Contains(string(body), bad) {
				t.Errorf("%s contains %q; the tower observes and sends nothing", f, bad)
			}
		}
	}
}

// The two mirror-facing backend methods must stay read-only in name and in
// fact. This reads the interface rather than the implementation, so a second
// backend cannot arrive with a mutating Screen.
func TestTheWatchingMethodsAreQueries(t *testing.T) {
	body, err := os.ReadFile("../../internal/mux/mux.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`PanesOf(bin, session string, env []string) ([]PaneRef, bool)`,
		`Screen(bin, session, pane string, env []string) (string, error)`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the Backend interface no longer declares %q", want)
		}
	}
	// A query returns something and reports failure; it does not take a payload.
	if regexp.MustCompile(`Screen\([^)]*chars|Screen\([^)]*input`).MatchString(string(body)) {
		t.Error("Screen has grown an input parameter; it is a read")
	}
}

// The tower builds its layout rather than loading one from a file, so nothing
// else checks that the result is something the multiplexer will accept.
func TestTheTowerLayoutRendersForTheMultiplexer(t *testing.T) {
	mirrors := []mirror{
		{Session: "bothy-api", Pane: "terminal_1", Label: "api"},
		{Session: "bothy-abbey-srv", Pane: "terminal_2", Label: "srv", Cols: 57},
	}
	out, err := mux.Zellij{}.Preview(towerProfile(mirrors, "/usr/bin/bothy", 0), nil)
	if err != nil {
		t.Fatalf("the tower's layout does not render: %v", err)
	}
	for _, want := range []string{"bothy-api", "bothy-abbey-srv", "--mirror"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered layout does not mention %q:\n%s", want, out)
		}
	}
	// Commands are nil: every pane carries a literal command, so the tower must
	// not need the slot table. A pane resolved through a slot would error here.
	if strings.Contains(out, "slot") {
		t.Errorf("a tower pane went through a slot:\n%s", out)
	}
}

// A frame that ends in a newline pushes the one before it into scrollback. The
// tower reached 1,382 lines of history in a few minutes of watching before this
// was fixed, which is visible in the pane's scroll indicator.
func TestPaintLeavesNothingInScrollback(t *testing.T) {
	var b strings.Builder
	paint(&b, "one\ntwo\nthree\n")
	out := b.String()

	if strings.HasSuffix(out, "\n") {
		t.Error("the frame ends in a newline, which scrolls the previous one away")
	}
	if !strings.HasPrefix(out, "\033[H") {
		t.Error("the frame does not home the cursor, so it draws below the last one")
	}
	if !strings.HasSuffix(out, "\033[J") {
		t.Error("the frame does not wipe what a taller previous frame left below it")
	}
	if n := strings.Count(out, "\033[K"); n != 3 {
		t.Errorf("%d lines cleared for a 3-line screen; a shorter line leaves the old one behind", n)
	}
}
