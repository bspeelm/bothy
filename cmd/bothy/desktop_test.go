package main

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

// #73. desktop.go wrote ~/.local/share/applications/bothy.desktop wherever it
// was run, with no platform check. On a Mac that is an inert file in a
// directory nothing reads, and bothy reported success — which is worse than
// declining, because nothing about it looks wrong.
func TestDesktopEntriesAreRefusedWhereTheyMeanNothing(t *testing.T) {
	for _, os := range []string{"darwin", "windows"} {
		err := desktopEntriesBelongHere(platform.Info{OS: os})
		if err == nil {
			t.Errorf("%s: a desktop entry was allowed where the platform has none", os)
			continue
		}
		// A refusal that does not say what to do instead is just a wall.
		if !strings.Contains(err.Error(), "instead") {
			t.Errorf("%s: the refusal offers no alternative: %v", os, err)
		}
	}
}

// A list of the platforms known to have no XDG rather than of the one known to
// have it, so a BSD is not refused for not being Linux.
func TestFreedesktopPlatformsAreStillAllowed(t *testing.T) {
	for _, os := range []string{"linux", "freebsd", "openbsd"} {
		if err := desktopEntriesBelongHere(platform.Info{OS: os}); err != nil {
			t.Errorf("%s: refused a platform that uses desktop entries: %v", os, err)
		}
	}
}
