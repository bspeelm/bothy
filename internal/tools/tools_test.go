package tools

import (
	"errors"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

func TestShippedDefinitionsAreComplete(t *testing.T) {
	all, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("no tool definitions embedded")
	}
	linux := platform.Info{OS: "linux", Arch: "x86_64"}
	for _, tool := range all {
		if _, err := tool.Min(); err != nil {
			t.Errorf("%s: min_version %q is unparseable", tool.Name, tool.MinVersion)
		}
		// Every tool must at least target the platform bothy is developed on;
		// a missing asset there is a definition nobody could have tested.
		asset, err := tool.Asset(linux, "1.2.3")
		if err != nil {
			t.Errorf("%s: %v", tool.Name, err)
			continue
		}
		if strings.Contains(asset, "{") {
			t.Errorf("%s: unsubstituted placeholder in %q", tool.Name, asset)
		}
	}
}

// The two minimums that exist for a reason must carry that reason, because it
// is what the install output shows when it replaces someone's binary.
func TestVersionGatesExplainThemselves(t *testing.T) {
	for _, name := range []string{"zellij", "yazi"} {
		tool, err := Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if tool.MinVersion == "" {
			t.Errorf("%s has no minimum version", name)
		}
		if tool.Reason == "" {
			t.Errorf("%s has a minimum but no reason; the install output would not explain itself", name)
		}
	}
}

func fakeLookup(paths map[string]string) func(string) (string, error) {
	return func(bin string) (string, error) {
		if p, ok := paths[bin]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func fakeVersion(versions map[string]string) func(string) (string, error) {
	return func(path string) (string, error) {
		if v, ok := versions[path]; ok {
			return v, nil
		}
		return "", errors.New("no version")
	}
}

// The policy in one test: good enough is left alone, too old is replaced.
func TestResolveUsesAGoodSystemTool(t *testing.T) {
	zellij, _ := Get("zellij")
	d := Resolve(zellij,
		fakeLookup(map[string]string{"zellij": "/usr/bin/zellij"}),
		fakeVersion(map[string]string{"/usr/bin/zellij": "zellij 0.46.0"}))
	if d.Action != UseSystem {
		t.Errorf("action = %s, want use-system for a newer zellij (%s)", d.Action, d.Reason)
	}
}

func TestResolveFetchesATooOldTool(t *testing.T) {
	zellij, _ := Get("zellij")
	d := Resolve(zellij,
		fakeLookup(map[string]string{"zellij": "/usr/bin/zellij"}),
		fakeVersion(map[string]string{"/usr/bin/zellij": "zellij 0.42.2"}))
	if d.Action != Fetch {
		t.Fatalf("action = %s, want fetch for 0.42.2", d.Action)
	}
	// The reason has to say what was wrong and why it matters — this is the
	// text a user reads when bothy replaces a binary they installed.
	for _, want := range []string{"0.42.2", "0.45.1", "Kitty graphics"} {
		if !strings.Contains(d.Reason, want) {
			t.Errorf("reason %q does not mention %q", d.Reason, want)
		}
	}
}

func TestResolveFetchesAMissingTool(t *testing.T) {
	zellij, _ := Get("zellij")
	d := Resolve(zellij, fakeLookup(nil), fakeVersion(nil))
	if d.Action != Fetch || d.Reason != "not installed" {
		t.Errorf("action = %s reason = %q", d.Action, d.Reason)
	}
}

// A tool that will not report a version is replaced rather than trusted: an
// unknown version cannot be checked against a minimum.
func TestResolveFetchesWhenVersionIsUnreadable(t *testing.T) {
	zellij, _ := Get("zellij")
	d := Resolve(zellij,
		fakeLookup(map[string]string{"zellij": "/usr/bin/zellij"}),
		fakeVersion(map[string]string{"/usr/bin/zellij": "no numbers here"}))
	if d.Action != Fetch {
		t.Errorf("action = %s, want fetch", d.Action)
	}
}

// bothy should not fetch a git TUI for someone who turned the side pane off.
func TestRequiredFollowsTheConfiguration(t *testing.T) {
	got, err := Required("zellij", "yazi", []string{"lazygit"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "lazygit,yazi,zellij" {
		t.Errorf("Required() = %v", got)
	}

	got, err = Required("zellij", "none", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "zellij" {
		t.Errorf("with no browser, Required() = %v", got)
	}
}

func TestYaziDeclaresItsSecondBinary(t *testing.T) {
	yazi, err := Get("yazi")
	if err != nil {
		t.Fatal(err)
	}
	bins := yazi.Binaries()
	if len(bins) != 2 || bins[0] != "yazi" || bins[1] != "ya" {
		t.Errorf("Binaries() = %v; the archive ships ya alongside yazi", bins)
	}
}
