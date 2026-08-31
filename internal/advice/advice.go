// Package advice holds install instructions for the things bothy will not
// install itself.
//
// Two of them, for different reasons. Ghostty publishes no release binaries and
// every path to it runs a package manager as root — on an image-based host,
// with a reboot — so bothy could start that job and not finish it, which is
// worse than printing the command. The agent is a documented non-goal: install
// methods change, auth is not bothy's business, and a workspace tool that
// quietly installs an AI agent is doing something nobody asked it to.
//
// What bothy can do is name the right command for the machine it is on, and
// keep people away from the repositories that are known to cause harm.
package advice

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bothy-dev/bothy"
	"github.com/bothy-dev/bothy/internal/platform"
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
	keys := []string{p.DistroID, p.OS, "default"}
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

// Warnings renders the repositories to stay away from.
func (a Advice) Warnings() string {
	if len(a.Avoid) == 0 {
		return ""
	}
	var out []string
	for _, w := range a.Avoid {
		out = append(out, "avoid "+w.Repo+" — "+w.Why)
	}
	return strings.Join(out, "; ")
}
