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

// The launch decision was the most branching logic in the project and the
// least covered: spawn a window, hop into a container, or run here, crossed
// with where the directory and profile come from. planLaunch takes it without
// touching disk, so the matrix can be asserted rather than run.
func TestPlanLaunchDecidesWhereTheWorkspaceOpens(t *testing.T) {
	// The terminal question is decideLaunch's and is tested there; replacing it
	// makes this matrix about the rest, on any machine.
	restore := decide
	t.Cleanup(func() { decide = restore })
	decide = func(platform.Info, string) launchMode { return launchMode{} }

	here := platform.Info{Home: "/home/u", Terminal: "ghostty"}

	for _, tc := range []struct {
		name  string
		p     platform.Info
		cfg   config.Config
		cwd   string
		flags launchFlags
		want  launchPlan
	}{
		{
			name: "the current directory, when nothing says otherwise",
			p:    here, cfg: config.Config{Profile: "cockpit"}, cwd: "/w/proj",
			want: launchPlan{Dir: "/w/proj", Profile: "cockpit"},
		},
		{
			name: "the configured project directory beats the current one",
			p:    here, cwd: "/w/proj",
			cfg:  config.Config{Profile: "cockpit", Workspace: config.Workspace{ProjectDir: "/w/other"}},
			want: launchPlan{Dir: "/w/other", Profile: "cockpit"},
		},
		{
			name: "--dir beats both",
			p:    here, cwd: "/w/proj",
			cfg:   config.Config{Profile: "cockpit", Workspace: config.Workspace{ProjectDir: "/w/other"}},
			flags: launchFlags{Dir: "/w/flag"},
			want:  launchPlan{Dir: "/w/flag", Profile: "cockpit"},
		},
		{
			name: "~ in the configured directory is expanded",
			p:    here, cwd: "/w/proj",
			cfg:  config.Config{Profile: "cockpit", Workspace: config.Workspace{ProjectDir: "~/code"}},
			want: launchPlan{Dir: "/home/u/code", Profile: "cockpit"},
		},
		{
			name: "--profile beats the configured one",
			p:    here, cfg: config.Config{Profile: "cockpit"}, cwd: "/w/proj",
			flags: launchFlags{Profile: "minimal"},
			want:  launchPlan{Dir: "/w/proj", Profile: "minimal"},
		},
		{
			name: "a configured container is hopped into",
			p:    here, cwd: "/w/proj",
			cfg:  config.Config{Profile: "cockpit", Workspace: config.Workspace{Container: "dev"}},
			want: launchPlan{Dir: "/w/proj", Profile: "cockpit", Container: "dev"},
		},
		{
			name: "already inside a container: no hop, or it would nest",
			p:    platform.Info{Home: "/home/u", Terminal: "ghostty", Container: platform.Toolbx},
			cwd:  "/w/proj",
			cfg:  config.Config{Profile: "cockpit", Workspace: config.Workspace{Container: "dev"}},
			want: launchPlan{Dir: "/w/proj", Profile: "cockpit"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := planLaunch(tc.p, tc.cfg, tc.cwd, tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			got.Reason = "" // the wording is decideLaunch's business
			if got != tc.want {
				t.Errorf("planLaunch = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The two flags are opposites, and saying both is a mistake worth naming
// rather than resolving by precedence.
func TestPlanLaunchRefusesContradictoryFlags(t *testing.T) {
	_, err := planLaunch(platform.Info{}, config.Config{}, "/w",
		launchFlags{Window: true, InPlace: true})
	if err == nil {
		t.Error("--window and --in-place together were accepted")
	}
}

// Spawning is decided before the container hop so the window opens once, on
// the host. A plan that did both would open a window and then hop out of it.
func TestPlanLaunchSpawnsOrHopsButSaysBoth(t *testing.T) {
	restore := decide
	t.Cleanup(func() { decide = restore })
	decide = func(platform.Info, string) launchMode {
		return launchMode{Spawn: true, Reason: "this terminal cannot draw images"}
	}

	got, err := planLaunch(
		platform.Info{Home: "/home/u"},
		config.Config{Profile: "cockpit", Workspace: config.Workspace{Container: "dev"}},
		"/w/proj", launchFlags{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Spawn {
		t.Error("a terminal that cannot draw images should spawn one that can")
	}
	if got.Container != "dev" {
		t.Error("the container is still named: spawning may fail, and then the hop is what happens")
	}
	if got.Reason == "" {
		t.Error("no reason given, so a failed spawn would report nothing")
	}
}
