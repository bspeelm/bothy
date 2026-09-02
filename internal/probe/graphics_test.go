package probe

import (
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := map[string]Version{
		"zellij 0.45.1": {0, 45, 1},
		"zellij 0.42.2": {0, 42, 2},
		"Yazi 26.5.6 (VERGEN_IDEMPOTENT_OUTPUT ...)": {26, 5, 6},
		"25.07.1":              {25, 7, 1},
		"delta 0.19.2":         {0, 19, 2},
		"v1.3.1":               {1, 3, 1},
		"Ghostty 1.3.1-4.fc44": {1, 3, 1},
	}
	for in, want := range cases {
		got, err := ParseVersion(in)
		if err != nil {
			t.Errorf("ParseVersion(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseVersion(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseVersionRejectsGarbage(t *testing.T) {
	if _, err := ParseVersion("no numbers here"); err == nil {
		t.Error("expected an error")
	}
}

func TestVersionLess(t *testing.T) {
	if !(Version{0, 42, 2}).Less(Version{0, 45, 1}) {
		t.Error("0.42.2 should sort before 0.45.1")
	}
	if (Version{0, 45, 1}).Less(Version{0, 45, 1}) {
		t.Error("0.45.1 is not less than itself")
	}
	if (Version{0, 46, 0}).Less(Version{0, 45, 1}) {
		t.Error("0.46.0 should not sort before 0.45.1")
	}
	// The trap this guards: a naive string compare puts "0.5" after "0.45".
	if !(Version{0, 5, 0}).Less(Version{0, 45, 0}) {
		t.Error("0.5.0 should sort before 0.45.0")
	}
}

// A terminal that cannot draw images makes the multiplexer version irrelevant.
func TestGraphicsNeedsACapableTerminal(t *testing.T) {
	g := CheckGraphics("gnome-terminal", MuxGraphics{Carries: true})
	if g.Supported {
		t.Error("gnome-terminal cannot do Kitty graphics")
	}
	if g.Reason == "" {
		t.Error("an unsupported verdict must carry a reason")
	}
}

func TestGraphicsWithoutMultiplexer(t *testing.T) {
	g := CheckGraphics("ghostty", MuxGraphics{None: true})
	if !g.Supported {
		t.Errorf("bare ghostty should support previews: %s", g.Reason)
	}
}

// #71. graphicsTerminals treated "draws inline images" and "speaks the Kitty
// protocol" as one fact, so iTerm2 -- which has drawn images since before that
// protocol existed, by one of its own -- was reported as able to do neither.
//
// The distinction is not pedantry: zellij carries Kitty graphics and no other,
// so the answer for iTerm2 depends on whether a multiplexer is in the way.
func TestITerm2DrawsImagesButNotThroughZellij(t *testing.T) {
	direct := CheckGraphics("iterm.app", MuxGraphics{None: true})
	if !direct.Supported {
		t.Errorf("iTerm2 with nothing in the way = unsupported: %s", direct.Reason)
	}

	through := CheckGraphics("iterm.app", MuxGraphics{Carries: true, Reason: "zellij carries it"})
	if through.Supported {
		t.Error("iTerm2 through zellij reported as working; zellij passes Kitty graphics only")
	}
	if !strings.Contains(through.Reason, "the multiplexer does not carry") {
		t.Errorf("the reason does not explain the mismatch: %s", through.Reason)
	}
}

// A terminal nobody has written down is unknown, which is not the same as
// known to be incapable -- and the reason should not claim otherwise.
func TestAnUnknownTerminalIsNotClaimedIncapable(t *testing.T) {
	g := CheckGraphics("apple_terminal", MuxGraphics{Carries: true})
	if g.Supported {
		t.Error("an unlisted terminal was reported as drawing images")
	}
	if !strings.Contains(g.Reason, "not known to") {
		t.Errorf("the reason overclaims: %s", g.Reason)
	}
}
