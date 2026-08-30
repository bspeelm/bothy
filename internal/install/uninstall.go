package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/state"
)

// UninstallReport is what came off the machine.
type UninstallReport struct {
	Removed  []string
	Restored []string
	// Kept lists files that were left in place because they no longer match
	// what bothy wrote — someone edited them, and deleting an edit is the one
	// mistake with no undo.
	Kept []string
}

// Uninstall reverses an install using only what the manifest recorded.
//
// It never scans directories looking for things that look like bothy's. A file
// bothy did not record writing is not bothy's to delete, even if it sits in a
// directory bothy created — that restraint is what makes the reversibility
// promise in PLAN.md §0 something a filesystem diff can verify.
func Uninstall(p platform.Info, m *state.Manifest, dryRun bool) (*UninstallReport, error) {
	rep := &UninstallReport{}

	for _, f := range m.Files {
		// A recorded backup means there was a file here before bothy: put it
		// back rather than leaving a hole.
		if f.Backup != "" {
			if err := restore(f, dryRun); err != nil {
				return nil, err
			}
			rep.Restored = append(rep.Restored, f.Path)
			continue
		}

		current, err := os.ReadFile(f.Path)
		if os.IsNotExist(err) {
			continue // already gone
		}
		if err != nil {
			return nil, fmt.Errorf("uninstall: %s: %w", f.Path, err)
		}
		if f.SHA256 != "" && state.HashBytes(current) != f.SHA256 {
			rep.Kept = append(rep.Kept, f.Path)
			continue
		}
		if !dryRun {
			if err := os.Remove(f.Path); err != nil {
				return nil, fmt.Errorf("uninstall: %s: %w", f.Path, err)
			}
			pruneEmptyDirs(filepath.Dir(f.Path), p.Home)
		}
		rep.Removed = append(rep.Removed, f.Path)
	}

	// Note: reverting these can leave an empty ~/.gitconfig behind, because git
	// creates the file the first time a global setting is written. bothy does
	// not delete it — a user's ~/.gitconfig is theirs, and an empty file is a
	// smaller surprise than a missing one.
	for _, g := range m.GitSettings {
		if err := revertGitSetting(g, dryRun); err != nil {
			return nil, err
		}
	}

	for _, b := range m.Binaries {
		if err := removeBinary(b, dryRun); err != nil {
			return nil, err
		}
		rep.Removed = append(rep.Removed, b.Path)
	}

	if dryRun {
		return rep, nil
	}

	m.Files = nil
	m.Binaries = nil
	m.GitSettings = nil
	return rep, m.Save(p.StateDir)
}

func restore(f state.File, dryRun bool) error {
	body, err := os.ReadFile(f.Backup)
	if err != nil {
		return fmt.Errorf("uninstall: reading backup %s: %w", f.Backup, err)
	}
	if dryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	return os.WriteFile(f.Path, body, 0o644)
}

func removeBinary(b state.Binary, dryRun bool) error {
	// Only remove a binary that still is the one bothy installed; a hand-placed
	// replacement at the same path belongs to whoever put it there.
	if b.SHA256 != "" {
		got, err := state.HashFile(b.Path)
		if err != nil || got != b.SHA256 {
			return nil
		}
	}
	if dryRun {
		return nil
	}
	if err := os.Remove(b.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("uninstall: %s: %w", b.Path, err)
	}
	return nil
}

// revertGitSetting puts a global git setting back — including putting it back
// to *unset*, which is why the manifest records HadPrevious separately from an
// empty Previous.
func revertGitSetting(g state.GitSetting, dryRun bool) error {
	current, err := exec.Command("git", "config", "--global", "--get", g.Key).Output()
	if err != nil {
		return nil // already unset
	}
	if trim(string(current)) != g.Value {
		return nil // someone changed it since; leave it alone
	}
	if dryRun {
		return nil
	}
	if g.HadPrevious {
		return exec.Command("git", "config", "--global", g.Key, g.Previous).Run()
	}
	// --unset exits 5 when the key is already absent, which is not a failure.
	_ = exec.Command("git", "config", "--global", "--unset", g.Key).Run()
	return nil
}

// pruneEmptyDirs removes directories bothy emptied, stopping at home so a
// shared parent like ~/.config is never touched.
func pruneEmptyDirs(dir, home string) {
	for dir != home && dir != "/" && dir != "." {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
