package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy attach` was the one command with no tests, and it had collected four
// bugs. Each test below is one of them.

// sandbox builds a platform.Info rooted in a temp dir, with an executable
// planted in bothy's own bin when want is true.
func sandbox(t *testing.T, ownZellij bool) platform.Info {
	t.Helper()
	home := t.TempDir()
	p := platform.Info{
		OS: "linux", Arch: "x86_64",
		Home:      home,
		DataDir:   filepath.Join(home, ".local", "share"),
		ConfigDir: filepath.Join(home, ".config"),
		LocalBin:  filepath.Join(home, ".local", "bin"),
	}
	if ownZellij {
		if err := os.MkdirAll(p.BinDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.BinDir(), "zellij"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// #13. On a machine where bothy supplied zellij -- the gap-filling case the
// whole project exists for -- attach resolved through the ambient PATH and
// reported "zellij is not installed" while `bothy` launched fine.
func TestAttachPrefersTheZellijBothyInstalled(t *testing.T) {
	p := sandbox(t, true)
	plan, err := planAttach(p, config.Default(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(p.BinDir(), "zellij")
	if plan.Bin != want {
		t.Errorf("Bin = %q, want bothy's own copy at %q", plan.Bin, want)
	}
}

// #14. The session server keeps the environment it was launched with, but the
// client reads config too -- keybindings in particular -- so an attach without
// bothy's environment reads the user's own zellij config.
func TestAttachCarriesBothysEnvironment(t *testing.T) {
	p := sandbox(t, true)
	plan, err := planAttach(p, config.Default(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var found string
	for _, kv := range plan.Env {
		if strings.HasPrefix(kv, "ZELLIJ_CONFIG_DIR=") {
			found = kv
		}
	}
	if found == "" {
		t.Fatal("ZELLIJ_CONFIG_DIR is not set; the client would read the user's own config")
	}
	if !strings.Contains(found, p.BothyDir()) {
		t.Errorf("%s does not point inside bothy's tree", found)
	}
}

// #18. The hop line is interpolated into `bash -lc`, and hopIntoContainer has
// always quoted what it embeds. This one joined raw, so a session name with a
// space in it arrived as two arguments.
func TestAttachQuotesWhatItPutsInAShell(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, `'zellij' attach`},
		{"plain name", []string{"work"}, `'zellij' attach 'work'`},
		{"a space", []string{"my session"}, `'zellij' attach 'my session'`},
		{"a quote", []string{"it's"}, `'zellij' attach 'it'\''s'`},
		{"a semicolon", []string{"a; rm -rf /"}, `'zellij' attach 'a; rm -rf /'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := attachCommand("zellij", tc.args); got != tc.want {
				t.Errorf("attachCommand() = %s, want %s", got, tc.want)
			}
		})
	}
}

// The hop is taken only from outside a container, and only when one is
// configured -- inside one, this is already the right machine.
func TestAttachHopsOnlyFromOutsideAContainer(t *testing.T) {
	cfg := config.Default()
	cfg.Workspace.Container = "bothy-test"

	outside := sandbox(t, true)
	plan, err := planAttach(outside, cfg, "", []string{"work"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Container != "bothy-test" {
		t.Errorf("Container = %q, want the configured one", plan.Container)
	}
	if plan.Bin != "" {
		t.Errorf("Bin = %q; a hop should not also resolve a local binary", plan.Bin)
	}

	inside := sandbox(t, true)
	inside.Container = platform.Toolbx
	inside.ContainerName = "bothy-test"
	plan, err = planAttach(inside, cfg, "", []string{"work"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Container != "" {
		t.Errorf("Container = %q from inside a container; it would hop into itself", plan.Container)
	}
	if plan.Bin == "" {
		t.Error("no binary resolved from inside the container")
	}
}

// A multiplexer that is nowhere is an error, not a panic or an empty command.
func TestAttachReportsAMissingMultiplexer(t *testing.T) {
	p := sandbox(t, false)
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	if _, err := planAttach(p, config.Default(), "", nil); err == nil {
		t.Error("attach resolved a zellij that is not installed anywhere")
	}
}

// #24. The `attach` word was checked before the FlagSet ran, so
// `bothy dev --profile x attach` saw "--profile" in args[0], fell through,
// and started a fresh session while silently ignoring what the user asked for.
//
// Asserted structurally: the check must come after fs.Parse. Running cmdDev
// itself would launch a workspace, which a test cannot do.
func TestDevChecksForAttachAfterParsingFlags(t *testing.T) {
	src, err := os.ReadFile("dev.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	parse := strings.Index(body, "if err := fs.Parse(args); err != nil")
	check := strings.Index(body, `fs.Arg(0) == "attach"`)
	if parse < 0 || check < 0 {
		t.Fatal("cmdDev no longer parses flags or no longer special-cases attach")
	}
	if check < parse {
		t.Error("cmdDev checks for `attach` before parsing flags, so `bothy dev --profile x attach` is misread")
	}
}
