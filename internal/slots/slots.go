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

// Provider is one file under slots/. A provider says what it is, then at most
// one way of being obtained -- [fetch] for what bothy installs, [advise] for
// what it only recommends. They stay apart rather than merging into one block
// because how a program is obtained is exactly what differs between them.
type Provider struct {
	Header
	// Detect names environment variables this program exports while running.
	// bothy refuses to open a workspace inside one, because the layout would
	// start a second copy in a pane of the first.
	Detect  []string `toml:"detect"`
	Fetch   *Fetch   `toml:"fetch"`
	Advise  *Advise  `toml:"advise"`
	Files   []File   `toml:"file"`
	Plugins []Plugin `toml:"plugin"`
}

// Fetch is a release bothy can download. The fields are the fetcher's, and
// this struct is the on-disk shape of tools.Tool.
type Fetch struct {
	Binary     string            `toml:"binary"`
	Extra      []string          `toml:"extra"`
	Repo       string            `toml:"repo"`
	MinVersion string            `toml:"min_version"`
	Reason     string            `toml:"reason"`
	Assets     map[string]string `toml:"assets"`
	Checksums  string            `toml:"checksums"`
}

// Advise is a program bothy names a command for and does not install.
type Advise struct {
	Binary  string            `toml:"binary"`
	Install map[string]string `toml:"install"`
	Avoid   []Avoid           `toml:"avoid"`
}

// Avoid is a repository known to cause a problem worse than the one it solves.
type Avoid struct {
	Repo    string   `toml:"repo"`
	Why     string   `toml:"why"`
	Distros []string `toml:"distros"`
}

// File is one config file bothy generates for this provider.
//
// Dest is relative to the config root, with {theme} interpolated, because the
// destination is not a convention: three of the seven generated files break
// any rule that fits the other four. When names a condition from a closed
// vocabulary -- an expression language would want a parser, and PLAN.md §13
// allows one dependency.
type File struct {
	Template string `toml:"template"`
	Dest     string `toml:"dest"`
	When     string `toml:"when"`
}

// Plugin is a package the provider's generated config depends on, pinned to a
// revision. Only yazi has any.
type Plugin struct {
	Name  string   `toml:"name"`
	Use   string   `toml:"use"`
	Rev   string   `toml:"rev"`
	Gives string   `toml:"gives"`
	Needs []string `toml:"needs"`
}

// Header is what a provider says about itself.
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

// All reads every provider, sorted by name so install and doctor output is
// stable between runs.
func All() ([]Provider, error) {
	entries, err := bothy.Slots.ReadDir("slots")
	if err != nil {
		return nil, fmt.Errorf("slots: %w", err)
	}
	var out []Provider
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
			continue
		}
		src, err := bothy.Slots.ReadFile("slots/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("slots: %w", err)
		}
		var pr Provider
		if err := toml.Unmarshal(src, &pr); err != nil {
			return nil, fmt.Errorf("slots: %s: %w", e.Name(), err)
		}
		if pr.Name == "" {
			return nil, fmt.Errorf("slots: %s declares no name", e.Name())
		}
		out = append(out, pr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get returns one provider's header. A name no file declares is not an error.
func Get(name string) (Provider, bool) {
	all, err := All()
	if err != nil {
		return Provider{}, false
	}
	for _, pr := range all {
		if pr.Name == name {
			return pr, true
		}
	}
	return Provider{}, false
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
