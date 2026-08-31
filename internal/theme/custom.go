package theme

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Custom palettes.
//
// bothy ships exactly one palette — the freely-licensed open Dracula one in
// theme.go — and reads any other from a file the user points it at. There is no
// code here that knows about any particular commercial theme, and no colour
// value in this repository that is not open Dracula's.
//
// That is a deliberate boundary, not an oversight. A palette you have licensed
// is yours to use on your own machine; it is not bothy's to carry, parse the
// vendor's files for, or copy around. Writing eleven values into a small TOML
// file keeps the licensed material entirely outside this project, and has the
// side effect of making every other palette — Catppuccin, Nord, Gruvbox, your
// own — work by exactly the same route.

// paletteFile is the on-disk shape of a custom palette.
type paletteFile struct {
	Name  string `toml:"name"`
	Light bool   `toml:"light"`

	Fg        string `toml:"fg"`
	Bg        string `toml:"bg"`
	Comment   string `toml:"comment"`
	Selection string `toml:"selection"`

	Cyan   string `toml:"cyan"`
	Green  string `toml:"green"`
	Orange string `toml:"orange"`
	Pink   string `toml:"pink"`
	Purple string `toml:"purple"`
	Red    string `toml:"red"`
	Yellow string `toml:"yellow"`
}

var hexColour = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Load reads a palette from a TOML file.
func Load(path string) (Palette, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, fmt.Errorf("theme: reading palette %s: %w", path, err)
	}

	var f paletteFile
	if err := toml.Unmarshal(src, &f); err != nil {
		return Palette{}, fmt.Errorf("theme: %s: %w", path, err)
	}

	p := Palette{
		Name:      f.Name,
		Light:     f.Light,
		Fg:        strings.ToUpper(f.Fg),
		Bg:        strings.ToUpper(f.Bg),
		Comment:   strings.ToUpper(f.Comment),
		Selection: strings.ToUpper(f.Selection),
		Cyan:      strings.ToUpper(f.Cyan),
		Green:     strings.ToUpper(f.Green),
		Orange:    strings.ToUpper(f.Orange),
		Pink:      strings.ToUpper(f.Pink),
		Purple:    strings.ToUpper(f.Purple),
		Red:       strings.ToUpper(f.Red),
		Yellow:    strings.ToUpper(f.Yellow),
	}
	if p.Name == "" {
		p.Name = "custom"
	}

	if err := p.validate(path); err != nil {
		return Palette{}, err
	}
	return p, nil
}

// validate refuses a palette that would produce a half-themed workspace.
// A missing token renders as an empty string in a config file, which some tools
// accept and then draw unreadably — so this fails at install time instead.
func (p Palette) validate(source string) error {
	if miss := p.missing(); len(miss) > 0 {
		return fmt.Errorf("theme: %s is missing: %s\n"+
			"      a palette needs all eleven tokens; see 'bothy theme example'",
			source, strings.Join(miss, ", "))
	}
	for _, tok := range paletteTokens {
		v, _ := p.Color(tok)
		if !hexColour.MatchString(v) {
			return fmt.Errorf("theme: %s: %s = %q is not a #RRGGBB colour", source, tok, v)
		}
	}
	return nil
}

// ExampleFile is a blank palette for someone to fill in. It carries no colour
// values at all, so that a palette anyone has licensed stays on their machine
// and out of this repository.
const ExampleFile = `# A bothy palette.
#
# Eleven tokens theme the whole workspace: the multiplexer, the file browser,
# the terminal and a generated vim colorscheme. Fill them in and point bothy
# here:
#
#     bothy config set theme.palette ~/.config/bothy/my-palette.toml
#     bothy install
#
# If you have licensed a theme, copy its values in here. bothy neither ships
# nor parses any commercial theme's files — this file is yours, it stays on
# your machine, and nothing is redistributed.

name  = "my-palette"
light = false        # true for a light palette

fg        = ""       # body text
bg        = ""       # window background
comment   = ""       # de-emphasised text, borders
selection = ""       # selected background, inactive chrome

cyan   = ""
green  = ""
orange = ""
pink   = ""
purple = ""          # the accent: focused pane, cursor, active tab
red    = ""
yellow = ""
`
