// Package state records what bothy installed.
//
// It is deliberately small. Revision 1 of PLAN.md needed a manifest of every
// config file bothy wrote, the backup it took first, and the hash of what it
// left behind, because uninstall had to reverse edits to files bothy did not
// own. Under isolation (ADR-009) none of that applies: every config bothy
// generates lives inside its own tree, so removing the tree removes them, and
// the only thing worth recording is which *binaries* were installed — those go
// in bothy's bin/, and what is worth recording about them is the version and
// checksum each was installed at.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ManifestVersion is bumped when the on-disk shape changes incompatibly.
const ManifestVersion = 2

// Manifest is <bothy>/state/manifest.json.
type Manifest struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	BothyVer  string    `json:"bothy_version"`
	// InstalledIn is the container bothy resolved its tools in, or "" for the
	// host. It matters because home is shared but PATH is not: tools found at
	// /usr/bin inside a container are simply absent on the host, so a launch
	// from the other side finds nothing. Recorded so `bothy` can go back to
	// where its tools actually are.
	InstalledIn string   `json:"installed_in,omitempty"`
	Binaries    []Binary `json:"binaries"`
}

// Binary is one tool bothy supplied because the system's was missing or too
// old. Source records which, so `bothy doctor` can report provenance rather
// than leaving you guessing which zellij you are running.
type Binary struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	// Source is "bothy" for a fetched binary, or the path it was found at when
	// the system's copy was good enough to use.
	Source string `json:"source"`
}

// ManifestPath is the manifest file inside a state directory.
func ManifestPath(stateDir string) string { return filepath.Join(stateDir, "manifest.json") }

// Load reads the manifest. A missing manifest is an empty one, not an error:
// that is simply a machine bothy has not installed on yet.
func Load(stateDir string) (*Manifest, error) {
	src, err := os.ReadFile(ManifestPath(stateDir))
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{Version: ManifestVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(src, &m); err != nil {
		return nil, fmt.Errorf("state: %s: %w", ManifestPath(stateDir), err)
	}
	if m.Version > ManifestVersion {
		return nil, fmt.Errorf("state: manifest version %d is newer than this bothy understands (%d)",
			m.Version, ManifestVersion)
	}
	return &m, nil
}

// Save writes the manifest atomically. A half-written manifest would leave
// uninstall unable to account for installed binaries.
func (m *Manifest) Save(stateDir string) error {
	m.Version = ManifestVersion
	m.UpdatedAt = time.Now().UTC()
	sort.Slice(m.Binaries, func(i, j int) bool { return m.Binaries[i].Name < m.Binaries[j].Name })

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	path := ManifestPath(stateDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	return os.Rename(tmp, path)
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
