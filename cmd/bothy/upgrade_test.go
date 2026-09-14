package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

// Only one of these paths exists on any given machine, so the decision is
// separated from os.Executable and tested against all of them.
func TestDescribeInstallNamesTheRightOwner(t *testing.T) {
	home := "/home/x"
	p := platform.Info{Home: home, LocalBin: filepath.Join(home, ".local", "bin")}

	// Which package manager is present is a fact about the runner, not the
	// path, so assert on whichever this machine actually has.
	pkgCmd := "use whatever installed it"
	if _, err := exec.LookPath("rpm"); err == nil {
		pkgCmd = "dnf"
	} else if _, err := exec.LookPath("dpkg"); err == nil {
		pkgCmd = "apt"
	}

	for _, tc := range []struct {
		name, self, ver string
		wantIn          string
	}{
		{"package manager", "/usr/bin/bothy", "0.1.5", pkgCmd},
		{"go install", filepath.Join(home, "go", "bin", "bothy"), "0.1.5", "go install"},
		{"install script", filepath.Join(home, ".local", "bin", "bothy"), "0.1.5", "install.sh"},
		{
			// Same directory as the script, told apart by the version shape.
			"source build", filepath.Join(home, ".local", "bin", "bothy"),
			"v0.1.5-3-gabc1234-dirty", "make install-binary",
		},
		// Both brew prefixes. The README advertises Homebrew and upgrade did
		// not recognise it, so a cask install was told it was unrecognised.
		{"cask, apple silicon", "/opt/homebrew/Caskroom/bothy/0.8.1/bothy", "0.8.1", "brew upgrade --cask"},
		{"cask, intel", "/usr/local/Caskroom/bothy/0.8.1/bothy", "0.8.1", "brew upgrade --cask"},
		{"somewhere else", "/opt/weird/bothy", "0.1.5", "releases/latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			where, how := describeInstall(tc.self, p, tc.ver)
			if !strings.Contains(where, tc.self) {
				t.Errorf("the description does not name the path: %s", where)
			}
			if !strings.Contains(how, tc.wantIn) {
				t.Errorf("command = %q, want it to mention %q", how, tc.wantIn)
			}
		})
	}
}

// A source build in ~/.local/bin must not be told to re-run the install
// script, which would replace it with a release and lose the local work.
func TestDescribeInstallDoesNotTellASourceBuildToRunTheScript(t *testing.T) {
	home := "/home/x"
	p := platform.Info{Home: home, LocalBin: filepath.Join(home, ".local", "bin")}
	_, how := describeInstall(filepath.Join(p.LocalBin, "bothy"), p, "v0.1.5-3-gabc1234")
	if strings.Contains(how, "install.sh") {
		t.Errorf("a source build was told to run the install script: %q", how)
	}
}

// `bothy upgrade` called every source build "ahead of" the latest release
// without comparing anything, so a build from before a release was told it led
// it. The shapes below are the ones `git describe --tags --always --dirty`
// actually produces, enumerated in TestIsSourceBuildKnowsWhichVersionsAreAhead.
func TestASourceBuildIsPlacedAgainstTheRelease(t *testing.T) {
	for _, c := range []struct {
		name, here, latest, want string
	}{
		// The reported case: three commits past v0.12.0, with v0.12.1 out.
		{"behind the release", "v0.12.0-3-g4eb1a87", "v0.12.1", "from before v0.12.1"},
		{"past the release", "v0.12.1-3-g4eb1a87", "v0.12.1", "past v0.12.1"},
		{"past a newer tag", "v0.13.0-2-gabc1234", "v0.12.1", "past v0.12.1"},
		{"dirty at the release", "v0.12.1-dirty", "v0.12.1", "past v0.12.1"},
		{"dirty behind it", "v0.12.0-dirty", "v0.12.1", "from before v0.12.1"},
		// Numeric, not lexical: 0.9.0 is behind 0.12.1 though "9" > "1".
		{"double-digit minor", "v0.9.0-1-gabc1234", "v0.12.1", "from before v0.12.1"},
		// Nothing to parse: it must not claim a direction either way.
		{"no tag to describe from", "abc1234", "v0.12.1", "is the latest release"},
		{"dev build", "dev", "v0.12.1", "is the latest release"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := sourceStanding(c.here, c.latest)
			if !strings.Contains(got, c.want) {
				t.Errorf("sourceStanding(%q, %q) = %q, want it to contain %q",
					c.here, c.latest, got, c.want)
			}
			// The old bug in one assertion: never "ahead" of something newer.
			if c.want == "from before v0.12.1" && strings.Contains(got, "ahead") {
				t.Errorf("sourceStanding(%q, %q) = %q; it is behind", c.here, c.latest, got)
			}
		})
	}
}
