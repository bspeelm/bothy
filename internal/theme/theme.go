// Package theme resolves one 11-token Palette for the whole workspace, from
// the built-in open Dracula palette or a file the user points bothy at
// (custom.go, ADR-006).
package theme

import (
	"fmt"
)

// paletteTokens is every colour a palette must define, and the canonical order
// they are reported in.
var paletteTokens = []string{
	"fg", "bg", "comment", "selection",
	"cyan", "green", "orange", "pink", "purple", "red", "yellow",
}

// Palette is the complete set of colours every template may reference.
// Keep it at these eleven tokens: a template that needs a twelfth colour is a
// sign the template is doing design work that belongs in the palette.
type Palette struct {
	Name  string // display name, e.g. "dracula"; names the generated theme files
	Light bool   // true for light palettes (alucard); tools that need to know

	Fg        string
	Bg        string
	Comment   string
	Selection string

	Cyan   string
	Green  string
	Orange string
	Pink   string
	Purple string
	Red    string
	Yellow string
}

// Open is the standard, freely-licensed Dracula palette. This is bothy's
// default and the only palette whose values live in this repository.
func Open() Palette {
	return Palette{
		Name:      "dracula",
		Fg:        "#F8F8F2",
		Bg:        "#282A36",
		Comment:   "#6272A4",
		Selection: "#44475A",
		Cyan:      "#8BE9FD",
		Green:     "#50FA7B",
		Orange:    "#FFB86C",
		Pink:      "#FF79C6",
		Purple:    "#BD93F9",
		Red:       "#FF5555",
		Yellow:    "#F1FA8C",
	}
}

// Color looks a token up by name. It is the funcmap entry point templates call
// as {{ theme.Color "purple" }}; an unknown token is a hard error, because an
// empty colour shows up only as an unreadable pane.
func (p Palette) Color(token string) (string, error) {
	switch token {
	case "fg", "foreground":
		return p.Fg, nil
	case "bg", "background":
		return p.Bg, nil
	case "comment":
		return p.Comment, nil
	case "selection":
		return p.Selection, nil
	case "cyan":
		return p.Cyan, nil
	case "green":
		return p.Green, nil
	case "orange":
		return p.Orange, nil
	case "pink", "magenta":
		return p.Pink, nil
	case "purple", "blue":
		return p.Purple, nil
	case "red":
		return p.Red, nil
	case "yellow":
		return p.Yellow, nil
	}
	return "", fmt.Errorf("theme: unknown colour token %q", token)
}

// missing reports the tokens that are still empty, so an install fails loudly
// rather than writing a half-themed config.
func (p Palette) missing() []string {
	var out []string
	for _, tok := range paletteTokens {
		if v, _ := p.Color(tok); v == "" {
			out = append(out, tok)
		}
	}
	return out
}

// Resolve returns the palette to theme the workspace with. palettePath wins
// when set: pointing bothy at a file is how any palette other than open
// Dracula gets in.
func Resolve(provider, palettePath string) (Palette, error) {
	if palettePath != "" {
		return Load(palettePath)
	}
	switch provider {
	case "", "dracula", "open":
		return Open(), nil
	}
	return Palette{}, fmt.Errorf("theme: unknown provider %q\n"+
		"      built in: dracula\n"+
		"      for anything else, write a palette file and set theme.palette", provider)
}
