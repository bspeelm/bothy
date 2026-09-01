package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

// Set is the whole of `bothy config set`, and it was a 45-case switch with no
// coverage at all. The risk in a switch this shape is not that a case is wrong
// -- it is that a key is spelled one way here and another in the usage text,
// or that two keys write the same field. Round-tripping every key catches both.
func TestSetWritesTheFieldItNames(t *testing.T) {
	for _, tc := range []struct {
		key, value string
		read       func(Config) string
	}{
		{"profile", "editor", func(c Config) string { return c.Profile }},
		{"slots.terminal", "ghostty", func(c Config) string { return c.Slots.Terminal }},
		{"slots.mux", "zellij", func(c Config) string { return c.Slots.Mux }},
		{"slots.browser", "yazi", func(c Config) string { return c.Slots.Browser }},
		{"slots.editor", "helix", func(c Config) string { return c.Slots.Editor }},
		{"slots.agent", "none", func(c Config) string { return c.Slots.Agent }},
		{"theme.provider", "dracula", func(c Config) string { return c.Theme.Provider }},
		{"theme.palette", "/tmp/p.toml", func(c Config) string { return c.Theme.Palette }},
		{"theme.vim_colorscheme", "dracula", func(c Config) string { return c.Theme.VimColorscheme }},
		{"theme.font", "Fira Code", func(c Config) string { return c.Theme.Font }},
		{"workspace.container", "bothy-test", func(c Config) string { return c.Workspace.Container }},
		{"workspace.project_dir", "/w", func(c Config) string { return c.Workspace.ProjectDir }},
		{"workspace.pane_frames", "titles", func(c Config) string { return c.Workspace.PaneFrames }},
	} {
		t.Run(tc.key, func(t *testing.T) {
			c := Default()
			if err := c.Set(tc.key, tc.value); err != nil {
				t.Fatalf("Set(%q) = %v", tc.key, err)
			}
			if got := tc.read(c); got != tc.value {
				t.Errorf("Set(%q, %q) then read = %q", tc.key, tc.value, got)
			}
		})
	}
}

func TestSetRejectsAnUnknownKey(t *testing.T) {
	c := Default()
	err := c.Set("slots.mux.name", "zellij")
	if err == nil {
		t.Fatal("an unknown key was accepted silently")
	}
	// The message has to carry the key back, or a typo is unfindable.
	if !strings.Contains(err.Error(), "slots.mux.name") {
		t.Errorf("the error does not name the key: %v", err)
	}
}

// pane_frames is the one key with a closed set of values, so it is the one
// place a bad value can be caught at assignment rather than at install.
func TestSetRejectsABadPaneFrames(t *testing.T) {
	c := Default()
	if err := c.Set("workspace.pane_frames", "sometimes"); err == nil {
		t.Error("workspace.pane_frames accepted a value it does not understand")
	}
	if c.Workspace.PaneFrames == "sometimes" {
		t.Error("the rejected value was written anyway")
	}
}

// The booleans take several spellings because people type all of them, and
// anything else means false rather than an error.
func TestSetParsesBooleans(t *testing.T) {
	for _, v := range []string{"true", "1", "yes"} {
		c := Default()
		if err := c.Set("workspace.watermark", v); err != nil {
			t.Fatal(err)
		}
		if !c.Workspace.Watermark {
			t.Errorf("watermark = false after setting %q", v)
		}
	}
	c := Default()
	if err := c.Set("workspace.watermark", "no"); err != nil {
		t.Fatal(err)
	}
	if c.Workspace.Watermark {
		t.Error(`watermark = true after setting "no"`)
	}
}

// passthrough is the only key that parses a list, and the only one where
// setting "" has to mean "clear" rather than "one empty entry".
func TestSetPassthroughParsesAndClears(t *testing.T) {
	c := Default()
	if err := c.Set("passthrough", " yazi , zellij "); err != nil {
		t.Fatal(err)
	}
	if len(c.Passthrough) != 2 || c.Passthrough[0] != "yazi" || c.Passthrough[1] != "zellij" {
		t.Fatalf("Passthrough = %q, want the two slots trimmed", c.Passthrough)
	}
	if !c.PassesThrough("yazi") || c.PassesThrough("vim") {
		t.Error("PassesThrough disagrees with the list it was given")
	}
	if err := c.Set("passthrough", ""); err != nil {
		t.Fatal(err)
	}
	if len(c.Passthrough) != 0 {
		t.Errorf("Passthrough = %q after clearing", c.Passthrough)
	}
}

func TestValidateRejectsAnEmptyProfile(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("the default config does not validate: %v", err)
	}
	c.Profile = ""
	if err := c.Validate(); err == nil {
		t.Error("an empty profile validated; install would render nothing")
	}
}

func TestExpandResolvesTilde(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"~", "/home/x"},
		{"~/p.toml", "/home/x/p.toml"},
		{"/etc/p.toml", "/etc/p.toml"},
		{"", ""},
		// Only a leading "~/" is a home reference. "~foo" is another user's
		// home to a shell and a plain relative path here; either way it is not
		// this user's home and must not be rewritten as though it were.
		{"~foo/p.toml", "~foo/p.toml"},
	} {
		if got := Expand(tc.in, "/home/x"); got != tc.want {
			t.Errorf("Expand(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The container the workspace runs in is the one bothy installed into, unless
// the config overrides it. Getting this backwards sends `dev` to the wrong box.
func TestContainerForPrefersTheConfiguredName(t *testing.T) {
	var p platform.Info
	c := Default()
	if got := c.ContainerFor(p, "recorded"); got != "recorded" {
		t.Errorf("ContainerFor() = %q, want the recorded install container", got)
	}
	c.Workspace.Container = "configured"
	if got := c.ContainerFor(p, "recorded"); got != "configured" {
		t.Errorf("ContainerFor() = %q, want the configured container to win", got)
	}
}

// A typo used to cost nothing to make and everything to find: Unmarshal over
// the defaults accepted any key, so `slots.borwser = "yazi"` loaded cleanly,
// did nothing, and kept doing nothing on every machine the config was carried
// to.
func TestLoadCollectsUnknownKeysWithoutFailing(t *testing.T) {
	p := writeConfig(t, `
profile = 'cockpit'

[slots]
browser = 'yazi'
borwser = 'yazi'

[bogus]
x = 1
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("an unknown key made the config fail to load: %v", err)
	}
	// The recognised keys must still have been applied -- collecting the
	// unknown ones must not cost the known ones.
	if cfg.Profile != "cockpit" || cfg.Slots.Browser != "yazi" {
		t.Errorf("recognised keys were lost: profile=%q browser=%q", cfg.Profile, cfg.Slots.Browser)
	}
	want := map[string]bool{"slots.borwser": true, "bogus": true}
	if len(cfg.Unknown) != len(want) {
		t.Fatalf("Unknown = %q, want %d entries", cfg.Unknown, len(want))
	}
	for _, k := range cfg.Unknown {
		if !want[k] {
			t.Errorf("unexpected unknown key %q", k)
		}
	}
}

func TestLoadReportsNoUnknownKeysForAGoodConfig(t *testing.T) {
	p := writeConfig(t, "profile = 'cockpit'\n\n[slots]\nbrowser = 'yazi'\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Unknown) != 0 {
		t.Errorf("Unknown = %q for a config with no typos", cfg.Unknown)
	}
}

// Unknown must never be written back: a config carried between machines in
// git should not grow a record of its own typos.
func TestSaveDoesNotWriteUnknownKeys(t *testing.T) {
	p := writeConfig(t, "profile = 'cockpit'\nbogus = 1\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Unknown) == 0 {
		t.Fatal("nothing was collected, so this test proves nothing")
	}
	if err := Save(p, cfg); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(Path(p))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "Unknown") || strings.Contains(string(src), "bogus") {
		t.Errorf("the rewritten config carries the unknown key:\n%s", src)
	}
}

func TestKeysCoversTheWholeStruct(t *testing.T) {
	keys := Keys()
	for _, want := range []string{
		"profile", "slots.browser", "slots.editor", "theme.palette",
		"workspace.container", "workspace.pane_frames", "editor.provide_config",
	} {
		if !slices.Contains(keys, want) {
			t.Errorf("Keys() is missing %q", want)
		}
	}
	// Unknown is toml:"-" and must not appear as a settable key.
	for _, k := range keys {
		if strings.EqualFold(k, "unknown") {
			t.Error("Keys() lists the Unknown field, which is not a config key")
		}
	}
}

func TestNearestSuggestsOnlyWhenItIsClose(t *testing.T) {
	if got := Nearest("slots.borwser"); got != "slots.browser" {
		t.Errorf("Nearest(slots.borwser) = %q", got)
	}
	if got := Nearest("profil"); got != "profile" {
		t.Errorf("Nearest(profil) = %q", got)
	}
	// Nothing close. Suggesting anything here would send someone to edit a
	// line that was never the problem.
	if got := Nearest("completely_unrelated_thing"); got != "" {
		t.Errorf("Nearest(completely_unrelated_thing) = %q, want no suggestion", got)
	}
}

// passthrough naming something that is not a slot used to surface as a
// workspace that opened wrong rather than as an error.
func TestValidateRejectsAPassthroughThatIsNotASlot(t *testing.T) {
	c := Default()
	c.Passthrough = []string{"browser", "kitchen-sink"}
	err := c.Validate()
	if err == nil {
		t.Fatal("passthrough accepted a name that is not a slot")
	}
	if !strings.Contains(err.Error(), "kitchen-sink") {
		t.Errorf("the error does not name the offender: %v", err)
	}

	c.Passthrough = []string{"browser", "editor"}
	if err := c.Validate(); err != nil {
		t.Errorf("real slots were rejected: %v", err)
	}
}

// The slot list Validate checks against has to match the Slots struct, or a
// new slot becomes un-passthrough-able for no stated reason.
func TestSlotNamesMatchTheSlotsStruct(t *testing.T) {
	var fields []string
	ty := reflect.TypeOf(Slots{})
	for i := 0; i < ty.NumField(); i++ {
		tag, _, _ := strings.Cut(ty.Field(i).Tag.Get("toml"), ",")
		fields = append(fields, tag)
	}
	slices.Sort(fields)
	got := slices.Clone(slotNames)
	slices.Sort(got)
	if !slices.Equal(fields, got) {
		t.Errorf("slotNames = %q, Slots struct has %q", got, fields)
	}
}

// writeConfig plants a config.toml in a temporary home and returns a
// platform.Info pointing at it.
func writeConfig(t *testing.T, body string) platform.Info {
	t.Helper()
	home := t.TempDir()
	p := platform.Info{
		Home:      home,
		ConfigDir: filepath.Join(home, ".config"),
		DataDir:   filepath.Join(home, ".local", "share"),
	}
	if err := os.MkdirAll(filepath.Dir(Path(p)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(p), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
