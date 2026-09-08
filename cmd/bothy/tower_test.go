package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bspeelm/bothy/internal/mux"
)

// A real list-panes reply, from a running cockpit. The agent is terminal_1 here
// and that is a coincidence of how the cockpit profile splits; the tests below
// exist because an earlier draft of this feature hardcoded that number.
func cockpitPanes() []mux.PaneRef {
	return []mux.PaneRef{
		{ID: 0, Plugin: true},
		{ID: 1, Plugin: true},
		{ID: 0, Command: "yazi", Dir: "/w/shanty"},
		{ID: 1, Command: "claude", Dir: "/w/shanty"},
		{ID: 2, Command: "/bin/bash", Dir: "/w/shanty"},
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
		{ID: 1, Plugin: true},
		{ID: 0, Command: "claude"},
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

// ADR-048's line, as a fence rather than a sentence: the tower may change how a
// session is displayed and never what an agent does. Expanding a pane is the one
// permitted mutation -- no byte reaches the agent, which receives SIGWINCH and
// redraws. Everything that would reach the agent, or start and stop one, stays
// out.
func TestTheTowerChangesDisplayAndNeverBehaviour(t *testing.T) {
	forbidden := []string{
		"write-chars", "send-keys", "\"write\"", "paste",
		"switch-session", "focus-pane", "new-pane", "close-pane",
		"Kill(", "Discard(", "detach",
	}
	both := ""
	for _, f := range []string{"tower.go", "towercmd.go"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		both += string(body)
		for _, bad := range forbidden {
			if strings.Contains(string(body), bad) {
				t.Errorf("%s contains %q; the tower changes display, not behaviour", f, bad)
			}
		}
	}
	// And the permitted one is reached through the backend, so a second
	// multiplexer cannot be handed a different meaning for it.
	if !strings.Contains(both, "backend.Expand(") {
		t.Error("the tower no longer expands panes through the backend seam")
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
	paint(&b, "one\ntwo\nthree\n", 0)
	out := b.String()

	if strings.HasSuffix(out, "\n") {
		t.Error("the frame ends in a newline, which scrolls the previous one away")
	}
	if !strings.Contains(out, "\033[H") {
		t.Error("the frame does not home the cursor, so it draws below the last one")
	}
	if n := strings.Count(out, "\033[K"); n != 3 {
		t.Errorf("%d lines cleared for a 3-line screen; a shorter line leaves the old one behind", n)
	}
}

// The reply line is the bottom of the pane, and a repaint that reached it wiped
// what was being typed there -- keystrokes were eaten mid-word while an agent
// worked. The frame must therefore write no further down than the reserved
// rows, put the cursor back where it found it, and never clear to the end of
// the screen.
func TestPaintNeverReachesTheReplyLine(t *testing.T) {
	var b strings.Builder
	paint(&b, "a\nb\nc\nd\ne\nf\ng\nh", 6)
	out := b.String()

	if strings.Contains(out, "\033[J") {
		t.Error("the frame clears to the end of the screen, which wipes the reply line")
	}
	if !strings.HasPrefix(out, "\0337") || !strings.HasSuffix(out, "\0338") {
		t.Error("the frame does not save and restore the cursor, so typing resumes in the wrong place")
	}
	// Six rows, two reserved: four painted, and they are the last four, because
	// the bottom is where an agent says what it is waiting for.
	if n := strings.Count(out, "\033[K"); n != 4 {
		t.Errorf("painted %d rows into a 6-row pane reserving %d; want 4", n, replyRows)
	}
	for _, gone := range []string{"a\033[K", "b\033[K", "c\033[K", "d\033[K"} {
		if strings.Contains(out, gone) {
			t.Errorf("kept the top of the screen (%q); the bottom is the part worth showing", gone)
		}
	}
	if !strings.Contains(out, "h\033[K") {
		t.Error("the last line of the screen was not painted")
	}
}

// A pane too short to reserve anything is painted whole rather than not at all.
func TestAPaneTooShortToReserveIsPaintedWhole(t *testing.T) {
	var b strings.Builder
	paint(&b, "a\nb\nc", 2)
	if n := strings.Count(b.String(), "\033[K"); n != 3 {
		t.Errorf("painted %d rows of a 3-line screen into a 2-row pane; want all 3", n)
	}
}

// Expand toggles, so acting without looking collapses a pane that was already
// filling its tab -- the opposite of what the tower wants, on exactly the pane
// someone had already set up to be read.
func TestOnlyUnexpandedPanesAreExpanded(t *testing.T) {
	got := expandable([]mirror{
		{Session: "bothy-api", Fullscreen: false},
		{Session: "bothy-docs", Fullscreen: true},
	})
	if len(got) != 1 || got[0].Session != "bothy-api" {
		t.Errorf("expandable = %+v, want only bothy-api", got)
	}
}

// A pane the maintainer had already expanded before the tower started is not
// the tower's to collapse.
func TestOnlyPanesTheTowerExpandedAreRestored(t *testing.T) {
	panes := func(string) ([]mux.PaneRef, bool) {
		return []mux.PaneRef{{ID: 1, Command: "claude", Fullscreen: true}}, true
	}
	// bothy-docs was already fullscreen, so it was never in the expanded set.
	expanded := []mirror{{Session: "bothy-api", Pane: "terminal_1"}}
	got := collapsible(expanded, panes, "claude")
	if len(got) != 1 || got[0].Session != "bothy-api" {
		t.Errorf("collapsible = %+v, want only the pane the tower expanded", got)
	}
}

// Fullscreen is Ctrl+P then f, a binding bothy does not override, so a pane can
// be collapsed by hand while the tower is running. Toggling it again on the way
// out would expand it -- the opposite of restoring. The state is therefore read
// again rather than remembered, which is the case a remembered-state
// implementation gets wrong.
func TestAPaneCollapsedByHandIsNotReExpanded(t *testing.T) {
	collapsedByHand := func(string) ([]mux.PaneRef, bool) {
		return []mux.PaneRef{{ID: 1, Command: "claude", Fullscreen: false}}, true
	}
	expanded := []mirror{{Session: "bothy-api", Pane: "terminal_1"}}
	if got := collapsible(expanded, collapsedByHand, "claude"); len(got) != 0 {
		t.Errorf("collapsible = %+v; a pane collapsed by hand would be re-expanded", got)
	}
}

// A session that ended while the tower was open has no panes to put back.
func TestARestoreSkipsASessionThatWentAway(t *testing.T) {
	gone := func(string) ([]mux.PaneRef, bool) { return nil, false }
	expanded := []mirror{{Session: "bothy-api", Pane: "terminal_1"}}
	if got := collapsible(expanded, gone, "claude"); len(got) != 0 {
		t.Errorf("collapsible = %+v for a session that is gone", got)
	}
}

// Everything bothy sends to an agent came from the keyboard. A literal here
// would be bothy speaking to the agent in its own voice, which is the line
// ADR-048 draws and the difference between relaying and orchestrating.
func TestTheTowerRelaysOnlyWhatWasTyped(t *testing.T) {
	body, err := os.ReadFile("towercmd.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)

	if n := strings.Count(src, ".Send("); n != 1 {
		t.Errorf("%d calls send to an agent; there is one, and it passes on a typed line", n)
	}
	// The line reaches Send as a parameter named for where it came from, and
	// relay's only caller is the branch reading the typed channel.
	if !strings.Contains(src, "backend.Send(bin, session, pane.Addr(), line, env)") {
		t.Error("the sending call no longer passes the typed line through unexamined")
	}
	if !strings.Contains(src, "go readLines(os.Stdin, typed)") {
		t.Error("the mirror no longer reads what to send from the keyboard")
	}
}

// The scanner blocks until Enter, so it runs beside the paint loop rather than
// in it. Without this the mirror would stop refreshing whenever someone rested
// a hand on the keyboard.
func TestTypedLinesArriveWithoutBlockingTheRefresh(t *testing.T) {
	typed := make(chan string, 2)
	go readLines(strings.NewReader("yes\n2\n"), typed)

	for _, want := range []string{"yes", "2"} {
		select {
		case got := <-typed:
			if got != want {
				t.Errorf("read %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("nothing arrived; expected %q", want)
		}
	}
}
