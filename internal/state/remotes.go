package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Remote is a machine a workspace opens against, and where the work lives on
// it. Identity is a path to a key, never a key: bothy chooses which one ssh
// should offer and has no business holding the thing itself.
type Remote struct {
	Dir      string `json:"dir"`
	Identity string `json:"identity,omitempty"`
}

// Remotes is what bothy has learned about the machines it has connected to,
// keyed by the host string as typed.
//
// Keyed by host and not by project, which is where this differs from Boxes: a
// box belongs to a project, so that record is keyed by directory, but a remote
// belongs to a machine and usually has no local counterpart at all.
// `bothy connect myserver` has to work from any directory, including one that
// has nothing to do with it.
type Remotes map[string]Remote

// RemotesPath is the host record inside a state directory.
func RemotesPath(stateDir string) string { return filepath.Join(stateDir, "remotes.json") }

// LoadRemotes reads the record. A missing file is an empty record, not an
// error: that is a machine that has not connected anywhere yet.
func LoadRemotes(stateDir string) (Remotes, error) {
	src, err := os.ReadFile(RemotesPath(stateDir))
	if errors.Is(err, os.ErrNotExist) {
		return Remotes{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	r := Remotes{}
	if err := json.Unmarshal(src, &r); err != nil {
		return nil, fmt.Errorf("state: %s: %w", RemotesPath(stateDir), err)
	}
	return r, nil
}

// Save writes the record.
//
// Nothing is pruned, which is the other difference from Boxes. That record
// drops entries whose directory is gone, because the key is a local path this
// machine can stat. A host is not a path here, and Dir is a path on somebody
// else's machine: stat-ing it locally would delete a good record for a
// directory that is simply elsewhere.
func (r Remotes) Save(stateDir string) error {
	return writeJSON(RemotesPath(stateDir), r)
}
