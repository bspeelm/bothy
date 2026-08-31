// Package config is the user's answer to "what should my workspace be".
//
// It holds slot choices, theme selection and the handful of machine-specific
// values that used to be hardcoded in a hand-edited .bashrc — the container
// name and the pinned project directory in particular.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/theme"
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
	// it is a nice touch, not a feature, and it needs per-layout measuring.
	Watermark bool `toml:"watermark"`
}

// DefaultExtras is the set of small CLI tools the workspace assumes are there.
// delta is in the list because the git pager wiring is part of the setup being
// ported; the rest are what Yazi's previews and the side pane lean on.
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
		Extras: append([]string(nil), DefaultExtras...),
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
	if err := toml.Unmarshal(src, &cfg); err != nil {
		return cfg, fmt.Errorf("config: %s: %w", Path(p), err)
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

// Validate catches the configuration mistakes that would otherwise surface as
// a broken workspace rather than an error message.
func (c Config) Validate() error {
	if c.Profile == "" {
		return fmt.Errorf("config: profile is empty")
	}
	return nil
}

// Palette resolves the configured theme, expanding ~ in the pack path so that
// a hand-edited config.toml behaves the way its author expected.
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
	return expand(c.Theme.Palette, p.Home)
}

// ContainerFor returns the container `dev` should enter, preferring an explicit
// setting over detection.
func (c Config) ContainerFor(p platform.Info) string {
	if c.Workspace.Container != "" {
		return c.Workspace.Container
	}
	return p.ContainerName
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
	case "workspace.watermark":
		c.Workspace.Watermark = value == "true" || value == "1" || value == "yes"
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	// Only the assigned value is validated here, not the configuration as a
	// whole. Cross-field rules — a PRO variant needing a pack — are checked at
	// install time instead, because enforcing them per-assignment makes the
	// natural ordering impossible: you cannot set the variant before the pack,
	// and you cannot set the pack for a variant you were not allowed to select.
	return nil
}

// Incomplete returns the cross-field problem with a configuration, if any, as
// something to report rather than refuse. Callers that must have a usable
// configuration should use Validate.
func (c Config) Incomplete() error { return c.Validate() }

func expand(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
