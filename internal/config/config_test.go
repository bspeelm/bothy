package config

import (
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
