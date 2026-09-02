// Package tools decides, for each tool the workspace needs, whether the
// system's copy will do or bothy has to supply one.
//
// Fill gaps, never replace (PLAN.md §4): a tool on PATH meeting the minimum is
// used as it is, and one bothy supplies goes in its own bin/, on PATH for the
// session only so it never shadows your everyday copy. Minimums are "the
// oldest that works", so on a normally equipped machine bothy downloads
// nothing.
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
	"github.com/bspeelm/bothy/internal/slots"
)

// Tool is a provider bothy can fetch: its header, flattened with its [fetch]
// block. Projected from slots.Provider rather than parsed here, so the file
// format has one reader.
type Tool struct {
	slots.Header
	slots.Fetch
}

// Binaries is every binary this tool installs.
func (t Tool) Binaries() []string { return append([]string{t.Binary}, t.Extra...) }

// Asset returns the release asset name for a platform, with {version}
// substituted. version is the tag with any leading "v" or "<name>-" removed,
// which is how every asset name in slots/ spells it.
func (t Tool) Asset(p platform.Info, version string) (string, error) {
	key := p.OS + "_" + p.Arch
	pattern, ok := t.Assets[key]
	if !ok {
		return "", fmt.Errorf("tools: %s has no asset for %s", t.Name, key)
	}
	return strings.ReplaceAll(pattern, "{version}", version), nil
}

// ChecksumFile is the release asset carrying this tool's own sha256, or "".
// Two upstream shapes — a sibling per asset or one manifest — with the same
// "<sha256>  <filename>" lines inside.
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

// Load reads every provider bothy can fetch, sorted by name so install and
// doctor output is stable between runs.
func Load() ([]Tool, error) {
	all, err := slots.All()
	if err != nil {
		return nil, err
	}
	var out []Tool
	for _, pr := range all {
		if pr.Fetch == nil {
			continue
		}
		if pr.Fetch.Binary == "" || pr.Fetch.Repo == "" {
			return nil, fmt.Errorf("tools: %s is missing binary or repo", pr.Name)
		}
		out = append(out, Tool{Header: pr.Header, Fetch: *pr.Fetch})
	}
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

// SystemLookPath finds a binary on PATH, ignoring one directory -- which
// callers set to bothy's own bin. That directory is on PATH inside the
// session, so a plain exec.LookPath answers "does the system have this?"
// differently depending on where it was asked from.
func SystemLookPath(skip string) func(string) (string, error) {
	skip = filepath.Clean(skip)
	return func(bin string) (string, error) {
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			if dir == "" || (skip != "." && filepath.Clean(dir) == skip) {
				continue
			}
			path := filepath.Join(dir, bin)
			if fi, err := os.Stat(path); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
				return path, nil
			}
		}
		return "", fmt.Errorf("%s not found in $PATH", bin)
	}
}

func SystemVersion(path string) (string, error) {
	out, err := exec.Command(path, "--version").Output()
	return string(out), err
}

// ResolveAll decides for every named tool, judging what the system has.
// ownBin is bothy's own directory, left out of the search so that the same
// question gets the same answer inside a bothy session and outside one.
func ResolveAll(names []string, ownBin string) ([]Decision, error) {
	var out []Decision
	for _, n := range names {
		t, err := Get(n)
		if err != nil {
			return nil, err
		}
		out = append(out, Resolve(t, SystemLookPath(ownBin), SystemVersion))
	}
	return out, nil
}

// Required returns the tools a configuration actually needs. Asking for the
// whole list would have bothy fetching a git TUI for someone who turned the
// side pane off.
//
// providers is every slot's provider, not the two bothy can fetch today: one
// with no [fetch] block is dropped by the same known[] test that drops a
// misspelled extra. Naming mux and browser made "which slots are fetchable" an
// argument list, which the files say for themselves now.
func Required(providers, extras []string) ([]string, error) {
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
	for _, p := range providers {
		add(p)
	}
	for _, e := range extras {
		add(e)
	}
	sort.Strings(want)
	return want, nil
}
