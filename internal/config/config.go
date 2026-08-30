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
	Workspace Workspace `toml:"workspace"`
	Extras    []string  `toml:"extras"`
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

// Theme selects the palette. Variant "open" needs nothing else; any Dracula
// PRO variant needs ProPack pointing at the user's own licensed copy.
type Theme struct {
	Provider string `toml:"provider"`
	Variant  string `toml:"variant"`
	ProPack  string `toml:"pro_pack"`
	// Font is a font directory name inside the PRO pack's fonts/ (e.g.
	// "fira-code"). Empty means bothy does not touch the terminal's font.
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
			Variant:  "open",
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
		"# Slots pick a provider per component; theme.variant \"open\" is the\n" +
		"# freely-licensed Dracula palette. To use Dracula PRO, set variant to one\n" +
		"# of pro/blade/buffy/lincoln/morbius/van-helsing/alucard and point\n" +
		"# theme.pro_pack at your own copy of the pack.\n\n"
	return os.WriteFile(path, append([]byte(header), out...), 0o644)
}

// Validate catches the configuration mistakes that would otherwise surface as
// a broken workspace rather than an error message.
func (c Config) Validate() error {
	if c.Theme.Variant != "" && c.Theme.Variant != "open" {
		if !theme.IsProVariant(c.Theme.Variant) {
			return fmt.Errorf("config: theme.variant %q is not open or a Dracula PRO variant (%s)",
				c.Theme.Variant, strings.Join(theme.ProVariants, ", "))
		}
		if c.Theme.ProPack == "" {
			return fmt.Errorf("config: theme.variant = %q needs theme.pro_pack set to your own copy\n"+
				"        of the Dracula PRO pack — bothy ships no PRO colours of its own", c.Theme.Variant)
		}
	}
	return nil
}

// Palette resolves the configured theme, expanding ~ in the pack path so that
// a hand-edited config.toml behaves the way its author expected.
func (c Config) Palette(p platform.Info) (theme.Palette, error) {
	return theme.Resolve(c.Theme.Variant, expand(c.Theme.ProPack, p.Home))
}

// ProPackPath is the expanded pack directory, or "" when none is configured.
func (c Config) ProPackPath(p platform.Info) string {
	return expand(c.Theme.ProPack, p.Home)
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
	case "theme.variant":
		if value != "" && value != "open" && !theme.IsProVariant(value) {
			return fmt.Errorf("config: theme.variant %q is not open or a Dracula PRO variant (%s)",
				value, strings.Join(theme.ProVariants, ", "))
		}
		c.Theme.Variant = value
	case "theme.pro_pack":
		c.Theme.ProPack = value
	case "theme.font":
		c.Theme.Font = value
	case "workspace.container":
		c.Workspace.Container = value
	case "workspace.project_dir":
		c.Workspace.ProjectDir = value
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
