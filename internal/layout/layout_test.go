package layout

import (
	"strings"
	"testing"
)

func devCommands() Commands {
	return Commands{"browser": "yazi", "agent": "claude", "editor": "vim"}
}

func mustRender(t *testing.T, path string) string {
	t.Helper()
	p, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile(%s): %v", path, err)
	}
	kdl, err := Render(p, devCommands())
	if err != nil {
		t.Fatalf("Render(%s): %v", path, err)
	}
	return kdl
}

// The cockpit profile must render byte-for-byte the layout the origin setup ran
// by hand. This is the whole point of the port: if this drifts, the workspace
// someone gets from bothy is not the workspace the cheat sheet describes.
func TestCockpitMatchesOriginLayout(t *testing.T) {
	got := mustRender(t, "../../profiles/cockpit.toml")

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
	got := mustRender(t, "../../profiles/cockpit.toml")
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
	got := mustRender(t, "../../profiles/cockpit.toml")
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "plugin location=") && strings.Contains(line, "{") {
			t.Errorf("plugin node emitted on one line, which needs a trailing ';': %q", line)
		}
	}
}

// Zellij execs pane commands directly rather than through a shell, so a command
// with arguments has to become `command` plus `args`.
func TestCommandWithArgsIsSplit(t *testing.T) {
	p := Profile{
		Name:      "t",
		TabBar:    ptr(false),
		StatusBar: ptr(false),
		Rows:      []Row{{Panes: []Pane{{Slot: "agent"}}}},
	}
	got, err := Render(p, Commands{"agent": "claude --continue"})
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
	got := mustRender(t, "../../profiles/cockpit.toml")
	if !strings.Contains(got, `pane size="40%" name="side"`+"\n") {
		t.Errorf("the side pane should be a bare node with no body:\n%s", got)
	}
}

func TestAllShippedProfilesRender(t *testing.T) {
	for _, name := range []string{"cockpit", "editor", "minimal"} {
		p, err := LoadProfile("../../profiles/" + name + ".toml")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if p.Name != name {
			t.Errorf("%s: profile names itself %q", name, p.Name)
		}
		if _, err := Render(p, devCommands()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestMinimalDisablesTabBar(t *testing.T) {
	got := mustRender(t, "../../profiles/minimal.toml")
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
	p := Profile{Name: "t", Rows: []Row{{Panes: []Pane{{Slot: "browser"}}}}}
	if _, err := Render(p, Commands{}); err == nil {
		t.Fatal("expected an error when a pane's slot has no command")
	}
}

func TestValidateRejectsTwoFocusedPanes(t *testing.T) {
	p := Profile{Name: "t", Rows: []Row{{Panes: []Pane{
		{Slot: "agent", Focus: true},
		{Name: "side", Focus: true},
	}}}}
	if err := p.Validate(); err == nil {
		t.Fatal("expected an error when two panes are focused")
	}
}

func TestValidateRejectsEmptyProfile(t *testing.T) {
	if err := (Profile{Name: "t"}).Validate(); err == nil {
		t.Fatal("expected an error for a profile with no rows")
	}
}

// PaneCount is what the doctor compares against Zellij's resolved layout to
// prove the layout actually built as described.
func TestPaneCountExcludesBars(t *testing.T) {
	p, err := LoadProfile("../../profiles/cockpit.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.PaneCount(); got != 3 {
		t.Errorf("PaneCount() = %d, want 3 (browser, agent, side)", got)
	}
}

func TestSlotsListsWhatMustBeInstalled(t *testing.T) {
	p, err := LoadProfile("../../profiles/cockpit.toml")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(p.Slots(), ",")
	if got != "agent,browser" {
		t.Errorf("Slots() = %q, want \"agent,browser\"", got)
	}
}

func TestGeneratedLayoutSaysHowToChangeIt(t *testing.T) {
	got := mustRender(t, "../../profiles/cockpit.toml")
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

func ptr(b bool) *bool { return &b }
