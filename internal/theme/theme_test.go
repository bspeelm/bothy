package theme

import (
	"strings"
	"testing"
)

// The palette a terminal is painted with has to be the one a spawned window
// would have been started with, or the same workspace looks different
// depending on how it was opened.
func TestPaintCarriesEveryColourTheProvidersWrite(t *testing.T) {
	p := Open()
	got := p.Paint()

	for _, want := range []string{
		"\033]10;" + p.Fg + "\a",      // foreground
		"\033]11;" + p.Bg + "\a",      // background
		"\033]12;" + p.Comment + "\a", // cursor
		"\033]4;0;" + p.Bg + "\a",     // ansi 0, as the wezterm and ghostty templates map it
		"\033]4;4;" + p.Purple + "\a", // blue is Dracula's purple, as in those templates
		"\033]4;8;" + p.Comment + "\a",
		"\033]4;15;" + p.Fg + "\a",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Paint() is missing %q", want)
		}
	}
	if n := strings.Count(got, "\033]4;"); n != 16 {
		t.Errorf("Paint() sets %d palette entries, want 16", n)
	}
}

// Painting without putting it back leaves somebody's terminal recoloured after
// they detached from a workspace they are no longer looking at.
func TestUnpaintResetsEverythingPaintSet(t *testing.T) {
	for _, want := range []string{"\033]104\a", "\033]110\a", "\033]111\a", "\033]112\a"} {
		if !strings.Contains(Unpaint(), want) {
			t.Errorf("Unpaint() is missing %q", want)
		}
	}
}
