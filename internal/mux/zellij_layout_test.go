package mux

import (
	"os"
	"path/filepath"
	"testing"
)

// z is the backend under test.
var z = Zellij{}

// The fixtures in testdata are real resolved layouts, taken off a working
// machine and scrubbed of its paths. Inventing a fixture here would
// have tested my idea of the format rather than the format.
//
// Each was produced by the origin cockpit layout: yazi on top, an agent and a
// shell below. Three content panes.
func TestCountContentPanesOnRealResolvedLayouts(t *testing.T) {
	files, err := filepath.Glob("testdata/resolved-*.kdl")
	if err != nil || len(files) == 0 {
		t.Fatal("no fixtures")
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := z.countPanes(string(body))
		if !ok {
			t.Errorf("%s: could not read the layout's shape", f)
			continue
		}
		if got != 3 {
			t.Errorf("%s: counted %d panes, want 3", filepath.Base(f), got)
		}
	}
}

// Three things in a resolved layout would corrupt a naive count. Each is worth
// asserting separately, because each was found by reading a real file rather
// than by reasoning about the format.
func TestCountIgnoresTheNewTabTemplate(t *testing.T) {
	// The whole layout is repeated under new_tab_template; counting both
	// doubles everything.
	body, err := os.ReadFile("testdata/resolved-1.kdl")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := z.countPanes(string(body)); got == 6 {
		t.Error("counted the new_tab_template copy as well as the real tab")
	}
}

func TestCountIgnoresFloatingAndPluginPanes(t *testing.T) {
	kdl := `layout {
    tab name="Tab #1" focus=true {
        pane size=1 borderless=true {
            plugin location="zellij:tab-bar"
        }
        pane command="yazi" size="50%" {
            start_suspended true
        }
        pane size="50%" split_direction="vertical" {
            pane command="claude" name="agent"
            pane name="side" size="40%"
        }
        pane size=2 borderless=true {
            plugin location="zellij:status-bar"
        }
        floating_panes {
            pane name="About Zellij" {
                plugin location="zellij:about"
            }
        }
    }
}
`
	got, ok := z.countPanes(kdl)
	if !ok {
		t.Fatal("could not read the shape")
	}
	if got != 3 {
		t.Errorf("counted %d, want 3 — plugin bars and the floating about-pane are not content", got)
	}
}

// A format this code no longer understands must skip, not report a number
// derived from guesswork.
func TestCountRefusesRatherThanGuess(t *testing.T) {
	if _, ok := z.countPanes("something else entirely"); ok {
		t.Error("claimed to understand a layout it did not")
	}
}

// list-panes.json is a real reply from `zellij action list-panes -a -j`,
// scrubbed of its paths. Every field PaneRef reads is named by zellij rather
// than by bothy, and a rename decodes silently: json.Unmarshal succeeds and
// leaves the field zero. So the reply is pinned, and the pin is what fails when
// the shape moves.
//
// Measured while writing it: plugin panes omit pane_command and pane_cwd
// entirely, and terminal_command is null for a pane running the default shell
// while pane_command carries "/bin/bash" -- which is why PaneRef reads
// pane_command and not the other one.
func TestTheseAreThePaneFieldsZellijSends(t *testing.T) {
	body, err := os.ReadFile("testdata/list-panes.json")
	if err != nil {
		t.Fatal(err)
	}
	panes, ok := decodePanes(body)
	if !ok {
		t.Fatal("the recorded reply no longer decodes")
	}
	if len(panes) != 6 {
		t.Fatalf("decoded %d panes, want 6", len(panes))
	}

	var agent PaneRef
	for _, p := range panes {
		if p.Title == "agent" {
			agent = p
		}
	}
	// Each of these is a separate json tag, and each has a caller: the tower
	// matches on Command, addresses with ID and Plugin, labels with Dir, skips
	// on Exited and reads Fullscreen before toggling it.
	if agent.Command != "claude" {
		t.Errorf("pane_command decoded as %q; the tower finds the agent by it", agent.Command)
	}
	if agent.Dir == "" {
		t.Error("pane_cwd decoded empty; the mirror label comes from it")
	}
	if agent.Addr() != "terminal_1" {
		t.Errorf("id/is_plugin decoded to %q, want terminal_1", agent.Addr())
	}
	if agent.Exited {
		t.Error("exited decoded true for a running agent")
	}

	// A shell pane is the case that made pane_command the right field.
	shell := false
	for _, p := range panes {
		if p.Command == "/bin/bash" {
			shell = true
		}
	}
	if !shell {
		t.Error("no pane reports /bin/bash; terminal_command is null for a default shell")
	}

	// Plugin panes carry a title and no command, which is what makes a title
	// the thing legible() looks for.
	plugins := 0
	for _, p := range panes {
		if p.Plugin {
			plugins++
			if p.Title == "" {
				t.Error("a plugin pane decoded with no title")
			}
		}
	}
	if plugins != 3 {
		t.Errorf("decoded %d plugin panes, want 3", plugins)
	}
}

// A rename upstream is not a decode error, so nothing but a check on the result
// separates "the session has no agent" from "bothy cannot read this zellij".
func TestAReplyWithNoLegiblePaneIsNotAnEmptySession(t *testing.T) {
	// The same six panes with every key renamed, as a version bump would.
	renamed := []byte(`[{"id":0,"is_plugin":false,"cmd":"claude","name":"agent"}]`)
	if _, ok := decodePanes(renamed); ok {
		t.Error("a reply bothy cannot read was reported as readable")
	}
	// An empty list is a real answer: a session can have no panes to report.
	if _, ok := decodePanes([]byte(`[]`)); !ok {
		t.Error("an empty pane list should be an answer, not a failure")
	}
	if _, ok := decodePanes([]byte(`not json`)); ok {
		t.Error("malformed JSON was reported as readable")
	}
}
