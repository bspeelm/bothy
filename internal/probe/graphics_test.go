package probe

import "testing"

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
	if !(Version{0, 42, 2}).Less(MinZellijGraphics) {
		t.Error("0.42.2 should sort before 0.45.1")
	}
	if (Version{0, 45, 1}).Less(MinZellijGraphics) {
		t.Error("0.45.1 is not less than itself")
	}
	if (Version{0, 46, 0}).Less(MinZellijGraphics) {
		t.Error("0.46.0 should not sort before 0.45.1")
	}
	// The trap this guards: a naive string compare puts "0.5" after "0.45".
	if !(Version{0, 5, 0}).Less(Version{0, 45, 0}) {
		t.Error("0.5.0 should sort before 0.45.0")
	}
}

// A terminal that cannot draw images makes the multiplexer version irrelevant.
func TestGraphicsNeedsACapableTerminal(t *testing.T) {
	g := CheckGraphics("zellij", "gnome-terminal")
	if g.Supported {
		t.Error("gnome-terminal cannot do Kitty graphics")
	}
	if g.Reason == "" {
		t.Error("an unsupported verdict must carry a reason")
	}
}

func TestGraphicsWithoutMultiplexer(t *testing.T) {
	g := CheckGraphics("", "ghostty")
	if !g.Supported {
		t.Errorf("bare ghostty should support previews: %s", g.Reason)
	}
}

// A multiplexer bothy cannot interrogate is assumed incapable. Guessing "it is
// probably fine" is what produces the phantom-keypress bug.
func TestGraphicsUnknownMuxIsAssumedIncapable(t *testing.T) {
	g := CheckGraphics("definitely-not-a-real-binary", "ghostty")
	if g.Supported {
		t.Error("an uninterrogable multiplexer must not be assumed capable")
	}
}
