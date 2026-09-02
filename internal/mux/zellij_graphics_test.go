package mux

import "testing"

// A multiplexer bothy cannot interrogate is assumed incapable. Guessing "it is
// probably fine" produces the phantom-keypress bug: zellij below MinGraphics
// mangles its reply to yazi's capability query, and the reply is read as
// keystrokes.
func TestGraphicsAssumesIncapableWhenItCannotAsk(t *testing.T) {
	carries, reason := z.Graphics("definitely-not-a-real-binary")
	if carries {
		t.Error("an uninterrogable multiplexer must not be assumed capable")
	}
	if reason == "" {
		t.Error("no reason given")
	}
}

func TestMinGraphicsIsTheVersionThatFixedSizing(t *testing.T) {
	if MinGraphics.String() != "0.45.1" {
		t.Errorf("MinGraphics = %s; 0.45.0 added the protocol, 0.45.1 fixed sizing",
			MinGraphics)
	}
}
