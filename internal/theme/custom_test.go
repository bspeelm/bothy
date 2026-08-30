package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writePalette drops a palette file into a temporary directory. The colours are
// obviously synthetic — this repository has no business containing anyone's
// real palette, test fixture or not.
func writePalette(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "palette.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const completePalette = `
name  = "test-palette"
light = false

fg        = "#010101"
bg        = "#020202"
comment   = "#030303"
selection = "#040404"
cyan      = "#050505"
green     = "#060606"
orange    = "#070707"
pink      = "#080808"
purple    = "#090909"
red       = "#0A0A0A"
yellow    = "#0B0B0B"
`

func TestLoadCompletePalette(t *testing.T) {
	p, err := Load(writePalette(t, completePalette))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "test-palette" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Purple != "#090909" {
		t.Errorf("Purple = %q", p.Purple)
	}
	if miss := p.missing(); len(miss) > 0 {
		t.Errorf("palette reported incomplete: %v", miss)
	}
}

// A missing token would render as an empty string in a config file. Some tools
// accept that and then draw something unreadable, so it has to fail here.
func TestLoadRejectsIncompletePalette(t *testing.T) {
	body := strings.Replace(completePalette, `purple    = "#090909"`, "", 1)
	_, err := Load(writePalette(t, body))
	if err == nil {
		t.Fatal("expected an error for a palette missing a token")
	}
	if !strings.Contains(err.Error(), "purple") {
		t.Errorf("error should name the missing token, got: %v", err)
	}
}

func TestLoadRejectsBadHex(t *testing.T) {
	for _, bad := range []string{"blue", "#FFF", "090909", "#GGGGGG"} {
		body := strings.Replace(completePalette, `"#090909"`, `"`+bad+`"`, 1)
		if _, err := Load(writePalette(t, body)); err == nil {
			t.Errorf("expected an error for purple = %q", bad)
		}
	}
}

func TestLoadNormalisesCase(t *testing.T) {
	body := strings.Replace(completePalette, `"#0A0A0A"`, `"#0a0a0a"`, 1)
	p, err := Load(writePalette(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if p.Red != "#0A0A0A" {
		t.Errorf("Red = %q, want upper-cased", p.Red)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml")); err == nil {
		t.Fatal("expected an error for a missing palette file")
	}
}

func TestLightPaletteFlagSurvives(t *testing.T) {
	body := strings.Replace(completePalette, "light = false", "light = true", 1)
	p, err := Load(writePalette(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Light {
		t.Error("light = true was not carried through")
	}
}

// Resolve is the single entry point: a palette file wins, otherwise a built-in.
func TestResolvePrefersPaletteFile(t *testing.T) {
	path := writePalette(t, completePalette)
	p, err := Resolve("dracula", path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "test-palette" {
		t.Errorf("Name = %q — the palette file should win over the provider", p.Name)
	}
}

func TestResolveDefaultsToOpenDracula(t *testing.T) {
	p, err := Resolve("", "")
	if err != nil {
		t.Fatal(err)
	}
	if p != Open() {
		t.Error("an unset provider should give the built-in open palette")
	}
}

// An unknown provider must fail rather than silently theming the workspace as
// something the user did not ask for — and the error should point at the way in.
func TestResolveUnknownProviderExplainsTheWayIn(t *testing.T) {
	_, err := Resolve("some-paid-theme", "")
	if err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
	if !strings.Contains(err.Error(), "theme.palette") {
		t.Errorf("error should point at theme.palette, got: %v", err)
	}
}

// The blank example must actually load once someone fills it in — otherwise
// the one on-ramp bothy offers for a palette of your own is broken.
func TestExampleFileBecomesAValidPaletteWhenFilled(t *testing.T) {
	// Fill every empty colour field, whatever the alignment happens to be.
	var n int
	filled := regexp.MustCompile(`(?m)^(\s*)(fg|bg|comment|selection|cyan|green|orange|pink|purple|red|yellow)(\s*=\s*)""`).
		ReplaceAllStringFunc(ExampleFile, func(m string) string {
			n++
			return strings.Replace(m, `""`, fmt.Sprintf("\"#%02X%02X%02X\"", n, n, n), 1)
		})

	if n != len(paletteTokens) {
		t.Fatalf("filled %d fields, expected %d — the example is missing a token", n, len(paletteTokens))
	}
	p, err := Load(writePalette(t, filled))
	if err != nil {
		t.Fatalf("the filled-in example does not load: %v", err)
	}
	if p.Name != "my-palette" {
		t.Errorf("Name = %q", p.Name)
	}
}
