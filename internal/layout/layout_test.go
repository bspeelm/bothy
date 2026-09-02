package layout

import (
	"strings"
	"testing"
)

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

func ptr(b bool) *bool { return &b }

// The shipped profiles are embedded, and reaching them through a file loader
// meant writing each one to a temp file purely to read it back. ParseProfile
// is what the file loader now wraps.
func TestParseProfileReadsBytesDirectly(t *testing.T) {
	src := []byte(`
name = "test"
description = "a profile"
[[rows]]
size = "50%"
[[rows.panes]]
name = "browser"
slot = "browser"
`)
	p, err := ParseProfile(src, "profiles/test.toml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "test" || len(p.Rows) != 1 || len(p.Rows[0].Panes) != 1 {
		t.Errorf("parsed %+v", p)
	}
}

// The name is what an error calls the source, and a shipped profile has no
// path to report -- which is the reason it is a parameter rather than a path.
func TestParseProfileNamesTheSourceInErrors(t *testing.T) {
	_, err := ParseProfile([]byte("name = [unclosed"), "profiles/cockpit.toml")
	if err == nil {
		t.Fatal("invalid TOML parsed without complaint")
	}
	if !strings.Contains(err.Error(), "profiles/cockpit.toml") {
		t.Errorf("the error does not say what failed to parse: %v", err)
	}
}

// Validation still runs -- it is the reason LoadProfile returned two values.
func TestParseProfileValidates(t *testing.T) {
	if _, err := ParseProfile([]byte(`name = "x"`), "t.toml"); err == nil {
		t.Error("a profile with no rows was accepted")
	}
}
