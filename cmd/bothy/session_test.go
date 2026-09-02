package main

import (
	"reflect"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
)

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
