// Package theme resolves a colour palette for the whole workspace.
//
// Every tool bothy configures is themed from one 11-token Palette, so a theme
// provider is a palette plus per-tool templates — there is no theme engine.
//
// Two sources fill a Palette:
//
//   - Built-in open palettes (Dracula, MIT-licensed), baked into this package.
//   - A user's own licensed Dracula PRO pack, parsed at install time. See pro.go
//     and docs/decisions.md ADR-006: bothy ships no PRO colour of its own.
package theme

import (
	"fmt"
	"strings"
)

// Palette is the complete set of colours every template may reference.
// Keep it at these eleven tokens: a template that needs a twelfth colour is a
// sign the template is doing design work that belongs in the palette.
type Palette struct {
	Name    string // display name, e.g. "dracula" or "dracula-pro"
	Variant string // "open", "pro", "blade", …
	Light   bool   // true for light palettes (alucard); tools that need to know

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
		Variant:   "open",
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

// Color looks a token up by name. This is the funcmap entry point used by
// templates as {{ theme.Color "purple" }}, so an unknown token must be a hard
// error — a template silently rendering an empty colour is the kind of failure
// that only shows up as an unreadable pane.
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

// complete reports the tokens that are still empty. Used to fail an install
// loudly rather than write a half-themed config.
func (p Palette) missing() []string {
	var out []string
	for _, tok := range []string{
		"fg", "bg", "comment", "selection",
		"cyan", "green", "orange", "pink", "purple", "red", "yellow",
	} {
		if v, _ := p.Color(tok); v == "" {
			out = append(out, tok)
		}
	}
	return out
}

// Resolve returns the palette for a configured variant.
//
// "open" (the default) is built in. Every other variant is a Dracula PRO one
// and requires the user's own pack; there is deliberately no fallback that
// quietly substitutes open Dracula, because silently themeing the workspace
// differently from what was asked for is worse than refusing.
func Resolve(variant, packDir string) (Palette, error) {
	if variant == "" || variant == "open" || variant == "dracula" {
		return Open(), nil
	}
	if !IsProVariant(variant) {
		return Palette{}, fmt.Errorf("theme: unknown variant %q (open, %s)",
			variant, strings.Join(ProVariants, ", "))
	}
	return LoadPro(packDir, variant)
}
