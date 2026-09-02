// Package slots is what every provider declares about itself, across both of
// the dialects under slots/.
//
// A provider file used to say only how to get the program, never what it was,
// so slot membership lived in Go and the data could not be checked against it.
// One header serves both dialects because "what is this" is the same question
// whether bothy installs the program or only advises on it.
package slots

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bspeelm/bothy"
)

var dirs = []string{"slots/tools", "slots/advice"}

// Header is what a provider says about itself, embedded by both dialects.
type Header struct {
	Name string `toml:"name"`
	What string `toml:"what"`
	// Slot is the role this provider fills, empty for an extra that fills
	// none. One assigned to a slot it does not declare is rejected.
	Slot string `toml:"slot"`
	// Platforms restates a tool's [assets] keys; a test holds them together.
	Platforms []string `toml:"platforms"`
	// Provides is how doctor.Supplied tells a capability nothing verifies
	// from one nothing supplies.
	Provides []string `toml:"provides"`
}

// All reads every provider header, sorted by name.
func All() ([]Header, error) {
	var out []Header
	for _, dir := range dirs {
		entries, err := bothy.Slots.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("slots: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
				continue
			}
			src, err := bothy.Slots.ReadFile(dir + "/" + e.Name())
			if err != nil {
				return nil, fmt.Errorf("slots: %w", err)
			}
			var h Header
			if err := toml.Unmarshal(src, &h); err != nil {
				return nil, fmt.Errorf("slots: %s: %w", e.Name(), err)
			}
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get returns one provider's header. A name no file declares is not an error.
func Get(name string) (Header, bool) {
	all, err := All()
	if err != nil {
		return Header{}, false
	}
	for _, h := range all {
		if h.Name == name {
			return h, true
		}
	}
	return Header{}, false
}

// Fills names the providers declaring a slot.
func Fills(slot string) []string {
	all, err := All()
	if err != nil || slot == "" {
		return nil
	}
	var out []string
	for _, h := range all {
		if h.Slot == slot {
			out = append(out, h.Name)
		}
	}
	return out
}
