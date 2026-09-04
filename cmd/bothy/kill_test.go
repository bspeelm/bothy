package main

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/mux"
)

// A Zellij with the four answers killPlan asks for replaced. Embedding keeps
// the rest of the interface real, so a method added to Backend later cannot
// silently start being answered by a stub.
type killStub struct {
	mux.Zellij
	current string
	live    []string
	stopped []string
}

func (k killStub) CurrentSession() string            { return k.current }
func (k killStub) Live(string, []string) []string    { return k.live }
func (k killStub) Stopped(string, []string) []string { return k.stopped }

// Every refusal has to name the command that does what was being asked, or it
// is a dead end: three of these are "you want a different verb".
func TestKillRefusesAndSaysWhatToRunInstead(t *testing.T) {
	stub := killStub{
		current: "bothy-here",
		live:    []string{"bothy-here", "bothy-other"},
		stopped: []string{"bothy-gone"},
	}
	for _, tc := range []struct {
		name, session, wants string
	}{
		{"the one you are in", "bothy-here", "Ctrl-q"},
		{"already stopped", "bothy-gone", "bothy ls --prune"},
		{"never existed", "bothy-nope", "bothy ls"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := killPlan(stub, "", nil, "/tmp/x", []string{tc.session})
			if err == nil {
				t.Fatalf("killed %s", tc.session)
			}
			if !strings.Contains(err.Error(), tc.wants) {
				t.Errorf("the refusal is %q, which never mentions %q", err, tc.wants)
			}
		})
	}
}

// A live session that is not this one is what the command is for.
func TestKillEndsALiveSessionThatIsNotYours(t *testing.T) {
	stub := killStub{current: "bothy-here", live: []string{"bothy-here", "bothy-other"}}
	got, err := killPlan(stub, "", nil, "/tmp/x", []string{"bothy-other"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "bothy-other" {
		t.Errorf("planned to end %q, want bothy-other", got)
	}
}

// No name means this directory's session, the way `bothy attach` reads it.
func TestKillWithNoNameMeansThisDirectory(t *testing.T) {
	stub := killStub{live: []string{"bothy-x"}}
	got, err := killPlan(stub, "", nil, "/home/me/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "bothy-x" {
		t.Errorf("planned to end %q, want the session for this directory", got)
	}
}
