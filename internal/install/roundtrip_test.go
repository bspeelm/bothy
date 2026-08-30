package install

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/render"
	"github.com/bothy-dev/bothy/internal/state"
)

// sandbox builds a platform.Info pointing entirely at a temporary directory, so
// a test can install for real without touching the machine it runs on.
func sandbox(t *testing.T) platform.Info {
	t.Helper()
	home := t.TempDir()
	return platform.Info{
		OS:        "linux",
		Arch:      "x86_64",
		Home:      home,
		ConfigDir: filepath.Join(home, ".config"),
		StateDir:  filepath.Join(home, ".local", "state"),
		LocalBin:  filepath.Join(home, ".local", "bin"),
		Terminal:  "ghostty",
		Term:      "xterm-ghostty",
	}
}

// snapshot lists every file under a directory, relative and sorted.
func snapshot(t *testing.T, root string, skip ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, s := range skip {
			if strings.HasPrefix(rel, s) {
				return nil
			}
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

// TestRoundTripLeavesNothingBehind is the reversibility promise from PLAN.md §0,
// checked rather than asserted: install, uninstall, and the filesystem is as it
// was. bothy's own state directory is excluded because the manifest is the
// record of the removal itself.
func TestRoundTripLeavesNothingBehind(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	// The git settings touch the real global git config, so they are exercised
	// separately in TestGitSettingsRoundTrip rather than here.
	cfg.Extras = nil

	before := snapshot(t, p.Home)

	if _, err := Run(p, cfg, Options{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	installed := snapshot(t, p.Home, filepath.Join(".local", "state"))
	if len(installed) == 0 {
		t.Fatal("install wrote nothing")
	}

	m, err := state.Load(p.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p, m, false); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	after := snapshot(t, p.Home, filepath.Join(".local", "state"))
	if len(after) != len(before) {
		t.Errorf("uninstall left %d file(s) behind:\n  %s",
			len(after)-len(before), strings.Join(after, "\n  "))
	}
}

// A file that was there before bothy must come back exactly as it was, not be
// deleted along with bothy's own output.
func TestRoundTripRestoresPreExistingFiles(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Extras = nil

	original := "# my own yazi config\nratio = [1, 1, 1]\n"
	dest := filepath.Join(p.ConfigDir, "yazi", "yazi.toml")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(p, cfg, Options{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	// bothy's version is in place now.
	if b, _ := os.ReadFile(dest); string(b) == original {
		t.Fatal("install did not replace the existing config")
	}
	if !render.IsManaged(dest) {
		t.Error("the installed file does not carry the managed-by header")
	}

	m, err := state.Load(p.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p, m, false); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("the user's own file was not restored: %v", err)
	}
	if string(got) != original {
		t.Errorf("restored content differs:\n got: %q\nwant: %q", got, original)
	}
}

// A managed file someone edited by hand must survive both a reinstall and an
// uninstall. Deleting an edit is the one mistake with no undo.
func TestHandEditedManagedFileIsNotClobbered(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Extras = nil

	if _, err := Run(p, cfg, Options{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	dest := filepath.Join(p.ConfigDir, "yazi", "yazi.toml")
	edited := "# managed by bothy\n# but then I changed it\nshow_hidden = false\n"
	if err := os.WriteFile(dest, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s == dest {
			skipped = true
		}
	}
	if !skipped {
		t.Error("a hand-edited managed file should be reported as skipped")
	}
	if b, _ := os.ReadFile(dest); string(b) != edited {
		t.Error("reinstall overwrote a hand-edited managed file")
	}

	m, err := state.Load(p.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p, m, false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != edited {
		t.Error("uninstall removed a hand-edited file it no longer owned")
	}
}

// Installing twice with no changes must report the second run as a no-op —
// which is what makes `bothy install` safe to re-run.
func TestInstallIsIdempotent(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Extras = nil

	first, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Written) != 0 {
		t.Errorf("second install rewrote %d file(s): %v", len(second.Written), second.Written)
	}
	if len(second.Unchanged) != len(first.Written) {
		t.Errorf("second install saw %d unchanged, first wrote %d",
			len(second.Unchanged), len(first.Written))
	}
}

// Overrides are appended after the template, so a user's setting wins in every
// format bothy writes.
func TestOverridesAreAppended(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Extras = nil

	ov := filepath.Join(p.ConfigDir, "bothy", "overrides", "yazi")
	if err := os.MkdirAll(ov, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ov, "yazi.toml"), []byte("show_hidden = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(p, cfg, Options{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(p.ConfigDir, "yazi", "yazi.toml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "show_hidden = false") {
		t.Error("the override was not applied")
	}
	if strings.Index(s, "show_hidden = false") < strings.Index(s, "ratio") {
		t.Error("the override must come after the template, or it would not win")
	}
}

// Every file bothy writes must say it is bothy's and where to put changes.
// Without that, the next person to open it has no way to know.
func TestEveryWrittenFileIsMarkedManaged(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Extras = nil

	res, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Written {
		if !render.IsManaged(f) {
			t.Errorf("%s has no managed-by header", f)
		}
	}
}

// The plan must never write outside the user's home.
func TestPlanStaysInsideHome(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	pal, err := cfg.Palette(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range plan(p, cfg, buildData(p, cfg, pal)) {
		if !strings.HasPrefix(f.Dest, p.Home) {
			t.Errorf("%s is outside the home directory", f.Dest)
		}
	}
}

// The xdg-open shim exists to forward to a host, so it belongs only inside a
// container. On the host it would shadow the real xdg-open for everything.
func TestXdgOpenShimOnlyInsideContainers(t *testing.T) {
	cfg := config.Default()

	host := sandbox(t)
	pal, _ := cfg.Palette(host)
	for _, f := range plan(host, cfg, buildData(host, cfg, pal)) {
		if filepath.Base(f.Dest) == "xdg-open" {
			t.Error("the shim must not be installed on a host")
		}
	}

	inside := sandbox(t)
	inside.Container = platform.Toolbx
	inside.ContainerName = "test"
	var found bool
	for _, f := range plan(inside, cfg, buildData(inside, cfg, pal)) {
		if filepath.Base(f.Dest) == "xdg-open" {
			found = true
			if !f.Exec {
				t.Error("the shim must be written executable")
			}
		}
	}
	if !found {
		t.Error("the shim should be installed inside a container")
	}
}
