package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Boxes maps an absolute project directory to the container that project runs
// in. Keyed by directory rather than by session name because a session name is
// the directory's base (internal/mux.SessionName), so ~/a/web and ~/b/web
// would share one entry and one project would open in another's box.
//
// An entry may be "", meaning the host was chosen. Presence is the answer, so
// this is one map and not a file per project: absence means never asked, and
// the two have to stay distinguishable or "don't ask again" cannot be honoured
// for anyone who answers "here".
type Boxes map[string]string

// BoxesPath is the project-to-box record inside a state directory.
func BoxesPath(stateDir string) string { return filepath.Join(stateDir, "boxes.json") }

// LoadBoxes reads the record. A missing file is an empty record, not an error.
func LoadBoxes(stateDir string) (Boxes, error) {
	src, err := os.ReadFile(BoxesPath(stateDir))
	if errors.Is(err, os.ErrNotExist) {
		return Boxes{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	b := Boxes{}
	if err := json.Unmarshal(src, &b); err != nil {
		return nil, fmt.Errorf("state: %s: %w", BoxesPath(stateDir), err)
	}
	return b, nil
}

// Save writes the record, dropping entries whose directory is gone: a project
// that no longer exists must not keep a claim on a box it is not using.
func (b Boxes) Save(stateDir string) error {
	for dir := range b {
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			delete(b, dir)
		}
	}
	return writeJSON(BoxesPath(stateDir), b)
}
