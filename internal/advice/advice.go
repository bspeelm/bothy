// Package advice holds install instructions for the things bothy will not
// install itself: Ghostty, which ships no release binaries and needs root
// (and a reboot on image-based hosts), the agent, whose install methods and
// credentials are not bothy's business, and editors, which are personal.
// bothy names the right command for the machine and warns about repositories
// known to cause problems.
package advice

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/platform"
)

// Advice is one thing bothy advises on but does not install.
type Advice struct {
	Name    string            `toml:"name"`
	What    string            `toml:"what"`
	Binary  string            `toml:"binary"`
	Install map[string]string `toml:"install"`
	Avoid   []Avoid           `toml:"avoid"`
}

// Avoid is a repository known to cause a problem worse than the one it solves.
type Avoid struct {
	Repo string `toml:"repo"`
	Why  string `toml:"why"`
	// Distros limits the warning to the distributions it is about. Empty
	// means everywhere. Without it bothy told Ubuntu users to avoid a Copr,
	// which is advice about a repository they cannot enable.
	Distros []string `toml:"distros"`
}

// Get loads advice by name.
func Get(name string) (Advice, error) {
	src, err := bothy.Slots.ReadFile(filepath.Join("slots", "advice", name+".toml"))
	if err != nil {
		return Advice{}, fmt.Errorf("advice: no entry for %q", name)
	}
	var a Advice
	if err := toml.Unmarshal(src, &a); err != nil {
		return Advice{}, fmt.Errorf("advice: %s: %w", name, err)
	}
	return a, nil
}

// Command returns the install command for a machine.
//
// An image-based host is looked up first: on those the command differs, needs a
// reboot, and getting it wrong is expensive — exactly the case a generic
// "install ghostty" leaves someone to work out alone.
func (a Advice) Command(p platform.Info) string {
	// DistroLike after DistroID: a derivative gets its own entry if one
	// exists, and its parent's otherwise.
	keys := []string{p.DistroID, p.DistroLike, p.OS, "default"}
	if p.Immutable {
		keys = append([]string{p.DistroID + "-ostree"}, keys...)
	}
	for _, key := range keys {
		if cmd, ok := a.Install[key]; ok && cmd != "" {
			return cmd
		}
	}
	return "see the project's own install instructions"
}

// Warnings renders the repositories to stay away from, keeping only those
// that apply to this machine.
func (a Advice) Warnings(p platform.Info) string {
	var out []string
	for _, w := range a.Avoid {
		if !w.appliesTo(p) {
			continue
		}
		out = append(out, "avoid "+w.Repo+" — "+w.Why)
	}
	return strings.Join(out, "; ")
}

func (w Avoid) appliesTo(p platform.Info) bool {
	if len(w.Distros) == 0 {
		return true
	}
	for _, d := range w.Distros {
		if d == p.DistroID || d == p.DistroLike || d == p.OS {
			return true
		}
	}
	return false
}
