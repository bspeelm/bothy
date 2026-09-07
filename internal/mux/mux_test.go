package mux

import (
	"strings"
	"testing"
)

// PWD is inherited from wherever bothy was typed and chdir does not touch it,
// so a tool that trusts the variable over the syscall opens the wrong place.
// yazi does exactly that: measured in a workspace whose every process had the
// right cwd, with yazi showing the directory bothy was launched from.
func TestPWDNamesTheWorkspaceNotWhereBothyWasTyped(t *testing.T) {
	env := []string{"PATH=/bin", "PWD=/where/bothy/was/typed", "HOME=/home/me"}
	got := withPWD(env, "/the/workspace")

	var pwds []string
	for _, kv := range got {
		if strings.HasPrefix(kv, "PWD=") {
			pwds = append(pwds, kv)
		}
	}
	if len(pwds) != 1 {
		t.Fatalf("PWD appears %d times: %q — a duplicate leaves the old one in play", len(pwds), pwds)
	}
	if pwds[0] != "PWD=/the/workspace" {
		t.Errorf("PWD = %q, want the workspace directory", pwds[0])
	}
	if len(got) != len(env) {
		t.Errorf("withPWD() changed %d entries into %d; nothing else may move", len(env), len(got))
	}
	// Set even when it was absent, or the tool falls back to a stale inherited one.
	if out := withPWD([]string{"PATH=/bin"}, "/w"); len(out) != 2 || out[1] != "PWD=/w" {
		t.Errorf("withPWD() on an env with no PWD = %q", out)
	}
}
