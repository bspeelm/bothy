package main

import (
	"strings"
	"testing"
)

// A tag names one project's release. Applied to every tool it would ask GitHub
// for "v1.2.3" of eight unrelated projects, and the ones that happened to have
// such a tag would be silently re-pinned to it. The refusal has to come before
// anything is downloaded.
func TestLockRefusesATagWithoutATool(t *testing.T) {
	// cmdLock writes bothy.lock in the working directory. Somewhere disposable,
	// so a broken guard leaves a file in a temporary directory rather than in
	// the package -- which is how a stray lockfile got into this branch once.
	t.Chdir(t.TempDir())

	err := cmdLock([]string{"--tag", "v1.2.3"})
	if err == nil {
		t.Fatal("lock --tag with no --tool was accepted")
	}
	if !strings.Contains(err.Error(), "-tool") {
		t.Errorf("the refusal is %q, which does not say what to add", err)
	}
}

// `bothy outdated` told everyone to run `bothy lock`, which outside a
// checkout writes a stray bothy.lock into the current directory and changes
// nothing. The weekly job runs in the checkout and keeps quoting the lock line.
func TestOutdatedSendsAUserToUpgradeAndTheMaintainerToLock(t *testing.T) {
	if got := takeUpdates(true); !strings.Contains(got, "bothy lock") {
		t.Errorf("in a checkout: %q, want the lock step", got)
	}
	got := takeUpdates(false)
	if strings.Contains(got, "bothy lock") || !strings.Contains(got, "bothy upgrade") {
		t.Errorf("outside a checkout: %q, want 'bothy upgrade' and no lock step", got)
	}
}
