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
