package mux

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// These moved from the launcher with the code they cover: the naming rule and
// the attach/create split are zellij's, not bothy's. tmux would answer both
// differently -- it accepts "." and ":" in a name and then cannot address the
// session (#64).

// #47. Every launch created an anonymous session, so two projects produced two
// sessions bothy could not tell apart and `bothy attach` could not choose
// between. The name has to come from the directory for that to be fixable.
func TestSessionNameComesFromTheDirectory(t *testing.T) {
	for _, tc := range []struct{ dir, want string }{
		{"/home/me/work", "bothy-work"},
		{"/home/me/work/", "bothy-work"},
		{"/home/me/my project", "bothy-my-project"},
		{"/home/me/my.project", "bothy-my-project"},
		{"/home/me/a--b", "bothy-a-b"},
		{"/home/me/.config", "bothy-config"},
		{"/", "bothy"},
	} {
		t.Run(tc.dir, func(t *testing.T) {
			if got := z.SessionName(tc.dir); got != tc.want {
				t.Errorf("z.SessionName(%q) = %q, want %q", tc.dir, got, tc.want)
			}
		})
	}
}

// The name is a directory under zellij's cache, so anything that would end the
// path component or escape it must not survive into it.
func TestSessionNameIsSafeAsAPathComponent(t *testing.T) {
	for _, dir := range []string{"/home/me/a/b", "/home/me/..", "/home/me/a\x00b", "/home/me/a b"} {
		name := z.SessionName(dir)
		if strings.ContainsAny(name, "/\x00 ") || name == ".." {
			t.Errorf("z.SessionName(%q) = %q, which is not safe as a path component", dir, name)
		}
	}
}

// zellij applies --layout to a session that already exists by adding it as a
// new tab. Carrying the layout into an attach would therefore grow the
// workspace by three panes every time someone ran `bothy` twice in one project.
func TestLaunchDoesNotCarryALayoutIntoALiveSession(t *testing.T) {
	args := z.launchArgs("bothy-work", "/tmp/cockpit.kdl", []string{"bothy-other", "bothy-work"})
	if want := []string{"attach", "bothy-work"}; !reflect.DeepEqual(args, want) {
		t.Errorf("launchArgs into a live session = %v, want %v", args, want)
	}
	for _, a := range args {
		if a == "--layout" {
			t.Error("--layout is passed to a session that already exists; zellij would add a tab")
		}
	}
}

// And a session that is not running is created with the layout, rather than
// attached to and found empty.
func TestLaunchCreatesWithTheLayoutWhenNothingIsRunning(t *testing.T) {
	want := []string{"--layout", "/tmp/cockpit.kdl", "attach", "--create", "bothy-work"}
	if got := z.launchArgs("bothy-work", "/tmp/cockpit.kdl", nil); !reflect.DeepEqual(got, want) {
		t.Errorf("launchArgs with nothing live = %v, want %v", got, want)
	}
}

// zellij has three states for a session and `list-sessions --short` reports
// two: a session whose last client has gone is EXITED, absent from that list,
// and resurrected by the next attach -- which brings back the saved layout
// with every command suspended behind "Waiting to run".
//
// So launchArgs is asked to create, and creating is right; what has to happen
// first is that the dead one stops being in the way. This asserts the shape
// the launch path depends on: not-live means create, and the caller clears the
// ground before it does.
func TestASessionMissingFromTheLiveListIsCreated(t *testing.T) {
	args := z.launchArgs("bothy-work", "/tmp/cockpit.kdl", []string{"bothy-other"})
	if len(args) == 0 || args[0] != "--layout" {
		t.Fatalf("launchArgs = %v, want a create carrying the freshly rendered layout", args)
	}
	// The layout is the point: a resurrection would use zellij's saved copy
	// and quietly ignore a changed profile.
	if args[1] != "/tmp/cockpit.kdl" {
		t.Errorf("the layout passed is %q, want the one just rendered", args[1])
	}
}

// A session zellij has stopped stays on the list, and `--short` prints it
// with the "(EXITED - attach to resurrect)" marker stripped. Reading that as
// live skips discardDead and drops --layout, so the launch resurrects the
// saved session instead of creating one: every command pane comes up
// suspended behind "Waiting to run", and a changed profile is ignored (#188).
//
// The fixture is real `list-sessions --no-formatting` output with the project
// names scrubbed. It carries all three line shapes zellij writes: exited,
// running, and the running one you are attached to.
func TestLiveOmitsTheSessionsZellijHasExited(t *testing.T) {
	out, err := os.ReadFile("testdata/list-sessions.txt")
	if err != nil {
		t.Fatal(err)
	}
	live, stopped := splitSessions(string(out))
	if want := []string{"bothy-site", "bothy-bothy"}; !reflect.DeepEqual(live, want) {
		t.Errorf("live = %v, want %v", live, want)
	}
	// The other half of the same read: what `bothy ls --prune` would clear.
	if want := []string{"polite-galaxy", "bothy-work"}; !reflect.DeepEqual(stopped, want) {
		t.Errorf("stopped = %v, want %v", stopped, want)
	}
}

// zellij writes prose to the same stream -- "No active zellij sessions
// found." -- and a sentence parsed as a session name would put a session
// called "No" in the list.
func TestLiveIgnoresProseOnTheSessionList(t *testing.T) {
	live, stopped := splitSessions("No active zellij sessions found.\n")
	if len(live) != 0 || len(stopped) != 0 {
		t.Errorf("splitSessions read %v / %v out of a sentence", live, stopped)
	}
}

// The output of `zellij action list-clients`, captured from a real session
// that had been re-entered. Two clients on one pane is the bug: zellij sizes
// the session to the smaller terminal, and a fullscreen TUI renders its wrap
// math against geometry that is not the window it is drawing into.
const twoClients = `CLIENT_ID ZELLIJ_PANE_ID RUNNING_COMMAND
1         terminal_1     claude         
2         terminal_1     claude         
`

// A live session with nobody looking at it prints the header and no rows, and
// that is the case a launch must go through -- it is what "detached" is.
const noClients = "CLIENT_ID ZELLIJ_PANE_ID RUNNING_COMMAND\n"

func TestCountClientsReadsRealOutput(t *testing.T) {
	if got := countClients(twoClients); got != 2 {
		t.Errorf("countClients on a re-entered session = %d, want 2", got)
	}
	if got := countClients(noClients); got != 0 {
		t.Errorf("countClients on a detached session = %d, want 0", got)
	}
	if got := countClients(""); got != 0 {
		t.Errorf("countClients on nothing = %d, want 0", got)
	}
}

// Re-running the launcher on a session already on someone's screen must stop
// before it attaches. There is no fixing this afterwards: zellij exposes no
// way to displace a client, and `action detach` addressed by environment exits
// 0 without detaching anyone.
func TestLaunchRefusesASessionSomeoneIsLookingAt(t *testing.T) {
	restore := attachedClients
	defer func() { attachedClients = restore }()
	attachedClients = func(string, []string, string, []string) (int, bool) { return 2, true }

	err := z.Open(Request{Session: "bothy-work", Live: []string{"bothy-work"}})
	if err == nil {
		t.Fatal("launched into a session with two clients attached")
	}
	// Both ways out, or it is a dead end. Quitting the other terminal is not
	// always available -- the session that prompted this had a client in a
	// window its owner could not get back to.
	for _, want := range []string{"bothy attach bothy-work", "bothy kill bothy-work"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal is %q, which never offers %q", err, want)
		}
	}
}

// Detached is the ordinary case -- you closed the terminal and came back -- and
// it must not be refused. Open goes on to render a layout it has not been
// given, so the assertion is that whatever fails, it is not this.
func TestLaunchReturnsToADetachedSession(t *testing.T) {
	restore := attachedClients
	defer func() { attachedClients = restore }()

	for _, tc := range []struct {
		name string
		n    int
		ok   bool
	}{
		{"nobody attached", 0, true},
		{"the probe could not say", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attachedClients = func(string, []string, string, []string) (int, bool) { return tc.n, tc.ok }
			err := z.Open(Request{Session: "bothy-work", Live: []string{"bothy-work"}})
			if err != nil && strings.Contains(err.Error(), "already open in another terminal") {
				t.Error("refused a session nobody is looking at")
			}
		})
	}
}
