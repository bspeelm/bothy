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
	want := []string{"bothy-site", "bothy-bothy"}
	if got := liveSessions(string(out)); !reflect.DeepEqual(got, want) {
		t.Errorf("liveSessions = %v, want %v", got, want)
	}
}

// zellij writes prose to the same stream -- "No active zellij sessions
// found." -- and a sentence parsed as a session name would put a session
// called "No" in the list.
func TestLiveIgnoresProseOnTheSessionList(t *testing.T) {
	if got := liveSessions("No active zellij sessions found.\n"); len(got) != 0 {
		t.Errorf("liveSessions read %v out of a sentence", got)
	}
}
