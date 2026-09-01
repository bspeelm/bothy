//go:build container || macos

// Assertions shared by the end-to-end jobs.
//
// The container job and the macOS job install bothy for real and then ask the
// same three questions: does the doctor say what it should, did anything land
// outside bothy's tree, and does uninstall take it all away. Only the way they
// reach a machine differs, so only that is written twice.
package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/doctor"
	"github.com/bspeelm/bothy/internal/tools"
)

// assertReport compares a doctor report against what this machine should say.
func assertReport(t *testing.T, rep doctor.Report, want map[string]doctor.Severity) {
	t.Helper()
	got := make(map[string]doctor.Severity, len(rep.Results))
	for _, r := range rep.Results {
		got[r.ID] = r.Severity
	}
	for id, wantSev := range want {
		gotSev, ok := got[id]
		if !ok {
			t.Errorf("check %q is missing from the report", id)
			continue
		}
		if gotSev != wantSev {
			t.Errorf("check %q = %s, want %s", id, gotSev, wantSev)
		}
	}
	// The half that keeps this honest: a new check must be accounted for here
	// rather than silently going uncovered.
	for id := range got {
		if _, ok := want[id]; !ok {
			t.Errorf("check %q is not in the expectation table; decide what it "+
				"should report on this machine and add it", id)
		}
	}
}

// filesUnder lists every file below root, relative to it.
func filesUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// bothysOwn are the only paths bothy itself writes.
var bothysOwn = []string{".local/share/bothy/", ".config/bothy/", ".local/bin/bothy"}

// toolOwned reports whether a path is a tool's own data or state directory,
// named after a tool bothy runs.
//
// Matching on the tool's name rather than allowing the parent directories
// wholesale is what keeps this an assertion: ~/.local/share is where anything
// might land, and "a directory under it" would wave through exactly the writes
// this is meant to catch.
func toolOwned(path string) bool {
	rest, ok := strings.CutPrefix(path, ".local/share/")
	if !ok {
		if rest, ok = strings.CutPrefix(path, ".local/state/"); !ok {
			return false
		}
	}
	name, _, ok := strings.Cut(rest, "/")
	if !ok {
		return false
	}
	all, err := tools.Load()
	if err != nil {
		return false
	}
	for _, t := range all {
		if t.Name == name || t.Binary == name {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// assertNothingUnexplained is ADR-009 checked end to end, after the real tools
// have really run.
//
// bothy points only XDG_CACHE_HOME into its own tree, so a tool keeps its data
// where it always would (ADR-022). So the promise this checks is the one bothy
// can keep: nothing outside its tree is bothy's doing, and anything else there
// belongs to a tool bothy ran and is named after it.
func assertNothingUnexplained(t *testing.T, files []string) {
	t.Helper()
	var stray, tooldata []string
	for _, f := range files {
		switch {
		case hasAnyPrefix(f, bothysOwn):
		case toolOwned(f):
			tooldata = append(tooldata, f)
		default:
			stray = append(stray, f)
		}
	}
	// Logged rather than asserted: which tool writes what is upstream's
	// business and changes with their releases. That it is *named after a
	// tool* is the assertion, and `bothy doctor` reports the same set.
	if len(tooldata) > 0 {
		t.Logf("%d file(s) the tools keep outside bothy's tree, by design:\n  %s",
			len(tooldata), strings.Join(tooldata, "\n  "))
	}
	if len(stray) > 0 {
		t.Errorf("%d file(s) written outside bothy's tree and belonging to no tool:\n  %s",
			len(stray), strings.Join(stray, "\n  "))
	}
}

// assertGone: uninstall removes bothy, which is its own tree and its own
// binary. A tool's data is not bothy's to delete, and the doctor says so
// before anyone runs this.
func assertGone(t *testing.T, files []string) {
	t.Helper()
	for _, f := range files {
		// config.toml is the user's own file and survives on purpose.
		if f == ".config/bothy/config.toml" || toolOwned(f) {
			continue
		}
		t.Errorf("uninstall left %s behind", f)
	}
}
