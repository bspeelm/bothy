package install

import (
	"os"
	"os/exec"
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

	rep, err := Uninstall(p, false, false)
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

// A slot passed through uses the user's own config directory, so bothy must
// not write its version — files nothing reads are clutter, and worse, they
// look authoritative to whoever finds them later.
func TestPassthroughSkipsThatSlotsConfigs(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()
	cfg.Passthrough = []string{"yazi"}

	res, err := Run(p, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Written {
		if strings.Contains(f, string(filepath.Separator)+"yazi"+string(filepath.Separator)) {
			t.Errorf("wrote %s for a passed-through slot", f)
		}
	}
	// The slots that were not passed through are still bothy's.
	var sawZellij bool
	for _, f := range res.Written {
		if strings.Contains(f, "zellij") {
			sawZellij = true
		}
	}
	if !sawZellij {
		t.Error("passing yazi through should not affect zellij")
	}
}

// Passthrough is one variable's value, not a second code path: the launcher
// simply does not point the tool at bothy's directory.
func TestPassthroughOmitsTheEnvironmentVariable(t *testing.T) {
	p := sandbox(t)
	cfg := config.Default()

	// Set them in the ambient environment first. A bothy session launched from
	// inside another one inherits exactly this, and "decline to set" would
	// leave the inherited value in place — which is how this test found a real
	// bug rather than a test bug.
	t.Setenv("YAZI_CONFIG_HOME", "/inherited/yazi")
	t.Setenv("ZELLIJ_CONFIG_DIR", "/inherited/zellij")

	env := SessionEnv(p, cfg)
	if !hasEnvPrefix(env, "YAZI_CONFIG_HOME=") {
		t.Fatal("YAZI_CONFIG_HOME should be set when bothy manages yazi")
	}

	cfg.Passthrough = []string{"yazi"}
	env = SessionEnv(p, cfg)
	if hasEnvPrefix(env, "YAZI_CONFIG_HOME=") {
		t.Error("YAZI_CONFIG_HOME must be left alone when yazi is passed through")
	}
	if !hasEnvPrefix(env, "ZELLIJ_CONFIG_DIR=") {
		t.Error("passing yazi through should not affect zellij")
	}
}

// The session must put bothy's own bin first, so a tool it supplied is used
// here — and only here.
func TestSessionPathPrefersBothysBin(t *testing.T) {
	p := sandbox(t)
	for _, kv := range SessionEnv(p, config.Default()) {
		if strings.HasPrefix(kv, "PATH=") {
			if !strings.HasPrefix(strings.TrimPrefix(kv, "PATH="), p.BinDir()+string(os.PathListSeparator)) {
				t.Errorf("PATH does not start with bothy's bin: %s", kv)
			}
			return
		}
	}
	t.Fatal("no PATH in the session environment")
}

func hasEnvPrefix(env []string, prefix string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return true
		}
	}
	return false
}

// Uninstall removed a multiplexer out from under a live session: the process
// kept running on a deleted inode, could never be reattached, and held its
// memory until reboot. Uninstall must notice processes it is about to
// invalidate — before removing the files, since afterwards the only evidence
// is a /proc link marked "(deleted)".
func TestUninstallReportsProcessesItWouldOrphan(t *testing.T) {
	p := sandbox(t)
	if err := os.MkdirAll(p.BinDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	// A real process running from bothy's bin: copy a binary in and start it.
	sh, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary")
	}
	body, err := os.ReadFile(sh)
	if err != nil {
		t.Skip("cannot read sleep")
	}
	fake := filepath.Join(p.BinDir(), "sleep")
	if err := os.WriteFile(fake, body, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(fake, "30")
	if err := cmd.Start(); err != nil {
		t.Skip("cannot start a process here")
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	found := StillRunning(p)
	if len(found) == 0 {
		t.Skip("/proc is not readable in this environment")
	}
	var sawIt bool
	for _, r := range found {
		if r.PID == cmd.Process.Pid {
			sawIt = true
		}
	}
	if !sawIt {
		t.Errorf("did not spot pid %d running from bothy's bin", cmd.Process.Pid)
	}

	// And uninstall must surface it rather than silently invalidating it.
	rep, err := Uninstall(p, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Orphaned) == 0 {
		t.Error("uninstall did not report the process it would orphan")
	}
}

// A machine with nothing running must not report phantoms.
func TestStillRunningIsQuietWhenNothingIs(t *testing.T) {
	p := sandbox(t)
	if got := StillRunning(p); len(got) != 0 {
		t.Errorf("reported %d process(es) for an empty bin dir: %v", len(got), got)
	}
}

// An uninstaller that leaves itself installed has not finished. A process can
// unlink its own executable on Linux, so "remove it by hand" was caution with
// nothing behind it.
func TestUninstallRemovesTheBinary(t *testing.T) {
	p := sandbox(t)
	if err := os.MkdirAll(p.LocalBin, 0o755); err != nil {
		t.Fatal(err)
	}
	self := filepath.Join(p.LocalBin, "bothy")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(p, config.Default(), Options{Offline: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := Uninstall(p, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(self); err == nil {
		t.Error("uninstall left its own binary behind")
	}
}

func TestUninstallKeepsTheBinaryWhenAsked(t *testing.T) {
	p := sandbox(t)
	if err := os.MkdirAll(p.LocalBin, 0o755); err != nil {
		t.Fatal(err)
	}
	self := filepath.Join(p.LocalBin, "bothy")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p, false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Error("--keep-binary removed the binary anyway")
	}
}

// A copy somewhere else — /usr/local/bin, or one a package manager owns — is
// not bothy's to delete.
func TestUninstallOnlyRemovesItsOwnInstallPath(t *testing.T) {
	p := sandbox(t)
	elsewhere := filepath.Join(t.TempDir(), "bothy")
	if err := os.WriteFile(elsewhere, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(elsewhere); err != nil {
		t.Error("removed a bothy binary outside its own install path")
	}
}

// The isolation guarantee covers what bothy writes; it does not automatically
// cover what the tools bothy runs decide to write. `ya pkg add` clones the
// whole yazi-rs/plugins repository into a package cache, and without
// XDG_CACHE_HOME redirected that landed in ~/.cache/yazi — 1.1 MB and 91 files
// outside bothy's tree, which uninstall could not reach.
func TestPluginInstallCachesInsideBothysTree(t *testing.T) {
	p := sandbox(t)
	if _, err := exec.LookPath("ya"); err != nil {
		t.Skip("ya is not installed")
	}

	home := p.Home
	before := countFiles(t, filepath.Join(home, ".cache"))

	if _, err := EnsureYaziPlugins(p, false); err != nil {
		t.Fatal(err)
	}

	if after := countFiles(t, filepath.Join(home, ".cache")); after != before {
		t.Errorf("plugin install wrote %d file(s) into ~/.cache, outside bothy's tree",
			after-before)
	}
	// And everything it did write is inside the tree, where uninstall reaches.
	for _, f := range walkFiles(t, home) {
		if !strings.HasPrefix(f, p.BothyDir()) {
			t.Errorf("wrote outside bothy's tree: %s", f)
		}
	}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	return len(walkFiles(t, dir))
}

func walkFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, path)
		}
		return nil
	})
	return out
}

// Uninstalling twice must still finish the job. An earlier version returned
// early when the tree was already gone, so every step after that — including
// removing the binary — was skipped, and `bothy uninstall` twice left bothy
// installed. Each step has to be independent of the others.
func TestUninstallWithNoTreeStillRemovesTheBinary(t *testing.T) {
	p := sandbox(t)
	if err := os.MkdirAll(p.LocalBin, 0o755); err != nil {
		t.Fatal(err)
	}
	self := filepath.Join(p.LocalBin, "bothy")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// No install: bothy's tree does not exist at all.
	if _, err := os.Stat(p.BothyDir()); err == nil {
		t.Fatal("precondition: the tree should not exist")
	}

	if _, err := Uninstall(p, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(self); err == nil {
		t.Error("no tree to remove, so the binary was left behind too")
	}
}

// The same thing, the way a user meets it: uninstall, then uninstall again.
func TestUninstallIsIdempotentAndFinishes(t *testing.T) {
	p := sandbox(t)
	if err := os.MkdirAll(p.LocalBin, 0o755); err != nil {
		t.Fatal(err)
	}
	self := filepath.Join(p.LocalBin, "bothy")
	if err := os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(p, config.Default(), Options{Offline: true}); err != nil {
		t.Fatal(err)
	}

	// First pass takes the tree. Pretend it did not take the binary, which is
	// exactly the state the bug left behind.
	if _, err := Uninstall(p, false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatal("precondition: --keep-binary should have kept it")
	}

	// Second pass has no tree left, and must still finish.
	if _, err := Uninstall(p, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(self); err == nil {
		t.Error("a second uninstall left bothy installed")
	}
}

// Home is shared between a host and its toolboxes; PATH is not. An install run
// inside a container resolves tools to /usr/bin paths that do not exist on the
// host, so `bothy` launched from the host opened a pane that died with
// "command not found: yazi". The container it was installed in is recorded so
// the launch can go back to where the tools actually are.
func TestLaunchGoesBackToTheContainerItWasInstalledIn(t *testing.T) {
	p := sandbox(t)
	p.Container = platform.Toolbx
	p.ContainerName = "aip"

	if _, err := Run(p, config.Default(), Options{Offline: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureTools(p, config.Default(), true, nil); err != nil {
		t.Fatal(err)
	}
	if got := InstalledIn(p); got != "aip" {
		t.Fatalf("InstalledIn() = %q, want aip — recorded even offline", got)
	}

	// Now the same tree, seen from the host: no container detected.
	host := p
	host.Container = platform.NotContainer
	host.ContainerName = ""
	if got := ContainerFor(host, config.Default()); got != "aip" {
		t.Errorf("from the host, ContainerFor() = %q, want aip", got)
	}
}

// Precedence: an explicit setting beats the current container, which beats
// where the install happened.
func TestContainerPrecedence(t *testing.T) {
	cfg := config.Default()
	inContainer := platform.Info{Container: platform.Toolbx, ContainerName: "here"}

	if got := cfg.ContainerFor(inContainer, "installed"); got != "here" {
		t.Errorf("got %q, want the current container", got)
	}
	cfg.Workspace.Container = "chosen"
	if got := cfg.ContainerFor(inContainer, "installed"); got != "chosen" {
		t.Errorf("got %q, want the explicit setting", got)
	}
	if got := config.Default().ContainerFor(platform.Info{}, "installed"); got != "installed" {
		t.Errorf("got %q, want the install container as the fallback", got)
	}
}
