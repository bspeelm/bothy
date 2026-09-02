package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/confine"
	"github.com/bspeelm/bothy/internal/platform"
)

// UninstallReport is what came off the machine.
type UninstallReport struct {
	Removed []string
	// Kept lists things deliberately left behind, with the reason.
	Kept []string
	// Orphaned lists processes still running binaries bothy removed.
	// Reported rather than killed: they are the user's sessions.
	Orphaned []Running
}

// Uninstall removes bothy's tree, which under ADR-009 is all of it: removing
// one directory is exact by construction rather than by bookkeeping. Two
// things are left deliberately -- ~/.config/bothy, which is the user's own
// settings, and the binary, which is running this code.
func Uninstall(p platform.Info, dryRun, keepBinary bool) (*UninstallReport, error) {
	rep := &UninstallReport{}

	// First, and regardless of what is left to remove: a process running from
	// a tree some earlier uninstall deleted is exactly as stuck.
	rep.Orphaned = StillRunning(p)

	// Each step is independent and none may short-circuit the rest: a tree
	// that is already gone must not stop the binary being removed, or a second
	// uninstall leaves bothy installed.
	if err := removeTree(p, rep, dryRun); err != nil {
		return nil, err
	}
	removeBinary(p, rep, dryRun, keepBinary)
	noteUserConfig(p, rep)
	noteConfineImage(p, rep)

	return rep, nil
}

// removeTree removes bothy's directory, if it is there.
func removeTree(p platform.Info, rep *UninstallReport, dryRun bool) error {
	dir := p.BothyDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // nothing to do
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

// removeBinary removes bothy itself. A running process can unlink its own
// executable on Linux -- the inode survives until it exits. Only the running
// binary, and only at the path the bootstrap uses: a copy a package manager
// owns is not bothy's to delete, and os.Executable answers "is this me".
func removeBinary(p platform.Info, rep *UninstallReport, dryRun, keepBinary bool) {
	self := runningBinary()
	if self == "" || p.LocalBin == "" || !sameDir(filepath.Dir(self), p.LocalBin) {
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
// it cannot be determined — in which case removing nothing is right.
// osExecutable is a variable so a test can say which binary is running: under
// `go test` the real one is the test binary.
var osExecutable = os.Executable

// sameDir compares two directories through whatever symlinks stand in the way.
// runningBinary resolves them and LocalBin does not, so on macOS -- home under
// /var reached through /private/var -- the two name one directory in two ways
// and never match, and the binary survives an uninstall reporting success.
func sameDir(a, b string) bool {
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	return a == b
}

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
// noteConfineImage names the container image. Half a gigabyte, removing the
// tree does not touch it, and bothy caused it to exist -- so it is named on the
// way out, like the settings and the desktop entry.
func noteConfineImage(p platform.Info, rep *UninstallReport) {
	runtime, err := confine.Runtime(p)
	if err != nil || !confine.ImageBuilt(runtime, confine.DefaultImage) {
		return
	}
	rep.Kept = append(rep.Kept, fmt.Sprintf("the %s container image — remove it with: %s rmi %s",
		confine.DefaultImage, strings.Join(runtime, " "), confine.DefaultImage))
}

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
