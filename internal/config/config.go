// Package config is the user's answer to "what should my workspace be":
// slot choices, theme, and the machine-specific values such as container
// name and project directory.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/theme"
)

// Config is ~/.config/bothy/config.toml.
type Config struct {
	Profile   string    `toml:"profile"`
	Slots     Slots     `toml:"slots"`
	Theme     Theme     `toml:"theme"`
	Editor    Editor    `toml:"editor"`
	Workspace Workspace `toml:"workspace"`
	Extras    []string  `toml:"extras"`
	// Passthrough names slots that should use your own config directory
	// instead of bothy's. It is one environment variable per slot at launch,
	// not a second code path. See PLAN.md §5.
	Passthrough []string `toml:"passthrough"`

	// Unknown holds keys in config.toml that bothy does not recognise. It is
	// populated by Load and never written back -- a config carried between
	// machines in git must not grow a record of its own typos.
	Unknown []string `toml:"-"`
}

// Editor holds the one editor setting bothy has an opinion about.
type Editor struct {
	// ProvideConfig generates a vimrc and colorscheme inside bothy's tree and
	// launches vim against them. Off by default: your editor config is yours,
	// and a workspace tool replacing it is overreach. Worth turning on for a
	// fresh machine with no vim config at all.
	ProvideConfig bool `toml:"provide_config"`
}

// Slots names the provider chosen for each slot. A slot with an empty value
// falls back to the default in Default(); "none" disables it.
type Slots struct {
	Terminal string `toml:"terminal"`
	Mux      string `toml:"mux"`
	Browser  string `toml:"browser"`
	Editor   string `toml:"editor"`
	Agent    string `toml:"agent"`
}

// Theme selects the palette.
//
// Provider names a built-in palette; only open Dracula is built in, because it
// is the only palette whose values this project may carry. Palette points at a
// file of your own and wins when set — that is the way in for any other
// palette, licensed or not, and it keeps that palette on your machine.
type Theme struct {
	Provider string `toml:"provider"`
	Palette  string `toml:"palette"`
	// VimColorscheme uses a colorscheme you already have installed instead of
	// the one bothy generates from the palette. Give the name vim knows it by.
	VimColorscheme string `toml:"vim_colorscheme"`
	// Font is a font family for the terminal, e.g. "Fira Code". Empty leaves
	// the terminal's own font setting alone.
	Font string `toml:"font"`
}

// Workspace holds the values that make `dev` portable instead of personal.
type Workspace struct {
	// Container overrides the detected Toolbx/Distrobox name that `dev` hops
	// into from the host. Empty means "use whatever was detected".
	Container string `toml:"container"`
	// ProjectDir pins `dev` to one directory. Empty means the current one.
	ProjectDir string `toml:"project_dir"`
	// Watermark enables the Ghostty background-image trick. Off by default:
	// it needs per-layout measuring to look right.
	Watermark bool `toml:"watermark"`
	// PaneFrames is "full", "titles" or "none".
	//
	// Set explicitly rather than left to Zellij, whose default changed to
	// "titles" in 0.45 — the workspace should look the same across zellij
	// versions rather than following an upstream default.
	PaneFrames string `toml:"pane_frames"`
}

// DefaultExtras are the CLI tools Yazi's previews, search and jump commands
// and the side pane lean on. delta is the exception — nothing bothy generates
// references it; see issue #45.
var DefaultExtras = []string{"lazygit", "delta", "fzf", "ripgrep", "fd", "zoxide", "jq"}

// Default is the shipped configuration: the origin setup, with every
// machine-specific value left blank for detection to fill in.
func Default() Config {
	return Config{
		Profile: "cockpit",
		Slots: Slots{
			Terminal: "ghostty",
			Mux:      "zellij",
			Browser:  "yazi",
			Editor:   "vim",
			Agent:    "claude-code",
		},
		Theme: Theme{
			Provider: "dracula",
		},
		Workspace: Workspace{PaneFrames: "full"},
		Extras:    append([]string(nil), DefaultExtras...),
	}
}

// Path returns the config file location for a detected machine.
func Path(p platform.Info) string {
	return filepath.Join(p.ConfigDir, "bothy", "config.toml")
}

// Load reads the config, filling unset fields from Default. A missing file is
// not an error: it means "everything default", which is what a fresh install is.
func Load(p platform.Info) (Config, error) {
	cfg := Default()

	src, err := os.ReadFile(Path(p))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}

	// Unmarshal over the defaults so an absent key keeps its default rather
	// than becoming the zero value.
	//
	// Strict, but only to *collect* what it does not recognise. An unknown key
	// is not an error: a config that refuses to load is worse than a typo, and
	// a key written by a newer bothy must not brick an older one, or carrying
	// ~/.config/bothy between machines in git breaks in the other direction.
	// The decoder populates the struct fully either way.
	dec := toml.NewDecoder(bytes.NewReader(src))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		var unknown *toml.StrictMissingError
		if !errors.As(err, &unknown) {
			return cfg, fmt.Errorf("config: %s: %w", Path(p), err)
		}
		for _, e := range unknown.Errors {
			cfg.Unknown = append(cfg.Unknown, strings.Join(e.Key(), "."))
		}
	}
	// A half-finished configuration still loads. Refusing here would make every
	// command — including the `config set` that would complete it, and the
	// `doctor` that would explain it — fail with the same message. Install
	// reports the problem when it actually matters.
	return cfg, nil
}

// Save writes the config, creating its directory.
func Save(p platform.Info, cfg Config) error {
	path := Path(p)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	out, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	header := "# bothy configuration\n" +
		"# Slots pick a provider per component. The built-in theme is open\n" +
		"# Dracula; for any other palette, including one you have licensed,\n" +
		"# write the eleven colours into a file of your own and set\n" +
		"# theme.palette to it (bothy theme example prints a blank one).\n\n"
	return os.WriteFile(path, append([]byte(header), out...), 0o644)
}

// slotNames are the slots a passthrough entry may name. A test asserts it
// matches Slots.
var slotNames = []string{"terminal", "mux", "browser", "editor", "agent"}

// Validate catches the configuration mistakes that would otherwise surface as
// a broken workspace rather than an error message.
func (c Config) Validate() error {
	if c.Profile == "" {
		return fmt.Errorf("config: profile is empty")
	}
	// Cross-field rules install already enforces by failing later. Checking
	// them here means a `bothy config set` mistake surfaces at the next
	// command rather than as a workspace that opens wrong.
	for _, name := range c.Passthrough {
		if !slices.Contains(slotNames, name) {
			return fmt.Errorf("config: passthrough names %q, which is not a slot (%s)",
				name, strings.Join(slotNames, ", "))
		}
	}
	if c.Workspace.PaneFrames != "" {
		switch c.Workspace.PaneFrames {
		case "full", "titles", "none":
		default:
			return fmt.Errorf("config: workspace.pane_frames is %q, want full, titles or none",
				c.Workspace.PaneFrames)
		}
	}
	return nil
}

// Palette resolves the configured theme, expanding ~ in the palette path so
// that a hand-edited config.toml behaves the way its author expected.
func (c Config) Palette(p platform.Info) (theme.Palette, error) {
	return theme.Resolve(c.Theme.Provider, c.PalettePath(p))
}

// PassesThrough reports whether a slot uses the user's own config directory.
func (c Config) PassesThrough(slot string) bool {
	for _, s := range c.Passthrough {
		if s == slot {
			return true
		}
	}
	return false
}

// PalettePath is the expanded custom palette file, or "" when none is set.
func (c Config) PalettePath(p platform.Info) string {
	return Expand(c.Theme.Palette, p.Home)
}

// ContainerFor returns the container to enter, preferring an explicit setting,
// then the current one, then wherever the install happened.
//
// That last fallback is the one that matters. Home is shared between host and
// container but PATH is not: an install run inside a toolbox resolves its tools
// to /usr/bin paths that do not exist on the host, so launching from the host
// finds nothing and a pane dies with "command not found". installedIn is
// recorded at install time precisely so the launch can go back.
func (c Config) ContainerFor(p platform.Info, installedIn string) string {
	if c.Workspace.Container != "" {
		return c.Workspace.Container
	}
	if p.ContainerName != "" {
		return p.ContainerName
	}
	return installedIn
}

// Set applies a dotted key assignment, as used by `bothy config set`.
func (c *Config) Set(key, value string) error {
	switch key {
	case "profile":
		c.Profile = value
	case "slots.terminal":
		c.Slots.Terminal = value
	case "slots.mux":
		c.Slots.Mux = value
	case "slots.browser":
		c.Slots.Browser = value
	case "slots.editor":
		c.Slots.Editor = value
	case "slots.agent":
		c.Slots.Agent = value
	case "theme.provider":
		c.Theme.Provider = value
	case "theme.palette":
		c.Theme.Palette = value
	case "theme.vim_colorscheme":
		c.Theme.VimColorscheme = value
	case "theme.font":
		c.Theme.Font = value
	case "workspace.container":
		c.Workspace.Container = value
	case "workspace.project_dir":
		c.Workspace.ProjectDir = value
	case "passthrough":
		// A comma-separated list, or "" to clear it.
		c.Passthrough = nil
		for _, slot := range strings.Split(value, ",") {
			if slot = strings.TrimSpace(slot); slot != "" {
				c.Passthrough = append(c.Passthrough, slot)
			}
		}
	case "editor.provide_config":
		c.Editor.ProvideConfig = value == "true" || value == "1" || value == "yes"
	case "workspace.pane_frames":
		switch value {
		case "full", "titles", "none":
			c.Workspace.PaneFrames = value
		default:
			return fmt.Errorf("config: workspace.pane_frames must be full, titles or none")
		}
	case "workspace.watermark":
		c.Workspace.Watermark = value == "true" || value == "1" || value == "yes"
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	// Only the assigned value is validated here, not the configuration as a
	// whole. Cross-field rules are checked at install time instead, because
	// enforcing them per-assignment would make some orderings impossible to
	// type: a pair of keys that are only valid together cannot both be set
	// first.
	return nil
}

// Expand resolves a leading ~ against home. Exported because the CLI expands
// --dir the same way.
func Expand(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
