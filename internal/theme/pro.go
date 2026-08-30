package theme

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Dracula PRO support, by reference only.
//
// PRO is a paid pack. bothy stores none of its colours (ADR-006); instead the
// user points theme.pro_pack at the copy they bought, and we read the palette
// out of it at install time. The pack also ships ready-made Ghostty themes and
// vim colorschemes, which are copied verbatim rather than regenerated — there
// is no point deriving what the pack already contains.

// ProVariants are the pack's variants, in the order the pack presents them.
// "pro" is the purple default; "alucard" is the light one.
var ProVariants = []string{"pro", "blade", "buffy", "lincoln", "morbius", "van-helsing", "alucard"}

// IsProVariant reports whether name selects a Dracula PRO variant.
func IsProVariant(name string) bool {
	for _, v := range ProVariants {
		if v == name {
			return true
		}
	}
	return false
}

// PackLayout describes where things live inside a Dracula PRO pack. Kept as
// data so a pack reorganisation is a one-line change rather than a hunt.
type PackLayout struct {
	Palette    string // design/palette.md
	GhosttyDir string // themes/ghostty  (files named per variant)
	VimColors  string // themes/vim/colors
	FontsDir   string // fonts
}

// DefaultPackLayout matches the pack as shipped.
var DefaultPackLayout = PackLayout{
	Palette:    filepath.Join("design", "palette.md"),
	GhosttyDir: filepath.Join("themes", "ghostty"),
	VimColors:  filepath.Join("themes", "vim", "colors"),
	FontsDir:   "fonts",
}

// ValidatePack checks that a directory really is a Dracula PRO pack before we
// try to read colours out of it. The error names the exact path that is
// missing: "pointed bothy at the wrong folder" is the overwhelmingly likely
// mistake, and a vague error would send people hunting through their config.
func ValidatePack(dir string) error {
	if dir == "" {
		return fmt.Errorf("theme: variant is a Dracula PRO variant but theme.pro_pack is not set\n" +
			"      set it to your own copy of the pack, e.g.\n" +
			"      bothy config set theme.pro_pack ~/Documents/Dracula_Theme")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("theme: pro_pack %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("theme: pro_pack %s is not a directory", dir)
	}
	pal := filepath.Join(dir, DefaultPackLayout.Palette)
	if _, err := os.Stat(pal); err != nil {
		return fmt.Errorf("theme: %s not found — is %s really a Dracula PRO pack?\n"+
			"      it should contain design/palette.md and themes/", pal, dir)
	}
	return nil
}

// LoadPro reads the palette for one variant out of a Dracula PRO pack.
func LoadPro(packDir, variant string) (Palette, error) {
	if err := ValidatePack(packDir); err != nil {
		return Palette{}, err
	}
	src, err := os.ReadFile(filepath.Join(packDir, DefaultPackLayout.Palette))
	if err != nil {
		return Palette{}, fmt.Errorf("theme: reading pack palette: %w", err)
	}
	p, err := ParsePalette(src, variant)
	if err != nil {
		return Palette{}, err
	}
	if miss := p.missing(); len(miss) > 0 {
		return Palette{}, fmt.Errorf("theme: variant %q from %s is missing colours: %s",
			variant, packDir, strings.Join(miss, ", "))
	}
	return p, nil
}

// GhosttyThemeFile is the pack's ready-made Ghostty theme for a variant.
// The pack names these with hyphens ("van-helsing"), matching our variant slugs.
func GhosttyThemeFile(packDir, variant string) string {
	return filepath.Join(packDir, DefaultPackLayout.GhosttyDir, variant)
}

// VimColorscheme is the colorscheme name for a variant. Note the pack is not
// self-consistent here: the Ghostty theme file is "van-helsing" but the vim
// colorscheme is "dracula_pro_van_helsing" — hyphens there, underscores here.
func VimColorscheme(variant string) string {
	if variant == "pro" {
		return "dracula_pro"
	}
	return "dracula_pro_" + strings.ReplaceAll(variant, "-", "_")
}

// ParsePalette extracts one variant's palette from the pack's palette.md.
//
// The file has two top-level sections: "# Color Palette" (the design palette,
// which is what we want) and "# Color Palette - Terminal Standard" (ANSI
// mappings). Variant headings repeat across both with different content, so
// parsing stops at the second top-level heading.
//
// Within the design section, "## Base Dracula PRO" carries the foreground and
// the eight accents shared by every dark variant, and each variant's own
// section carries whatever it overrides — usually just background, comment and
// selection, but Alucard (the light variant) restates every colour. Applying
// base first and the variant on top handles both without a special case.
func ParsePalette(src []byte, variant string) (Palette, error) {
	sections, err := paletteSections(src)
	if err != nil {
		return Palette{}, err
	}

	p := Palette{Name: "dracula-pro", Variant: variant, Light: variant == "alucard"}

	if base, ok := sections["base"]; ok {
		applyRows(&p, base)
	}
	rows, ok := sections[variant]
	if !ok {
		known := make([]string, 0, len(sections))
		for k := range sections {
			if k != "base" {
				known = append(known, k)
			}
		}
		sort.Strings(known)
		return Palette{}, fmt.Errorf("theme: variant %q not found in palette.md (found: %s)",
			variant, strings.Join(known, ", "))
	}
	applyRows(&p, rows)

	return p, nil
}

// paletteSections splits the design half of palette.md into
// slug -> {colour name: hex}.
func paletteSections(src []byte) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	var current map[string]string

	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	topLevelSeen := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		if strings.HasPrefix(line, "# ") {
			topLevelSeen++
			// The second top-level heading starts the terminal-standard
			// tables, which repeat every variant heading with ANSI content.
			// Everything we need is above it.
			if topLevelSeen > 1 {
				break
			}
			continue
		}

		if strings.HasPrefix(line, "## ") {
			slug := variantSlug(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			current = map[string]string{}
			out[slug] = current
			continue
		}

		if current == nil || !strings.HasPrefix(line, "|") {
			continue
		}
		name, hex, ok := parseRow(line)
		if ok {
			current[name] = hex
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("theme: scanning palette.md: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("theme: no colour tables found in palette.md")
	}
	return out, nil
}

// variantSlug turns a section heading into a variant name.
//
//	"Base Dracula PRO"           -> "base"
//	"Dracula PRO"                -> "pro"
//	"Dracula PRO - Van Helsing"  -> "van-helsing"
//	"Alucard"                    -> "alucard"
func variantSlug(heading string) string {
	h := strings.ToLower(strings.TrimSpace(heading))
	if h == "base dracula pro" {
		return "base"
	}
	h = strings.TrimPrefix(h, "dracula pro")
	h = strings.TrimSpace(strings.Trim(strings.TrimSpace(h), "-"))
	h = strings.TrimSpace(h)
	if h == "" {
		return "pro"
	}
	return strings.Join(strings.Fields(h), "-")
}

// parseRow pulls the colour name and hex out of a markdown table row.
// Separator rows (all dashes) and the header row are rejected.
func parseRow(line string) (name, hex string, ok bool) {
	cells := strings.Split(strings.Trim(line, "|"), "|")
	if len(cells) < 2 {
		return "", "", false
	}
	name = strings.ToLower(strings.TrimSpace(cells[0]))
	hex = strings.Trim(strings.TrimSpace(cells[1]), "`")

	if name == "" || name == "palette" || strings.Trim(name, "- ") == "" {
		return "", "", false
	}
	if !strings.HasPrefix(hex, "#") || len(hex) != 7 {
		return "", "", false
	}
	return name, strings.ToUpper(hex), true
}

// applyRows overlays one section's colours onto p, leaving tokens the section
// does not mention untouched.
func applyRows(p *Palette, rows map[string]string) {
	set := func(dst *string, key string) {
		if v, ok := rows[key]; ok {
			*dst = v
		}
	}
	set(&p.Fg, "foreground")
	set(&p.Bg, "background")
	set(&p.Comment, "comment")
	set(&p.Selection, "selection")
	set(&p.Cyan, "cyan")
	set(&p.Green, "green")
	set(&p.Orange, "orange")
	set(&p.Pink, "pink")
	set(&p.Purple, "purple")
	set(&p.Red, "red")
	set(&p.Yellow, "yellow")
}
