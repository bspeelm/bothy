package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

// #49. `ya pkg add` records a revision, but it records whatever HEAD was on
// that machine that day, so two installs a week apart got different plugins.
// A plugin without a pin reintroduces exactly that.
func TestEveryPluginIsPinned(t *testing.T) {
	plugins, err := YaziPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) == 0 {
		t.Fatal("no plugins loaded, so this test asserts nothing")
	}
	for _, pl := range plugins {
		if len(pl.Rev) != 40 {
			t.Errorf("%s is pinned at %q; want a full 40-character sha, "+
				"because a short one can become ambiguous as the repository grows",
				pl.Name, pl.Rev)
		}
		if strings.Trim(pl.Rev, "0123456789abcdef") != "" {
			t.Errorf("%s: rev %q is not a hex sha", pl.Name, pl.Rev)
		}
	}
}

// What bothy writes for `ya pkg install` to act on, and what it reads back to
// decide whether a plugin is at the revision it should be.
func TestPackageFileRoundTrips(t *testing.T) {
	p := sandboxPlatform(t)
	if err := os.MkdirAll(YaziDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := []Plugin{
		{Name: "git", Use: "yazi-rs/plugins:git", Rev: strings.Repeat("a", 40)},
		{Name: "chmod", Use: "yazi-rs/plugins:chmod", Rev: strings.Repeat("b", 40)},
	}
	if err := writePackageFile(p, plugins); err != nil {
		t.Fatal(err)
	}
	got := installedRevs(p)
	for _, pl := range plugins {
		if got[pl.Use] != pl.Rev {
			t.Errorf("%s read back as %q, want %q", pl.Name, got[pl.Use], pl.Rev)
		}
	}
}

// A plugin sitting at some other revision is not "installed". Without this,
// moving a pin would change the repository and nothing else: the directory
// exists, so the old copy would stay forever.
func TestAPluginAtTheWrongRevisionCountsAsMissing(t *testing.T) {
	p := sandboxPlatform(t)
	plugins, err := YaziPlugins()
	if err != nil {
		t.Fatal(err)
	}
	first := plugins[0]

	// Present on disk, but recorded at a revision that is not the pinned one.
	if err := os.MkdirAll(filepath.Join(YaziDir(p), "plugins", first.Name+".yazi"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := append([]Plugin(nil), plugins...)
	stale[0].Rev = strings.Repeat("f", 40)
	if err := writePackageFile(p, stale); err != nil {
		t.Fatal(err)
	}

	// Offline, so nothing is fetched and the classification is all that runs.
	rep, err := EnsureYaziPlugins(p, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, pl := range rep.Present {
		if pl.Name == first.Name {
			t.Fatalf("%s counted as present while sitting at the wrong revision, "+
				"so moving its pin would never take effect", first.Name)
		}
	}
}

// And one at the pinned revision is left alone, so an install that has nothing
// to do runs no subprocess at all.
func TestAPluginAtThePinnedRevisionIsLeftAlone(t *testing.T) {
	p := sandboxPlatform(t)
	plugins, err := YaziPlugins()
	if err != nil {
		t.Fatal(err)
	}
	for _, pl := range plugins {
		if err := os.MkdirAll(filepath.Join(YaziDir(p), "plugins", pl.Name+".yazi"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := writePackageFile(p, plugins); err != nil {
		t.Fatal(err)
	}

	rep, err := EnsureYaziPlugins(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Present) != len(plugins) {
		t.Errorf("%d of %d plugins recognised as present", len(rep.Present), len(plugins))
	}
	if len(rep.Failed) != 0 {
		t.Errorf("offline install reported failures for plugins already at their pin: %+v", rep.Failed)
	}
}

func sandboxPlatform(t *testing.T) platform.Info {
	t.Helper()
	home := t.TempDir()
	p := platform.Info{
		OS: "linux", Arch: "x86_64",
		Home:      home,
		DataDir:   filepath.Join(home, ".local", "share"),
		ConfigDir: filepath.Join(home, ".config"),
		LocalBin:  filepath.Join(home, ".local", "bin"),
	}
	if err := os.MkdirAll(YaziDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// `ya` records a hash of the plugin directory and refuses to touch a plugin
// whose contents no longer match it -- that is how it spots edits made by
// hand. bothy wrote the field empty on every run, which reads as an edit, so
// bumping a pin aborted with "you have modified the contents of the plugin".
// That is the one operation slots/yazi.toml documents.
func TestWritePackageFileKeepsTheHashYaRecorded(t *testing.T) {
	p := sandboxPlatform(t)
	if err := os.MkdirAll(YaziDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	// What ya leaves behind after a successful install.
	existing := `[plugin]

[[plugin.deps]]
use = "yazi-rs/plugins:git"
rev = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
hash = "5bb0bfab901d3601c370eafdd66edd31"
`
	if err := os.WriteFile(packagePath(p), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bump the pin, as `bothy install` does after a rev changes in slots/.
	err := writePackageFile(p, []Plugin{{
		Name: "git", Use: "yazi-rs/plugins:git",
		Rev: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}})
	if err != nil {
		t.Fatal(err)
	}

	deps := installedDeps(p)
	if len(deps) != 1 {
		t.Fatalf("wrote %d deps, want 1", len(deps))
	}
	if deps[0].Rev != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Errorf("rev = %q, want the bumped one", deps[0].Rev)
	}
	if deps[0].Hash != "5bb0bfab901d3601c370eafdd66edd31" {
		t.Errorf("hash = %q, want ya's record carried across; ya reads an empty "+
			"hash as a hand-edited plugin and refuses to update it", deps[0].Hash)
	}
}

// A plugin ya has never installed has no hash to carry, and must not gain one.
func TestWritePackageFileInventsNoHash(t *testing.T) {
	p := sandboxPlatform(t)
	if err := os.MkdirAll(YaziDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writePackageFile(p, []Plugin{{Name: "git", Use: "a/b:git", Rev: "c"}}); err != nil {
		t.Fatal(err)
	}
	if got := installedDeps(p)[0].Hash; got != "" {
		t.Errorf("hash = %q for a plugin never installed, want empty", got)
	}
}
