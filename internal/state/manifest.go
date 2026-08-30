// Package state records what bothy did to the machine.
//
// `uninstall` is driven entirely by this manifest and never guesses: if bothy
// did not record writing a file, it will not remove it. That is what makes
// PLAN.md's reversibility principle checkable rather than aspirational.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ManifestVersion is bumped when the on-disk shape changes incompatibly.
const ManifestVersion = 1

// Manifest is ~/.local/state/bothy/manifest.json.
type Manifest struct {
	Version     int          `json:"version"`
	UpdatedAt   time.Time    `json:"updated_at"`
	BothyVer    string       `json:"bothy_version"`
	Files       []File       `json:"files"`
	Binaries    []Binary     `json:"binaries"`
	GitSettings []GitSetting `json:"git_settings"`
}

// File is one config file bothy wrote.
type File struct {
	Path string `json:"path"`
	// SHA256 of the content bothy wrote. On uninstall a file whose hash still
	// matches is bothy's to delete; one that has drifted was edited by hand and
	// is left alone with a warning.
	SHA256 string `json:"sha256"`
	// Backup is where a pre-existing file was copied before being replaced.
	// Empty means there was nothing there before.
	Backup string `json:"backup,omitempty"`
}

// Binary is one tool bothy installed into ~/.local/bin.
type Binary struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// GitSetting records a `git config --global` key bothy changed, so uninstall
// can put back exactly what was there — including "it was not set at all",
// which is why HadPrevious exists separately from Previous.
type GitSetting struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Previous    string `json:"previous,omitempty"`
	HadPrevious bool   `json:"had_previous"`
}

// Dir is bothy's state directory.
func Dir(stateHome string) string { return filepath.Join(stateHome, "bothy") }

// ManifestPath is the manifest file.
func ManifestPath(stateHome string) string { return filepath.Join(Dir(stateHome), "manifest.json") }

// BackupDir is where this run's replaced files are kept.
func BackupDir(stateHome string, at time.Time) string {
	return filepath.Join(Dir(stateHome), "backup", at.UTC().Format("20060102-150405"))
}

// Load reads the manifest. A missing manifest is an empty one, not an error:
// that is simply a machine bothy has not touched yet.
func Load(stateHome string) (*Manifest, error) {
	src, err := os.ReadFile(ManifestPath(stateHome))
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{Version: ManifestVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(src, &m); err != nil {
		return nil, fmt.Errorf("state: %s: %w", ManifestPath(stateHome), err)
	}
	if m.Version > ManifestVersion {
		return nil, fmt.Errorf("state: manifest version %d is newer than this bothy understands (%d)",
			m.Version, ManifestVersion)
	}
	return &m, nil
}

// Save writes the manifest atomically. A half-written manifest would make
// uninstall unable to clean up, so the rename is not optional.
func (m *Manifest) Save(stateHome string) error {
	m.Version = ManifestVersion
	m.UpdatedAt = time.Now().UTC()
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	sort.Slice(m.Binaries, func(i, j int) bool { return m.Binaries[i].Name < m.Binaries[j].Name })

	if err := os.MkdirAll(Dir(stateHome), 0o755); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	path := ManifestPath(stateHome)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	return os.Rename(tmp, path)
}

// RecordFile adds or replaces a file entry, keeping the original backup path if
// one was already recorded — the first install is the one that saw the user's
// own file, and a later reinstall must not overwrite that memory with a backup
// of bothy's own output.
func (m *Manifest) RecordFile(f File) {
	for i := range m.Files {
		if m.Files[i].Path == f.Path {
			if f.Backup == "" {
				f.Backup = m.Files[i].Backup
			}
			m.Files[i] = f
			return
		}
	}
	m.Files = append(m.Files, f)
}

// RecordBinary adds or replaces a binary entry.
func (m *Manifest) RecordBinary(b Binary) {
	for i := range m.Binaries {
		if m.Binaries[i].Name == b.Name {
			m.Binaries[i] = b
			return
		}
	}
	m.Binaries = append(m.Binaries, b)
}

// RecordGitSetting adds a git setting entry, preserving the earliest recorded
// previous value for the same reason RecordFile preserves the first backup.
func (m *Manifest) RecordGitSetting(g GitSetting) {
	for i := range m.GitSettings {
		if m.GitSettings[i].Key == g.Key {
			g.Previous = m.GitSettings[i].Previous
			g.HadPrevious = m.GitSettings[i].HadPrevious
			m.GitSettings[i] = g
			return
		}
	}
	m.GitSettings = append(m.GitSettings, g)
}

// FileFor returns the recorded entry for a path.
func (m *Manifest) FileFor(path string) (File, bool) {
	for _, f := range m.Files {
		if f.Path == path {
			return f, true
		}
	}
	return File{}, false
}

// HashFile returns the hex SHA-256 of a file's contents.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashBytes returns the hex SHA-256 of a byte slice.
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
