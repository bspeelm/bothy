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
