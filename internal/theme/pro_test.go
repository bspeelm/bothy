package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixturePack = "../../test/fixtures/dracula-pro-pack"

func readFixture(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(fixturePack, "design", "palette.md"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return src
}

func TestParsePaletteInheritsBaseAccents(t *testing.T) {
	p, err := ParsePalette(readFixture(t), "pro")
	if err != nil {
		t.Fatalf("ParsePalette: %v", err)
	}
	// Accents come from the shared "Base Dracula PRO" table…
	if p.Purple != "#060606" {
		t.Errorf("Purple = %q, want #060606 (from base table)", p.Purple)
	}
	if p.Fg != "#010101" {
		t.Errorf("Fg = %q, want #010101 (from base table)", p.Fg)
	}
	// …while background/comment/selection come from the variant's own table.
	if p.Bg != "#111111" {
		t.Errorf("Bg = %q, want #111111 (from variant table)", p.Bg)
	}
	if p.Comment != "#222222" {
		t.Errorf("Comment = %q, want #222222", p.Comment)
	}
	if p.Selection != "#333333" {
		t.Errorf("Selection = %q, want #333333", p.Selection)
	}
	if miss := p.missing(); len(miss) > 0 {
		t.Errorf("palette incomplete, missing %v", miss)
	}
}

func TestParsePaletteVariantOverridesOnlyWhatItStates(t *testing.T) {
	p, err := ParsePalette(readFixture(t), "blade")
	if err != nil {
		t.Fatalf("ParsePalette: %v", err)
	}
	if p.Bg != "#444444" {
		t.Errorf("Bg = %q, want #444444", p.Bg)
	}
	if p.Purple != "#060606" {
		t.Errorf("Purple = %q, want the base accent #060606 — blade states no accents", p.Purple)
	}
}

// Alucard is the light variant and restates every colour, including accents.
// It is also the one section whose heading is not "Dracula PRO - X".
func TestParsePaletteLightVariantOverridesAccents(t *testing.T) {
	p, err := ParsePalette(readFixture(t), "alucard")
	if err != nil {
		t.Fatalf("ParsePalette: %v", err)
	}
	if !p.Light {
		t.Error("Light = false, want true for alucard")
	}
	if p.Purple != "#ABABAB" {
		t.Errorf("Purple = %q, want #ABABAB — alucard must override the base accent", p.Purple)
	}
	if p.Bg != "#AEAEAE" {
		t.Errorf("Bg = %q, want #AEAEAE", p.Bg)
	}
	if miss := p.missing(); len(miss) > 0 {
		t.Errorf("palette incomplete, missing %v", miss)
	}
}

// The pack repeats every variant heading under a second top-level heading with
// ANSI values. Parsing must stop at that boundary, or the design colours get
// silently overwritten by terminal ones — a bug that would look like "the
// theme is nearly right" rather than an error.
func TestParsePaletteIgnoresTerminalStandardSection(t *testing.T) {
	p, err := ParsePalette(readFixture(t), "blade")
	if err != nil {
		t.Fatalf("ParsePalette: %v", err)
	}
	if strings.HasPrefix(p.Bg, "#FF00") {
		t.Errorf("Bg = %q — parser read past the second top-level heading", p.Bg)
	}
}

func TestParsePaletteUnknownVariantListsWhatExists(t *testing.T) {
	_, err := ParsePalette(readFixture(t), "nosferatu")
	if err == nil {
		t.Fatal("expected an error for an unknown variant")
	}
	// The message has to be actionable: naming the variants actually present
	// is the difference between a two-second fix and a hunt through the pack.
	for _, want := range []string{"nosferatu", "alucard", "blade", "pro"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestVariantSlug(t *testing.T) {
	cases := map[string]string{
		"Base Dracula PRO":          "base",
		"Dracula PRO":               "pro",
		"Dracula PRO - Blade":       "blade",
		"Dracula PRO - Van Helsing": "van-helsing",
		"Alucard":                   "alucard",
	}
	for heading, want := range cases {
		if got := variantSlug(heading); got != want {
			t.Errorf("variantSlug(%q) = %q, want %q", heading, got, want)
		}
	}
}

// The pack is not self-consistent: its Ghostty theme file is "van-helsing"
// but its vim colorscheme is "dracula_pro_van_helsing".
func TestVimColorscheme(t *testing.T) {
	cases := map[string]string{
		"pro":         "dracula_pro",
		"blade":       "dracula_pro_blade",
		"van-helsing": "dracula_pro_van_helsing",
	}
	for variant, want := range cases {
		if got := VimColorscheme(variant); got != want {
			t.Errorf("VimColorscheme(%q) = %q, want %q", variant, got, want)
		}
	}
}

func TestValidatePackErrorsAreActionable(t *testing.T) {
	if err := ValidatePack(""); err == nil {
		t.Error("expected an error when pro_pack is unset")
	} else if !strings.Contains(err.Error(), "theme.pro_pack") {
		t.Errorf("error %q should name the setting to fix", err)
	}

	// A directory that exists but is not a pack — the likely user mistake.
	dir := t.TempDir()
	err := ValidatePack(dir)
	if err == nil {
		t.Fatal("expected an error for a directory that is not a pack")
	}
	if !strings.Contains(err.Error(), "palette.md") {
		t.Errorf("error %q should name the file it looked for", err)
	}
}

func TestLoadProFixturePack(t *testing.T) {
	p, err := LoadPro(fixturePack, "pro")
	if err != nil {
		t.Fatalf("LoadPro: %v", err)
	}
	if p.Name != "dracula-pro" || p.Variant != "pro" {
		t.Errorf("got Name=%q Variant=%q", p.Name, p.Variant)
	}
}

func TestResolveOpenNeedsNoPack(t *testing.T) {
	p, err := Resolve("open", "")
	if err != nil {
		t.Fatalf("Resolve(open): %v", err)
	}
	if p.Bg != "#282A36" {
		t.Errorf("Bg = %q, want the open Dracula background #282A36", p.Bg)
	}
}

// Selecting a PRO variant without a pack must fail rather than quietly falling
// back to open Dracula — see ADR-006.
func TestResolveProWithoutPackFails(t *testing.T) {
	if _, err := Resolve("pro", ""); err == nil {
		t.Fatal("expected an error: a PRO variant with no pack must not fall back")
	}
}

func TestResolveUnknownVariant(t *testing.T) {
	if _, err := Resolve("solarized", ""); err == nil {
		t.Fatal("expected an error for an unknown variant")
	}
}

func TestColorRejectsUnknownToken(t *testing.T) {
	if _, err := Open().Color("chartreuse"); err == nil {
		t.Error("expected an error: templates must not render an empty colour")
	}
}
