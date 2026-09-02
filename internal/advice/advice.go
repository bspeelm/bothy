// Package advice holds install instructions for what bothy will not install
// itself: Ghostty, which ships no binaries and needs root; the agent, whose
// credentials are not bothy's business; and editors, which are personal. It
// names the right command for the machine and warns about repositories known
// to cause problems.
package advice

import (
	"fmt"
	"strings"

	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
)

// Advice is a provider bothy advises on: its header, flattened with its
// [advise] block. Projected from slots.Provider, so the file format has one
// reader.
type Advice struct {
	slots.Header
	slots.Advise
}

// Avoid is re-exported so callers need not know both packages. Distros limits
// a warning to the distributions it is about: without it bothy told Ubuntu
// users to avoid a Copr, which is advice about a repository they cannot enable.
type Avoid = slots.Avoid

// Get loads advice by name.
// Binary is the command a provider is run as, which is not always its name:
// helix runs as hx, claude-code as claude. Declared in the provider's own file
// so that adding one needs no Go. A provider with no advice file is run as its
// own name, which is true of most of them.
func Binary(name string) string {
	if a, err := Get(name); err == nil && a.Binary != "" {
		return a.Binary
	}
	return name
}

func Get(name string) (Advice, error) {
	pr, ok := slots.Get(name)
	if !ok || pr.Advise == nil {
		return Advice{}, fmt.Errorf("advice: no entry for %q", name)
	}
	return Advice{Header: pr.Header, Advise: *pr.Advise}, nil
}

// Command returns the install command for a machine. An image-based host is
// looked up first: there the command differs, needs a reboot, and getting it
// wrong is expensive -- exactly what a generic "install ghostty" leaves
// someone to work out alone.
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
		if !appliesTo(w, p) {
			continue
		}
		out = append(out, "avoid "+w.Repo+" — "+w.Why)
	}
	return strings.Join(out, "; ")
}

func appliesTo(w Avoid, p platform.Info) bool {
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
