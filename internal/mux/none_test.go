package mux

import (
	"testing"

	"github.com/bspeelm/bothy/internal/layout"
)

// `slots.mux = "none"` was an accepted config that did not work: the launcher
// resolved the slot to a binary and reported "none is not installed". The
// backend runs the agent in this terminal instead.
func TestNoneRunsTheAgentRatherThanABinaryCalledNone(t *testing.T) {
	b, ok := For("none")
	if !ok {
		t.Fatal("no backend for none")
	}
	if got, _ := b.Preview(layout.Profile{}, nil); got != "" {
		t.Errorf("Preview = %q, want nothing: there are no panes", got)
	}
	if n, ok := b.Panes("", "", nil); ok || n != 0 {
		t.Errorf("Panes = %d, %v; none has no session to count", n, ok)
	}
	if got := b.SessionName("/some/project"); got != "" {
		t.Errorf("SessionName = %q, want none: there is no session", got)
	}
	if carries, _ := b.Graphics(""); !carries {
		t.Error("nothing is in the way, so previews are not blocked")
	}
}
