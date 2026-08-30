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
)

// sandbox builds a platform.Info rooted entirely in a temporary directory, so
// a test can install for real without touching the machine it runs on.
func sandbox(t *testing.T) platform.Info {
	t.Helper()
	home := t.TempDir()
	return platform.Info{
		OS:        "linux",
		Arch:      "x86_64",
		Home:      home,
		ConfigDir: filepath.Join(home, ".config"),
		DataDir:   filepath.Join(home, ".local", "share"),
		LocalBin:  filepath.Join(home, ".local", "bin"),
		Terminal:  "ghostty",
		Term:      "xterm-ghostty",
	}
}

func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

// This is ADR-009 as a test. Install everything, in every configuration that
// changes what gets written, and assert not one byte lands outside bothy's
// own directory. If this fails, the isolation promise is broken, whatever the
// README says.
func TestInstallWritesNothingOutsideBothysTree(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*config.Config)
		info func(*platform.Info)
	}{
		{name: "defaults", mut: func(c *config.Config) {}},
		{name: "watermark on", mut: func(c *config.Config) { c.Workspace.Watermark = true }},
		{name: "vim config provided", mut: func(c *config.Config) { c.Editor.ProvideConfig = true }},
		{
			name: "inside a container",
			mut:  func(c *config.Config) {},
			info: func(p *platform.Info) { p.Container = platform.Toolbx; p.ContainerName = "test" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := sandbox(t)
			if tc.info != nil {
				tc.info(&p)
			}
			cfg := config.Default()
			tc.mut(&cfg)

			if _, err := Run(p, cfg, Options{}); err != nil {
				t.Fatalf("install: %v", err)
			}

			for _, rel := range snapshot(t, p.Home) {
				abs := filepath.Join(p.Home, rel)
				if !strings.HasPrefix(abs, p.BothyDir()) {
					t.Errorf("wrote outside bothy's tree: ~/%s", rel)
				}
			}
		})
	}
}

// The three files revision 1 wrote that provoked ADR-009. None of them should
// ever appear again.
func TestInstallNeverWritesTheFilesThatProvokedIsolation(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Editor.ProvideConfig = true
	cfg.Workspace.Watermark = true

	if _, err := Run(p, cfg, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		filepath.Join(p.Home, ".vimrc"),
		filepath.Join(p.Home, ".bashrc.d", "bothy.sh"),
		filepath.Join(p.ConfigDir, "yazi", "yazi.toml"),
		filepath.Join(p.ConfigDir, "zellij", "config.kdl"),
		filepath.Join(p.ConfigDir, "ghostty", "config"),
		filepath.Join(p.LocalBin, "xdg-open"),
	} {
		if _, err := os.Stat(forbidden); err == nil {
			t.Errorf("%s was written; bothy no longer owns this path", forbidden)
		}
	}
}

// The writer refuses a destination outside the root, so a bad template
// destination fails loudly instead of escaping quietly.
func TestWriterRefusesToEscapeTheRoot(t *testing.T) {
	p := sandbox(t)
	w := render.NewWriter(p.ConfigRoot(), "")
	escape := filepath.Join(p.ConfigRoot(), "..", "..", "..", "escaped.txt")
	if _, err := w.Write(escape, []byte("nope")); err == nil {
		t.Fatal("expected a refusal for a path outside the root")
	}
	if _, err := os.Stat(filepath.Join(p.Home, "escaped.txt")); err == nil {
		t.Error("the escaping write actually happened")
	}
}

// The editor is yours unless you ask otherwise.
func TestVimConfigIsOptIn(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()

	res, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Written {
		if strings.Contains(f, "vim") {
			t.Errorf("wrote %s without editor.provide_config", f)
		}
	}

	cfg.Editor.ProvideConfig = true
	res, err = Run(sandboxFrom(t, p), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range res.Written {
		if filepath.Base(f) == "vimrc" {
			found = true
		}
	}
	if !found {
		t.Error("editor.provide_config did not produce a vimrc")
	}
}

// sandboxFrom gives a fresh tree with the same shape, for a second install.
func sandboxFrom(t *testing.T, _ platform.Info) platform.Info { return sandbox(t) }

// Ghostty's palette must be written into bothy's config file, not referenced
// as a named theme — a name is looked up in ~/.config/ghostty/themes, which is
// exactly the directory isolation exists to avoid.
func TestGhosttyConfigInlinesThePalette(t *testing.T) {
	p := sandbox(t)
	if _, err := Run(p, config.Default(), Options{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(GhosttyConf(p))
	if err != nil {
		t.Fatalf("no ghostty config written: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "background = #282A36") {
		t.Error("the palette was not inlined")
	}
	if strings.Contains(s, "\ntheme = ") {
		t.Error("config names a theme; lookup paths are not relocatable, so it must not")
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()

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

// Uninstall is a directory removal, and it must leave the user's own settings
// alone — that directory is the thing worth keeping in git.
func TestUninstallRemovesTheTreeAndKeepsYourSettings(t *testing.T) {
	p := sandbox(t)
	if _, err := Run(p, config.Default(), Options{}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.UserConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(p.UserConfigDir(), "config.toml")
	if err := os.WriteFile(mine, []byte("profile = \"cockpit\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Uninstall(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Removed) != 1 {
		t.Errorf("expected one directory removed, got %v", rep.Removed)
	}
	if _, err := os.Stat(p.BothyDir()); err == nil {
		t.Error("bothy's tree survived uninstall")
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("uninstall removed the user's own settings")
	}
}

func TestEveryGeneratedFileSaysItIsGenerated(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Workspace.Watermark = true
	cfg.Editor.ProvideConfig = true

	res, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Written {
		if filepath.Ext(f) == ".png" {
			continue // a header would stop it being a PNG
		}
		if !render.IsGenerated(f) {
			t.Errorf("%s has no generated-by header", f)
		}
	}
}

func TestOverridesAreAppended(t *testing.T) {
	p := sandbox(t)
	ov := filepath.Join(p.UserConfigDir(), "overrides", "yazi")
	if err := os.MkdirAll(ov, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ov, "yazi.toml"), []byte("show_hidden = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(p, config.Default(), Options{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(YaziDir(p), "yazi.toml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "show_hidden = false") {
		t.Fatal("the override was not applied")
	}
	if strings.Index(s, "show_hidden = false") < strings.Index(s, "ratio") {
		t.Error("the override must come after the template, or it would not win")
	}
}

func TestWatermarkAssetIsCopiedIntact(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Workspace.Watermark = true

	if _, err := Run(p, cfg, Options{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(p.ConfigRoot(), "watermark.png"))
	if err != nil {
		t.Fatalf("watermark not written: %v", err)
	}
	if len(got) < 8 || string(got[1:4]) != "PNG" {
		t.Fatal("watermark is not a PNG — a header was prepended")
	}
	want, err := assetBytes("templates/extras/watermark/tux.png")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("watermark bytes differ from the shipped asset")
	}
}
