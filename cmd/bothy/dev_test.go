package main

import (
	"testing"

	"github.com/bspeelm/bothy/internal/slots"
)

// The nesting guard and the agent list used to be two lists that disagreed:
// aider had a guard and no provider, codex and opencode had neither. Driving
// the guard from the providers is only worth anything if every variable a
// provider declares actually reaches it.
func TestEveryAgentsDetectVariableReachesTheGuard(t *testing.T) {
	all, err := slots.All()
	if err != nil {
		t.Fatal(err)
	}
	// The real environment already carries some of these -- this test suite
	// may well be running inside an agent -- so clear them all first.
	for _, pr := range all {
		for _, name := range pr.Detect {
			t.Setenv(name, "")
		}
	}
	agents := 0
	for _, pr := range all {
		if pr.Slot != "agent" {
			continue
		}
		agents++
		if len(pr.Detect) == 0 {
			t.Errorf("%s declares no detect variables, so bothy would open a "+
				"workspace inside it and start a second copy", pr.Name)
		}
		for _, name := range pr.Detect {
			t.Setenv(name, "1")
			got, nested := nestedAgent()
			if !nested || got != pr.Name {
				t.Errorf("%s=1 gave (%q, %v), want (%q, true)", name, got, nested, pr.Name)
			}
			t.Setenv(name, "")
		}
	}
	if agents < 3 {
		t.Errorf("found %d agent providers, expected at least claude-code, gemini-cli, aider", agents)
	}
}
