package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/fetch"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy upgrade` -- how to upgrade this copy, and nothing else. It does not
// touch bothy's own binary: PLAN.md §11 rules out auto-updaters, and replacing
// a running binary from inside it leaves a half-written one on a bad network.

// Repo is bothy's own slug, for asking about its releases.
const Repo = "bspeelm/bothy"

func cmdUpgrade(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	p := platform.Detect()
	here := version()

	// The advice needs no network, so it prints even when the lookup fails.
	// Someone offline still wants to know the command.
	method, how := upgradeMethod(p, here)

	latest, lookupErr := fetch.LatestRelease(Repo)
	switch {
	case lookupErr != nil:
		fmt.Printf("bothy %s is installed. Could not reach GitHub to check for a newer one.\n", here)
	case fetch.IsSourceBuild(here):
		fmt.Printf("bothy %s is a source build, ahead of %s.\n", here, latest)
	case fetch.VersionFromTag(latest) == fetch.VersionFromTag(here):
		fmt.Printf("bothy %s is the latest release.\n", here)
	default:
		fmt.Printf("bothy %s is installed; %s is available.\n", here, fetch.VersionFromTag(latest))
	}

	fmt.Printf("\n%s\n  %s\n", method, how)
	fmt.Println("\nThen run 'bothy install' to regenerate the configs: the templates")
	fmt.Println("are compiled into the binary, and a launch does not re-render them.")

	if lookupErr != nil {
		return fmt.Errorf("upgrade: %w", lookupErr)
	}
	// Being out of date is a fact, not a failure -- see `bothy outdated`.
	return nil
}

// upgradeMethod works out how this copy was installed and what would replace
// it. os.Executable is the only evidence there is: nothing records the install
// method.
func upgradeMethod(p platform.Info, ver string) (string, string) {
	self, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
	}
	return describeInstall(self, p, ver)
}

// describeInstall is upgradeMethod's decision, separated so it can be tested
// against paths this machine does not have.
func describeInstall(self string, p platform.Info, ver string) (string, string) {
	switch {
	// EvalSymlinks resolves a cask into Caskroom, the one marker both brew prefixes share.
	case strings.Contains(self, "/Caskroom/"):
		return "This copy is at " + self + ", which Homebrew owns:",
			"brew upgrade --cask bothy"

	case strings.HasPrefix(self, "/usr/bin/"), strings.HasPrefix(self, "/usr/local/bin/"):
		// Which package manager is a question about the machine, not the path.
		if _, err := exec.LookPath("rpm"); err == nil {
			return "This copy is at " + self + ", which dnf owns:",
				"sudo dnf upgrade bothy"
		}
		if _, err := exec.LookPath("dpkg"); err == nil {
			return "This copy is at " + self + ", which dpkg owns. The .deb is a file,\n" +
					"not a repository, so apt cannot fetch a newer one:",
				"download the next .deb from https://github.com/" + Repo + "/releases/latest\n" +
					"  then: sudo apt install ./bothy_*.deb"
		}
		return "This copy is at " + self + ", which a package manager owns:",
			"use whatever installed it"

	case p.Home != "" && strings.HasPrefix(self, filepath.Join(p.Home, "go", "bin")):
		return "This copy is at " + self + ", so it came from go install:",
			"go install github.com/" + Repo + "/cmd/bothy@latest"

	case p.LocalBin != "" && strings.HasPrefix(self, p.LocalBin):
		// The script and `make install-binary` both land here and leave no
		// trace of which ran, but a source build says so in its version.
		if fetch.IsSourceBuild(ver) {
			return "This copy is at " + self + " and its version is a git describe,\n" +
					"so it was built from source:",
				"git pull && make install-binary"
		}
		return "This copy is at " + self + ", where the install script puts it:",
			"curl -fsSL https://raw.githubusercontent.com/" + Repo + "/main/bootstrap/install.sh | sh"
	}

	return "This copy is at " + self + ", which bothy does not recognise:",
		"replace it from https://github.com/" + Repo + "/releases/latest"
}
