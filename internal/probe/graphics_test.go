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

// bothy recorded wezterm as speaking the Kitty protocol, which is true of the
// terminal and false of what Yazi does there. `ya env` inside a real session
// settles it: brand WezTerm, kgp false, sixel false, mux None, chosen driver
// Iip -- the iTerm2 protocol, which zellij does not carry. So the verdict is
// the same as iTerm2's: images with nothing in the way, none through a
// multiplexer. Claiming otherwise promises previews that never arrive.
func TestWezTermDrawsImagesButNotThroughAMultiplexer(t *testing.T) {
	direct := CheckGraphics("wezterm", MuxGraphics{None: true})
	if !direct.Supported {
		t.Errorf("bare wezterm = unsupported: %s", direct.Reason)
	}
	if strings.Contains(direct.Reason, "Kitty") {
		t.Errorf("the reason claims Kitty, which Yazi does not use here: %s", direct.Reason)
	}

	through := CheckGraphics("wezterm", MuxGraphics{
		Carries: true,
		Reason:  "zellij 0.45.1 implements the Kitty graphics protocol",
	})
	if through.Supported {
		t.Error("wezterm through a multiplexer reported as working; Yazi sends iTerm2 and zellij drops it")
	}
	if !strings.Contains(through.Reason, "the multiplexer does not carry") {
		t.Errorf("the reason does not explain the mismatch: %s", through.Reason)
	}
}

// The measurement that turned previews off everywhere. `ya env` in a live
// session on ghostty 1.3.1, zellij 0.45.1, yazi 26.9.1: zellij answers the
// version query as Zellij(4501), Yazi overrides its env-derived Ghostty brand
// with Zellij, and the driver it picks is KgpOld -- which zellij renders as
// garbage. A terminal that speaks Kitty perfectly still gets no previews
// through a multiplexer, so the verdict cannot be read off the terminal alone.
func TestGhosttyGetsNoPreviewsThroughAMultiplexer(t *testing.T) {
	for _, term := range []string{"ghostty", "kitty"} {
		direct := CheckGraphics(term, MuxGraphics{None: true})
		if !direct.Supported {
			t.Errorf("bare %s = unsupported: %s", term, direct.Reason)
		}

		through := CheckGraphics(term, MuxGraphics{
			Carries: true,
			Reason:  "zellij 0.45.1 implements the Kitty graphics protocol",
		})
		if through.Supported {
			t.Errorf("%s through a multiplexer reported as working; Yazi sends the old "+
				"variant and zellij renders it as garbage", term)
		}
		if !strings.Contains(through.Reason, "Yazi sends") {
			t.Errorf("%s: the reason blames the multiplexer's reach rather than Yazi's "+
				"choice of protocol: %s", term, through.Reason)
		}
	}
}

// What opening a window buys. zellij asks every terminal for the Kitty
// keyboard protocol -- measured under a pty, it writes CSI > 1 u under
// xterm-ghostty and xterm-256color alike -- so the terminal's answer decides
// whether Shift+Enter reaches an agent.
func TestKeyboardIsWhatAWindowIsFor(t *testing.T) {
	for _, term := range []string{"ghostty", "kitty", "wezterm", "iterm.app"} {
		if k := Keyboard(term); !k.Supported {
			t.Errorf("Keyboard(%q) = unsupported: %s", term, k.Reason)
		}
	}
	k := Keyboard("gnome-terminal")
	if k.Supported {
		t.Error("gnome-terminal reported as carrying the keyboard protocol")
	}
	if !strings.Contains(k.Reason, "not known to") {
		t.Errorf("the reason overclaims about an unlisted terminal: %s", k.Reason)
	}
	if !strings.Contains(Keyboard("").Reason, "this terminal") {
		t.Error("an unnamed terminal should still produce a readable reason")
	}
}

// `bothy` opened a window when the terminal could not draw images and now does
// it when the terminal drops keys (ADR-056). No terminal's behaviour changed,
// because the two sets coincide. If one is ever added to a column and not the
// other, that stops being true and this says so rather than a user finding out.
func TestTheTwoCapabilitiesStillCoincide(t *testing.T) {
	for name, e := range knownEmulators {
		if e.direct != "" && !e.keyboard {
			t.Errorf("%s draws images but is not recorded as carrying the keyboard "+
				"protocol: `bothy` now runs there where it used to open a window", name)
		}
		if e.keyboard && e.direct == "" {
			t.Errorf("%s carries the keyboard protocol but draws no images: `bothy` now "+
				"opens no window there where it used to", name)
		}
	}
}
