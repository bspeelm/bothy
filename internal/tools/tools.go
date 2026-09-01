// Package tools decides, for each tool the workspace needs, whether the
// system's copy will do or bothy has to supply one.
//
// The policy is PLAN.md §4: fill gaps, never replace. A tool already on PATH
// that meets the minimum version is used as it is; only a missing or too-old
// one is fetched. bothy never upgrades, removes, or asks a package manager for
// anything, and a tool it does supply goes in its own bin/ — on PATH for
// bothy's session only, so it never shadows your everyday one.
//
// Minimum versions are "the oldest that actually works", not "the newest
// available". For most of these almost any version does, so on a normally
// equipped machine bothy downloads nothing.
package tools

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
)

// Tool is a declarative definition, loaded from slots/tools/<name>.toml.
type Tool struct {
	Name   string `toml:"name"`
	Binary string `toml:"binary"`
	// Extra names additional binaries in the same archive that must be
	// installed alongside — yazi ships its package manager `ya` this way.
	Extra      []string          `toml:"extra"`
	Repo       string            `toml:"repo"`
	MinVersion string            `toml:"min_version"`
	Reason     string            `toml:"reason"`
	Assets     map[string]string `toml:"assets"`
	// Checksums names the file the project publishes its own sha256 in, with
	// {asset} and {version} interpolated. Empty when it publishes none, which
	// is most of them. See Tool.ChecksumFile.
	Checksums string `toml:"checksums"`
}

// Binaries is every binary this tool installs.
func (t Tool) Binaries() []string { return append([]string{t.Binary}, t.Extra...) }

// Asset returns the release asset name for a platform, with {version}
// substituted. version is the tag with any leading "v" or "<name>-" removed,
// which is how every asset name in slots/tools spells it.
func (t Tool) Asset(p platform.Info, version string) (string, error) {
	key := p.OS + "_" + p.Arch
	pattern, ok := t.Assets[key]
	if !ok {
		return "", fmt.Errorf("tools: %s has no asset for %s", t.Name, key)
	}
	return strings.ReplaceAll(pattern, "{version}", version), nil
}

// ChecksumFile is the release asset carrying this tool's own sha256 for one
// platform, or "" when the project publishes none.
//
// Two shapes are in use upstream and one field covers both: a sibling beside
// each asset ("{asset}.sha256"), or one manifest for the whole release
// ("checksums.txt"). What is inside them is the same either way -- lines of
// "<sha256>  <filename>" -- so the parser does not need to know which it got.
func (t Tool) ChecksumFile(p platform.Info, version string) (string, error) {
	if t.Checksums == "" {
		return "", nil
	}
	asset, err := t.Asset(p, version)
	if err != nil {
		return "", err
	}
	name := strings.ReplaceAll(t.Checksums, "{asset}", asset)
	return strings.ReplaceAll(name, "{version}", version), nil
}

// Min parses the minimum version.
func (t Tool) Min() (probe.Version, error) {
	if t.MinVersion == "" {
		return probe.Version{}, nil
	}
	return probe.ParseVersion(t.MinVersion)
}

// Load reads the embedded tool definitions, sorted by name so install and
// doctor output are stable between runs.
func Load() ([]Tool, error) {
	entries, err := bothy.Slots.ReadDir("slots/tools")
	if err != nil {
		return nil, fmt.Errorf("tools: %w", err)
	}
	var out []Tool
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
			continue
		}
		src, err := bothy.Slots.ReadFile("slots/tools/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("tools: %w", err)
		}
		var t Tool
		if err := toml.Unmarshal(src, &t); err != nil {
			return nil, fmt.Errorf("tools: %s: %w", e.Name(), err)
		}
		if t.Name == "" || t.Binary == "" || t.Repo == "" {
			return nil, fmt.Errorf("tools: %s is missing name, binary or repo", e.Name())
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get returns one tool by name.
func Get(name string) (Tool, error) {
	all, err := Load()
	if err != nil {
		return Tool{}, err
	}
	for _, t := range all {
		if t.Name == name {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("tools: no definition for %q", name)
}

// Action is what should happen to a tool at install time.
type Action string

const (
	// UseSystem: the copy already on PATH is good enough.
	UseSystem Action = "use-system"
	// Fetch: missing, or below the minimum version.
	Fetch Action = "fetch"
)

// Decision records what was decided about one tool, and why. The reason is
// carried because "bothy downloaded zellij" is only useful to read if it also
// says what was wrong with the one you had.
type Decision struct {
	Tool    Tool
	Action  Action
	Path    string        // where the system copy was found, when using it
	Version probe.Version // the system copy's version, when there is one
	Reason  string
}

// Resolve decides for one tool.
//
// lookPath is injectable so tests can decide what is "installed" without
// depending on the machine they run on.
func Resolve(t Tool, lookPath func(string) (string, error), version func(string) (string, error)) Decision {
	min, err := t.Min()
	if err != nil {
		return Decision{Tool: t, Action: Fetch,
			Reason: fmt.Sprintf("min_version %q is unparseable", t.MinVersion)}
	}

	path, err := lookPath(t.Binary)
	if err != nil {
		return Decision{Tool: t, Action: Fetch, Reason: "not installed"}
	}

	out, err := version(path)
	if err != nil {
		return Decision{Tool: t, Action: Fetch, Path: path,
			Reason: fmt.Sprintf("%s does not report a version", path)}
	}
	v, err := probe.ParseVersion(out)
	if err != nil {
		return Decision{Tool: t, Action: Fetch, Path: path,
			Reason: fmt.Sprintf("could not read a version from %s", path)}
	}

	if v.Less(min) {
		reason := fmt.Sprintf("%s is %s, below the minimum %s", path, v, min)
		if t.Reason != "" {
			reason += " — " + t.Reason
		}
		return Decision{Tool: t, Action: Fetch, Path: path, Version: v, Reason: reason}
	}
	return Decision{Tool: t, Action: UseSystem, Path: path, Version: v,
		Reason: fmt.Sprintf("%s is %s", path, v)}
}

// SystemLookPath and SystemVersion are the real implementations Resolve uses
// in production.
func SystemLookPath(bin string) (string, error) { return exec.LookPath(bin) }

func SystemVersion(path string) (string, error) {
	out, err := exec.Command(path, "--version").Output()
	return string(out), err
}

// ResolveAll decides for every named tool.
func ResolveAll(names []string) ([]Decision, error) {
	var out []Decision
	for _, n := range names {
		t, err := Get(n)
		if err != nil {
			return nil, err
		}
		out = append(out, Resolve(t, SystemLookPath, SystemVersion))
	}
	return out, nil
}

// Required returns the tools a configuration actually needs. Asking for the
// whole list would have bothy fetching a git TUI for someone who turned the
// side pane off.
func Required(mux, browser string, extras []string) ([]string, error) {
	all, err := Load()
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, t := range all {
		known[t.Name] = true
	}

	var want []string
	add := func(n string) {
		if n == "" || n == "none" || !known[n] {
			return
		}
		for _, existing := range want {
			if existing == n {
				return
			}
		}
		want = append(want, n)
	}
	add(mux)
	add(browser)
	for _, e := range extras {
		add(e)
	}
	sort.Strings(want)
	return want, nil
}
