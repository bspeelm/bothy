package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bothy-dev/bothy/internal/platform"
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

	// Look for these first, and regardless of whether there is anything left to
	// remove. A process running from a tree an *earlier* uninstall deleted is
	// exactly as stuck, and reporting nothing because there is nothing left to
	// delete would hide the problem the deletion caused.
	rep.Orphaned = StillRunning(p)

	dir := p.BothyDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		rep.Kept = append(rep.Kept, dir+" (nothing installed)")
		return rep, nil
	} else if err != nil {
		return nil, fmt.Errorf("uninstall: %s: %w", dir, err)
	}

	if !dryRun {
		if err := os.RemoveAll(dir); err != nil {
			return nil, fmt.Errorf("uninstall: %s: %w", dir, err)
		}
	}
	rep.Removed = append(rep.Removed, dir)

	if _, err := os.Stat(p.UserConfigDir()); err == nil {
		rep.Kept = append(rep.Kept,
			p.UserConfigDir()+" (your settings — delete it yourself if you want it gone)")
	}
	// The binary goes too. A running process can unlink its own executable on
	// Linux — the inode survives until it exits — so "remove it by hand" was
	// caution with nothing behind it, and an uninstaller that leaves itself
	// installed has not finished.
	//
	// Only at the path bothy's own bootstrap uses. A copy someone put in
	// /usr/local/bin, or one a package manager owns, is not bothy's to delete.
	self := filepath.Join(p.LocalBin, "bothy")
	if fileExists(self) {
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
	return rep, nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
