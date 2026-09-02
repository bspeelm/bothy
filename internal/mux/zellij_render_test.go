package mux

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/layout"
)

func devCommands() layout.Commands {
	return layout.Commands{"browser": "yazi", "agent": "claude", "editor": "vim"}
}
func mustrender(t *testing.T, path string) string {
	t.Helper()
	p, err := layout.LoadProfile(path)
	if err != nil {
		t.Fatalf("layout.LoadProfile(%s): %v", path, err)
	}
	kdl, err := render(p, devCommands())
	if err != nil {
		t.Fatalf("render(%s): %v", path, err)
	}
	return kdl
}

// The cockpit profile must render byte-for-byte the layout the origin setup ran
// by hand. This is the whole point of the port: if this drifts, the workspace
// someone gets from bothy is not the workspace the cheat sheet describes.
func TestCockpitMatchesOriginLayout(t *testing.T) {
	got := mustrender(t, "../../profiles/cockpit.toml")

	want := `layout {
    pane size=1 borderless=true {
        plugin location="zellij:tab-bar"
    }
    pane size="50%" {
        command "yazi"
    }
    pane split_direction="vertical" {
        pane focus=true name="agent" {
            command "claude"
        }
        pane size="40%" name="side"
    }
    pane size=2 borderless=true {
        plugin location="zellij:status-bar"
    }
}
`
	if body := stripHeader(got); body != want {
		t.Errorf("cockpit layout drifted from the origin dev.kdl.\n--- got ---\n%s\n--- want ---\n%s", body, want)
	}
}

// Trap 1: Zellij's "vertical" split direction lays panes out as columns. A row
// with more than one pane must produce it; a single-pane row must not.
func TestSideBySidePanesUseVerticalSplit(t *testing.T) {
	got := mustrender(t, "../../profiles/cockpit.toml")
	if !strings.Contains(got, `split_direction="vertical"`) {
		t.Error(`a row with two panes must render split_direction="vertical" (Zellij's word for columns)`)
	}
	if strings.Contains(got, `split_direction="horizontal"`) {
		t.Error(`nothing should emit split_direction="horizontal"; rows are the default stacking`)
	}
	// The browser row holds one pane, so it must not be wrapped in a split.
	browser := got[strings.Index(got, `size="50%"`):]
	if strings.Contains(browser[:strings.Index(browser, "\n")], "split_direction") {
		t.Error("a single-pane row must not be wrapped in a split")
	}
}

// Trap 2: `{ plugin location="…" }` on one line needs a trailing semicolon or
// Zellij fails to deserialise it. Always emitting the body multi-line sidesteps
// the question entirely — so assert the plugin node is never on one line.
func TestPluginBodiesAreMultiLine(t *testing.T) {
	got := mustrender(t, "../../profiles/cockpit.toml")
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "plugin location=") && strings.Contains(line, "{") {
			t.Errorf("plugin node emitted on one line, which needs a trailing ';': %q", line)
		}
	}
}

// Zellij execs pane commands directly rather than through a shell, so a command
// with arguments has to become `command` plus `args`.
func TestCommandWithArgsIsSplit(t *testing.T) {
	p := layout.Profile{
		Name:      "t",
		TabBar:    boolp(false),
		StatusBar: boolp(false),
		Rows:      []layout.Row{{Panes: []layout.Pane{{Slot: "agent"}}}},
	}
	got, err := render(p, layout.Commands{"agent": "claude --continue"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, `command "claude"`) {
		t.Errorf("want command \"claude\", got:\n%s", got)
	}
	if !strings.Contains(got, `args "--continue"`) {
		t.Errorf("want args \"--continue\", got:\n%s", got)
	}
}

// A pane with no command is a plain shell. Zellij wants no body at all there.
func TestShellPaneHasNoBody(t *testing.T) {
	got := mustrender(t, "../../profiles/cockpit.toml")
	if !strings.Contains(got, `pane size="40%" name="side"`+"\n") {
		t.Errorf("the side pane should be a bare node with no body:\n%s", got)
	}
}
func TestAllShippedProfilesrender(t *testing.T) {
	for _, name := range []string{"cockpit", "editor", "minimal"} {
		p, err := layout.LoadProfile("../../profiles/" + name + ".toml")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if p.Name != name {
			t.Errorf("%s: profile names itself %q", name, p.Name)
		}
		if _, err := render(p, devCommands()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
func TestMinimalDisablesTabBar(t *testing.T) {
	got := mustrender(t, "../../profiles/minimal.toml")
	if strings.Contains(got, "zellij:tab-bar") {
		t.Error("minimal sets tab_bar = false; the bar should not be rendered")
	}
	if !strings.Contains(got, "zellij:status-bar") {
		t.Error("the status bar is the only on-screen key hint and should remain")
	}
}

// A pane naming a slot nothing fills would launch an empty pane at best, so
// rendering has to fail rather than produce a subtly broken workspace.
func TestUnfilledSlotIsAnError(t *testing.T) {
	p := layout.Profile{Name: "t", Rows: []layout.Row{{Panes: []layout.Pane{{Slot: "browser"}}}}}
	if _, err := render(p, layout.Commands{}); err == nil {
		t.Fatal("expected an error when a pane's slot has no command")
	}
}
func TestGeneratedLayoutSaysHowToChangeIt(t *testing.T) {
	got := mustrender(t, "../../profiles/cockpit.toml")
	if !strings.HasPrefix(got, "// managed by bothy") {
		t.Error("generated layouts must identify themselves as generated")
	}
	// Layout changes apply at launch only — someone editing the KDL live and
	// wondering why nothing happens is a documented trap.
	if !strings.Contains(got, "launch only") {
		t.Error("the header should say layout changes apply at launch only")
	}
}
func stripHeader(kdl string) string {
	i := strings.Index(kdl, "layout {")
	if i < 0 {
		return kdl
	}
	return kdl[i:]
}

// The README promises the agent slot takes "any command you name". It did not:
// strings.Fields split on whitespace with no quote awareness, so
// `claude --append-system-prompt "be terse"` became the arguments `"be` and
// `terse"`, which Zellij passed on literally. An empty command panicked.
func TestSplitCommandHonoursQuotes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"plain", "claude", []string{"claude"}},
		{"flags", "claude --continue", []string{"claude", "--continue"}},
		{"double quotes", `claude --append-system-prompt "be terse"`,
			[]string{"claude", "--append-system-prompt", "be terse"}},
		{"single quotes", `sh -c 'echo hi'`, []string{"sh", "-c", "echo hi"}},
		{"quote inside a word", `git commit -m"a b"`, []string{"git", "commit", "-ma b"}},
		{"an empty argument", `sh -c ""`, []string{"sh", "-c", ""}},
		{"escaped space", `vim a\ b`, []string{"vim", "a b"}},
		{"a quote inside double quotes", `echo "it's"`, []string{"echo", "it's"}},
		{"runs of whitespace", "  claude   --continue  ", []string{"claude", "--continue"}},
		{"empty", "", nil},
		{"only spaces", "   ", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitCommand(tc.in)
			if err != nil {
				t.Fatalf("splitCommand(%q) = %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("splitCommand(%q) = %q, want %q", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitCommand(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
func TestSplitCommandRejectsAnUnbalancedQuote(t *testing.T) {
	if _, err := splitCommand(`claude --prompt "unterminated`); err == nil {
		t.Error("an unbalanced quote was accepted; the argument would silently swallow the rest of the line")
	}
}

// An empty command indexed parts[0] and took the whole program down with an
// index-out-of-range, from nothing worse than a stray line in a profile.
func TestAnEmptyCommandIsAnErrorNotAPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("rendering an empty command panicked: %v", r)
		}
	}()
	p := layout.Profile{Name: "x", Rows: []layout.Row{{Panes: []layout.Pane{{Name: "a", Command: "   "}}}}}
	if _, err := render(p, layout.Commands{}); err == nil {
		t.Error("an empty command rendered without complaint")
	}
}

func boolp(b bool) *bool { return &b }
