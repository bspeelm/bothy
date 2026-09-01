// Package config is the user's answer to "what should my workspace be":
// slot choices, theme, and the machine-specific values such as container
// name and project directory.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
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

	// Unreadable holds keys bothy recognises but could not read, because the
	// value is the wrong type -- usually a key whose type changed between
	// versions. Reported differently from Unknown, because "did you mean
	// background_image?" about background_image helps nobody.
	Unreadable []string `toml:"-"`

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
	// Launch decides whether `bothy` opens its own window or runs where it was
	// typed: "auto" opens one only when this terminal cannot draw images,
	// "here" never does, "window" always does. --in-place and --window
	// override it for a single run.
	Launch string `toml:"launch"`
	// BackgroundImage sits behind the terminal, given as a path to a file of
	// your own. Empty means none.
	//
	// A path rather than a switch because bothy ships no art: the trick wants
	// an image composited where one pane will be, which depends on your
	// screen, and a picture bothy chose would be wrong on most of them. See
	// docs/watermark.md.
	//
	// Named for what it is rather than what it is for. The rename is what lets
	// a config written by an older bothy -- where this key was a boolean
	// called `watermark` -- meet the unknown-key path instead of a type error.
	BackgroundImage string `toml:"background_image"`
	// PaneFrames is "full", "titles" or "none".
	//
	// Set explicitly rather than left to Zellij, whose default changed to
	// "titles" in 0.45 — the workspace should look the same across zellij
	// versions rather than following an upstream default.
	PaneFrames string `toml:"pane_frames"`
}

// DefaultExtras are the CLI tools Yazi's previews, search and jump commands
// and the side pane lean on.
var DefaultExtras = []string{"lazygit", "fzf", "ripgrep", "fd", "zoxide", "jq"}

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
		Workspace: Workspace{PaneFrames: "full", Launch: "auto"},
		Extras:    append([]string(nil), DefaultExtras...),
	}
}

// withoutUnreadableKeys removes keys the decoder cannot assign, returning the
// remaining TOML and the names it dropped.
//
// A key whose *type* changed between versions is the same problem as one whose
// name changed, and #31 settled that answer: warn, never refuse. The decoder
// stops at the first such key, so it has to be removed and the decode retried
// -- otherwise every key after it would quietly keep its default, which is a
// worse failure than the error it replaced.
//
// The key is found by position rather than by name: go-toml reports a row and
// column for a type error and leaves Key() empty, so the line is what there is
// to go on.
//
// Bounded because the loop is driven by an error: a decoder reporting a line
// this cannot remove would otherwise spin.
func withoutUnreadableKeys(src []byte) ([]byte, []string) {
	var dropped []string
	for range 16 {
		var probe Config
		err := toml.Unmarshal(src, &probe)
		var bad *toml.DecodeError
		if err == nil || !errors.As(err, &bad) {
			return src, dropped
		}
		row, _ := bad.Position()
		trimmed, name, ok := dropLine(src, row)
		if !ok {
			return src, dropped
		}
		// Only a cut that leaves valid TOML is accepted. A value spanning
		// several lines would otherwise be half-removed, turning one bad key
		// into a broken file -- which is the failure this exists to prevent.
		var tree map[string]any
		if toml.Unmarshal(trimmed, &tree) != nil {
			return src, dropped
		}
		src, dropped = trimmed, append(dropped, name)
	}
	return src, dropped
}

// dropLine removes one 1-indexed line and names the key it defined, qualified
// by whichever table it was under.
func dropLine(src []byte, row int) ([]byte, string, bool) {
	lines := strings.Split(string(src), "\n")
	if row < 1 || row > len(lines) {
		return nil, "", false
	}
	key, _, found := strings.Cut(lines[row-1], "=")
	key = strings.TrimSpace(key)
	if !found || key == "" {
		return nil, "", false
	}
	for i := row - 2; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			key = strings.Trim(t, "[]") + "." + key
			break
		}
	}
	out := append(append([]string{}, lines[:row-1]...), lines[row:]...)
	return []byte(strings.Join(out, "\n")), key, true
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
	src, unreadable := withoutUnreadableKeys(src)
	cfg.Unreadable = unreadable

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
	// The constraints Set applies, checked again for a config.toml written by
	// hand. Stating them once means a new constrained key is enforced on both
	// paths rather than on whichever one its author remembered.
	for _, key := range slices.Sorted(maps.Keys(allowed)) {
		field, err := fieldFor(reflect.ValueOf(c), key)
		if err != nil || field.Kind() != reflect.String {
			continue
		}
		if err := check(key, field.String()); err != nil {
			return err
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
//
// The walk mirrors Keys(): both derive from the struct, so adding a field
// makes it listable and settable at once. A hand-written switch beside a
// reflect-derived Keys() drifts in one direction only -- towards keys that are
// offered, suggested, and then refused.
//
// Only the assigned value is checked. Cross-field rules are left to Validate
// at install time, because enforcing them per-assignment makes some orderings
// impossible to type: a pair of keys valid only together cannot both be set
// first.
func (c *Config) Set(key, value string) error {
	field, err := fieldFor(reflect.ValueOf(c).Elem(), key)
	if err != nil {
		return err
	}
	if err := check(key, value); err != nil {
		return err
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("config: %s takes true or false, not %q", key, value)
		}
		field.SetBool(b)
	case reflect.Slice:
		field.Set(reflect.ValueOf(splitList(value)))
	default:
		return fmt.Errorf("config: %s cannot be set from the command line", key)
	}
	return nil
}

// fieldFor resolves a dotted key to the struct field it names, walking the
// toml tags rather than the Go names so that the key someone types and the key
// the file uses are the same string.
func fieldFor(v reflect.Value, key string) (reflect.Value, error) {
	for _, part := range strings.Split(key, ".") {
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("config: unknown key %q", key)
		}
		found := false
		for i := 0; i < v.NumField(); i++ {
			tag, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("toml"), ",")
			if tag == part && tag != "" && tag != "-" {
				v, found = v.Field(i), true
				break
			}
		}
		if !found {
			return reflect.Value{}, fmt.Errorf("config: unknown key %q", key)
		}
	}
	return v, nil
}

// allowed lists the keys whose values are a closed set. Data rather than a
// function per key, so that anything needing to know what a key accepts --
// an error message, a test, one day a completion -- can read it.
var allowed = map[string][]string{
	"workspace.pane_frames": {"full", "titles", "none"},
	"workspace.launch":      {"auto", "here", "window"},
}

// check applies the constraint on a key, if it has one. An empty value always
// passes: it means "unset", which every key allows.
func check(key, value string) error {
	set, ok := allowed[key]
	if !ok || value == "" || slices.Contains(set, value) {
		return nil
	}
	return fmt.Errorf("config: %s wants one of %s, not %q", key, strings.Join(set, ", "), value)
}

// parseBool is stricter than strconv.ParseBool is lenient: a value it does not
// recognise is a typo, and a typo that reads as false is the kind of silence
// this config stopped accepting in 0.1.5.
func parseBool(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "true", "yes", "on", "1":
		return true, nil
	case "false", "no", "off", "0":
		return false, nil
	}
	return false, fmt.Errorf("not a boolean")
}

// splitList reads a comma-separated value, as every list key in config.toml
// takes from the command line. An empty value clears the list.
func splitList(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
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
