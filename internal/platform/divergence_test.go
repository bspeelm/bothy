package platform

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The register is only worth its lines if the three things an entry promises
// are checked. These are the checks.

const repoRoot = "../.."

// A reference platform behaving like one is the half of a divergence that is
// easy to break and impossible to notice: the Windows arm is the one being
// worked on, and the other arms are only ever read. Asking the whole matrix
// from one runner is what ADR-031's injected-OS rule buys.
func TestTheReferencePlatformsDivergeInNothing(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		for _, mux := range []string{"zellij", "none", ""} {
			if got := Active(Info{OS: goos}, mux); len(got) != 0 {
				t.Errorf("%s with mux %q diverges in %d place(s): %v", goos, mux, len(got), ids(got))
			}
		}
	}
	// And the platform that does is not silently empty, which is how a
	// register stops being read.
	if got := Active(Info{OS: "windows"}, "zellij"); len(got) == 0 {
		t.Error("windows diverges in nothing; either the register emptied or Active stopped matching")
	}
}

// A narrowed entry applies to the multiplexer it was measured against and not
// to one nobody has tried.
func TestAnEntryNarrowedToAMultiplexerStaysThere(t *testing.T) {
	withZellij := len(Active(Info{OS: "windows"}, "zellij"))
	withNone := len(Active(Info{OS: "windows"}, "none"))
	if withNone >= withZellij {
		t.Errorf("windows diverges in %d place(s) without zellij and %d with it; "+
			"a zellij-only entry is being claimed for a multiplexer it was never measured against",
			withNone, withZellij)
	}
}

// Every entry names an ADR that exists. ADR-059 recorded a Windows take-over
// as impossible and stayed wrong for two releases; an entry whose decision
// cannot be found is the same failure one step earlier.
func TestEveryEntryNamesADecisionThatExists(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot, "docs", "decisions.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range Divergences() {
		if d.ADR == "" {
			t.Errorf("%s names no ADR", d.ID)
			continue
		}
		if !strings.Contains(string(body), "## "+d.ADR+" ") {
			t.Errorf("%s names %s, which is not a heading in docs/decisions.md", d.ID, d.ADR)
		}
	}
}

// Every entry names a test that exists, and says when it was last true.
func TestEveryEntryNamesATestAndADate(t *testing.T) {
	suite := allTestNames(t)
	for _, d := range Divergences() {
		if d.Test == "" || !suite[d.Test] {
			t.Errorf("%s names test %q, which is not in the suite", d.ID, d.Test)
		}
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`).MatchString(d.Measured) {
			t.Errorf("%s was measured %q; an entry nobody has re-measured is a bug in waiting", d.ID, d.Measured)
		}
		if d.Reference == "" || d.Here == "" {
			t.Errorf("%s does not say what both platforms do", d.ID)
		}
	}
}

// Entries are addressed by id, so two of a name is an entry nothing can point
// at, and a marker naming a cost is a branch compensating for something the
// register says is not compensated for.
func TestEveryEntryIsNamedOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range Divergences() {
		if seen[d.ID] {
			t.Errorf("%s is in the register twice", d.ID)
		}
		seen[d.ID] = true
		if d.OS == "linux" || d.OS == "darwin" {
			t.Errorf("%s diverges on %s, which is a reference platform", d.ID, d.OS)
		}
	}
}

// exemptBranches counts the per-OS branches that are not compensating for a
// divergence: a file name, a path, or which program does a job. The count is
// the guard, as it is for the build-tagged shims in ADR-031 -- a new branch
// in one of these files fails here until it is either registered or admitted
// to be another of the same.
var exemptBranches = map[string]int{
	"cmd/bothy/confinecmd.go":             1, // confine wants podman; the gate reports it unavailable
	"cmd/bothy/connect.go":                1, // how a mount is undone, per OS
	"cmd/bothy/terminal.go":               3, // where a display is, where an app bundle is, which stream a warning goes to
	"cmd/bothy/tower.go":                  1, // whether a pane command carries .exe
	"internal/config/config.go":           1, // which terminal exists to default to
	"internal/doctor/checks_isolation.go": 1, // an Apple quarantine flag
	"internal/doctor/checks_terminal.go":  2, // which opener to advise
	"internal/doctor/checks_tools.go":     1, // a Microsoft C runtime to look for
	"internal/install/install.go":         5, // file.exe, the opener, the layout path, the shell quoting
	"internal/mux/zellij.go":              1, // where the socket directory is
	"internal/platform/platform.go":       2, // the .exe suffix, and the execute bit nobody has there
}

var perOS = regexp.MustCompile(`(OS|goos) (==|!=) "(windows|darwin|linux)"`)
var marker = regexp.MustCompile(`//\s*divergence:\s*([a-z0-9-]+)`)

// A per-OS branch is either compensating for something in the register, and
// says so, or it is one of the mechanical ones counted above. This is the
// guard that makes the next divergence impossible to add in silence.
func TestEveryPlatformBranchIsRegisteredOrCounted(t *testing.T) {
	carried := map[string]bool{}
	for _, d := range Divergences() {
		if d.Carried != "" {
			carried[d.ID] = true
		}
	}
	found := map[string]int{}
	for _, dir := range []string{"cmd", "internal"} {
		walk(t, filepath.Join(repoRoot, dir), func(rel string, lines []string) {
			for i, line := range lines {
				if !perOS.MatchString(line) {
					continue
				}
				if id, ok := markedAbove(lines, i); ok {
					if !carried[id] {
						t.Errorf("%s:%d is marked %q, which the register does not carry", rel, i+1, id)
					}
					continue
				}
				found[rel]++
			}
		})
	}
	for file, n := range found {
		if want, ok := exemptBranches[file]; !ok {
			t.Errorf("%s has %d per-OS branch(es) that neither name a divergence nor are counted; "+
				"add a `// divergence: <id>` marker and an entry, or add the file here with the reason", file, n)
		} else if n != want {
			t.Errorf("%s has %d per-OS branch(es), counted %d; a new one is either a divergence or another of the same", file, n, want)
		}
	}
	for file := range exemptBranches {
		if found[file] == 0 {
			t.Errorf("%s is counted here and has no per-OS branch left; drop it", file)
		}
	}
}

// markedAbove reports the divergence a branch names, from the comment block
// above it. Twelve lines because a reason worth writing is a paragraph.
func markedAbove(lines []string, at int) (string, bool) {
	for i := at; i >= 0 && i > at-12; i-- {
		if m := marker.FindStringSubmatch(lines[i]); m != nil {
			return m[1], true
		}
	}
	return "", false
}

func walk(t *testing.T, root string, fn func(rel string, lines []string)) {
	t.Helper()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		fn(filepath.ToSlash(rel), strings.Split(string(body), "\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func allTestNames(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	decl := regexp.MustCompile(`^func (Test\w+)\(`)
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.Walk(filepath.Join(repoRoot, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(body), "\n") {
				if m := decl.FindStringSubmatch(line); m != nil {
					out[m[1]] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func ids(ds []Divergence) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}
