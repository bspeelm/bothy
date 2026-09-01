package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
)

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
			if got := sessionName(tc.dir); got != tc.want {
				t.Errorf("sessionName(%q) = %q, want %q", tc.dir, got, tc.want)
			}
		})
	}
}

// The name is a directory under zellij's cache, so anything that would end the
// path component or escape it must not survive into it.
func TestSessionNameIsSafeAsAPathComponent(t *testing.T) {
	for _, dir := range []string{"/home/me/a/b", "/home/me/..", "/home/me/a\x00b", "/home/me/a b"} {
		name := sessionName(dir)
		if strings.ContainsAny(name, "/\x00 ") || name == ".." {
			t.Errorf("sessionName(%q) = %q, which is not safe as a path component", dir, name)
		}
	}
}

// zellij applies --layout to a session that already exists by adding it as a
// new tab. Carrying the layout into an attach would therefore grow the
// workspace by three panes every time someone ran `bothy` twice in one project.
func TestLaunchDoesNotCarryALayoutIntoALiveSession(t *testing.T) {
	args := launchArgs("bothy-work", "/tmp/cockpit.kdl", []string{"bothy-other", "bothy-work"})
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
	if got := launchArgs("bothy-work", "/tmp/cockpit.kdl", nil); !reflect.DeepEqual(got, want) {
		t.Errorf("launchArgs with nothing live = %v, want %v", got, want)
	}
}

// Bare `bothy attach` means this project. Naming a session explicitly is how
// you reach one belonging to somewhere else, so an argument still wins.
func TestAttachDefaultsToThisProjectsSession(t *testing.T) {
	p := sandbox(t, true)

	plan, err := planAttach(p, config.Default(), "bothy-work", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"attach", "bothy-work"}; !reflect.DeepEqual(plan.Args, want) {
		t.Errorf("bare attach = %v, want %v", plan.Args, want)
	}

	plan, err = planAttach(p, config.Default(), "bothy-work", []string{"bothy-elsewhere"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"attach", "bothy-elsewhere"}; !reflect.DeepEqual(plan.Args, want) {
		t.Errorf("named attach = %v, want %v", plan.Args, want)
	}
}

// #48. Opening a window was decided per invocation, so anyone who preferred
// otherwise typed --in-place every time. workspace.launch is the standing
// answer; the flags still win for one run.
func TestLaunchModeIsASettingTheFlagsOverride(t *testing.T) {
	cfg := config.Default()

	cfg.Workspace.Launch = "here"
	if got := launchModeFor(cfg, false, false); got != "here" {
		t.Errorf("with no flags = %q, want the configured %q", got, "here")
	}
	if got := launchModeFor(cfg, true, false); got != "window" {
		t.Errorf("--window against launch=here = %q, want window", got)
	}

	cfg.Workspace.Launch = "window"
	if got := launchModeFor(cfg, false, true); got != "here" {
		t.Errorf("--in-place against launch=window = %q, want here", got)
	}
}

// "auto" and "" both mean "decide from the terminal", and neither settles it.
func TestAutoLeavesTheDecisionToTheTerminal(t *testing.T) {
	clearDisplay(t)
	for _, mode := range []string{"", "auto"} {
		m := decideLaunch(platform.Info{Terminal: "ghostty"}, mode)
		if m.Reason == "asked for a window" || m.Reason == "asked to run in this terminal" {
			t.Errorf("mode %q was treated as a forced answer: %q", mode, m.Reason)
		}
	}
}
