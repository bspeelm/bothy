package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bspeelm/bothy/internal/platform"
)

// UninstallReport is what came off the machine.
type UninstallReport struct {
	Removed []string
	// Kept lists things deliberately left behind, with the reason.
	Kept []string
	// Orphaned lists processes that were still running binaries bothy
	// removed. Reported rather than killed: they are the user's sessions,
	// and killing someone's workspace to tidy up is not a trade uninstall
	// gets to make on their behalf.
	Orphaned []Running
}

// Uninstall removes bothy's tree.
//
// This is the whole of it, and that is the point of ADR-009. Revision 1 needed
// a manifest replay — restore each backup, re-set each git key, skip anything
// edited since — because it had written into files the user owned. bothy now
// writes only inside one directory, so removing that directory is exact by
// construction rather than by careful bookkeeping.
//
// Two things are deliberately left: ~/.config/bothy, which is the user's own
// settings and the thing worth keeping in git, and the bothy binary itself,
// which is running this code.
func Uninstall(p platform.Info, dryRun, keepBinary bool) (*UninstallReport, error) {
	rep := &UninstallReport{}

	// Look for these first, and regardless of what is left to remove. A process
	// running from a tree an *earlier* uninstall deleted is exactly as stuck,
	// and reporting nothing because there is nothing left to delete would hide
	// the problem the deletion caused.
	rep.Orphaned = StillRunning(p)

	// Each step is independent. An earlier version returned early when the tree
	// was already gone, which meant a second `bothy uninstall` skipped every
	// step after it — including removing the binary, so uninstalling twice left
	// bothy installed. Nothing here may short-circuit anything else.
	if err := removeTree(p, rep, dryRun); err != nil {
		return nil, err
	}
	removeBinary(p, rep, dryRun, keepBinary)
	noteUserConfig(p, rep)

	return rep, nil
}

// removeTree removes bothy's directory, if it is there.
func removeTree(p platform.Info, rep *UninstallReport, dryRun bool) error {
	dir := p.BothyDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // nothing to do, and nothing worth saying about it
	} else if err != nil {
		return fmt.Errorf("uninstall: %s: %w", dir, err)
	}
	if !dryRun {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("uninstall: %s: %w", dir, err)
		}
	}
	rep.Removed = append(rep.Removed, dir)
	return nil
}

// removeBinary removes bothy itself.
//
// A running process can unlink its own executable on Linux — the inode
// survives until it exits — so "remove it by hand" was caution with nothing
// behind it, and an uninstaller that leaves itself installed has not finished.
//
// Only the running binary, and only when it is at the path bothy's own
// bootstrap uses. A copy someone put in /usr/local/bin, or one a package
// manager owns, is not bothy's to delete.
//
// That was the intent all along; the code checked whether ~/.local/bin/bothy
// *existed* instead. So an rpm- or deb-installed bothy at /usr/bin, run as
// `bothy uninstall`, deleted someone's leftover script install that it had
// nothing to do with. os.Executable is the only thing that actually answers
// "is this me".
func removeBinary(p platform.Info, rep *UninstallReport, dryRun, keepBinary bool) {
	self := runningBinary()
	if self == "" || p.LocalBin == "" || filepath.Dir(self) != p.LocalBin {
		return
	}
	if !fileExists(self) {
		return
	}
	switch {
	case keepBinary:
		rep.Kept = append(rep.Kept, self+" (--keep-binary)")
	case dryRun:
		rep.Removed = append(rep.Removed, self)
	default:
		if err := os.Remove(self); err != nil {
			rep.Kept = append(rep.Kept, self+" (could not remove: "+err.Error()+")")
		} else {
			rep.Removed = append(rep.Removed, self)
		}
	}
}

// runningBinary is this process's own executable, symlinks resolved, or "" if
// it cannot be determined -- in which case removing nothing is the right
// answer.
// osExecutable is a variable so a test can say which binary is running. Under
// `go test` the real one is the test binary, which is never the thing being
// uninstalled.
var osExecutable = os.Executable

func runningBinary() string {
	self, err := osExecutable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		return resolved
	}
	return self
}

// noteUserConfig mentions the settings directory, which is the user's and is
// never removed — it is the one thing worth keeping in git.
func noteUserConfig(p platform.Info, rep *UninstallReport) {
	if _, err := os.Stat(p.UserConfigDir()); err == nil {
		rep.Kept = append(rep.Kept,
			p.UserConfigDir()+" (your settings — delete it yourself if you want it gone)")
	}
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
