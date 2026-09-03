// Package layout turns a profile into a Zellij layout.
//
// Users write profiles; bothy writes KDL, because Zellij's layout language has
// two traps (docs/history/origin-cheatsheet.md §3):
//
//  1. split_direction="vertical" produces *columns*, not rows — the opposite of
//     what the word suggests. A profile says "columns" and means columns.
//  2. A `{ plugin location="…" }` body written on one line needs a trailing
//     semicolon or Zellij fails to deserialise the node. This renderer always
//     emits plugin bodies across multiple lines, so the question never arises.
package layout

import (
	"fmt"
	"os"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// Profile is one workspace arrangement, read from profiles/<name>.toml or
// ~/.config/bothy/profiles/<name>.toml. Deliberately two levels deep -- a
// stack of rows, each one pane or a set of columns -- because every layout in
// PLAN.md fits, and a nested tree buys expressiveness nobody asked for at the
// cost of a renderer that is harder to keep correct.
type Profile struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`

	// TabBar and StatusBar are fixed line counts, not percentages, so they are
	// flags rather than rows.
	TabBar    *bool `toml:"tab_bar"`
	StatusBar *bool `toml:"status_bar"`

	Rows []Row `toml:"rows"`
}

// Row is a horizontal band of the window.
type Row struct {
	// Size is a percentage ("50%") or a fixed line count ("3"). Empty means
	// "share what is left" — at most one row per profile should say nothing.
	Size  string `toml:"size"`
	Panes []Pane `toml:"panes"`
}

// Pane is one region. Either Slot or Command identifies what runs in it; a
// pane with neither is a plain shell, which the side pane wants.
type Pane struct {
	// Slot is resolved to a command by the caller ("browser" -> "yazi").
	Slot string `toml:"slot"`
	// Command overrides Slot with a literal command line.
	Command string `toml:"command"`
	Name    string `toml:"name"`
	Size    string `toml:"size"`
	Focus   bool   `toml:"focus"`
}

// Commands maps slot names to the command that runs in them.
type Commands map[string]string

// LoadProfile reads a profile from a TOML file.
func LoadProfile(path string) (Profile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("layout: %w", err)
	}
	return ParseProfile(src, path)
}

// ParseProfile reads a profile from bytes. name is what an error calls the
// source -- a path, or "profiles/cockpit.toml" for a shipped profile, which
// has none. Separate from LoadProfile because the shipped profiles are
// embedded, and a file loader meant writing them to a temp file to read back.
func ParseProfile(src []byte, name string) (Profile, error) {
	var p Profile
	if err := toml.Unmarshal(src, &p); err != nil {
		return Profile{}, fmt.Errorf("layout: %s: %w", name, err)
	}
	return p, p.Validate()
}

// Validate rejects profiles that would render into a layout Zellij accepts but
// nobody wants — an empty screen, or a focus fight between panes.
func (p Profile) Validate() error {
	if len(p.Rows) == 0 {
		return fmt.Errorf("layout: profile %q has no rows", p.Name)
	}
	focused := 0
	for ri, r := range p.Rows {
		if len(r.Panes) == 0 {
			return fmt.Errorf("layout: profile %q row %d has no panes", p.Name, ri+1)
		}
		for _, pane := range r.Panes {
			if pane.Focus {
				focused++
			}
		}
	}
	if focused > 1 {
		return fmt.Errorf("layout: profile %q focuses %d panes; exactly one may be focused", p.Name, focused)
	}
	return nil
}

// Slots lists the slots a profile needs filled, so install can refuse to write
// a layout whose panes would launch commands that were never installed.
func (p Profile) Slots() []string {
	seen := map[string]bool{}
	for _, r := range p.Rows {
		for _, pane := range r.Panes {
			if pane.Slot != "" {
				seen[pane.Slot] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// PaneCount is the number of content panes, excluding Zellij's own bars.
// The doctor compares this against the layout Zellij actually resolved, which
// is the only proof that the layout built the way the profile described.
func (p Profile) PaneCount() int {
	n := 0
	for _, r := range p.Rows {
		n += len(r.Panes)
	}
	return n
}
