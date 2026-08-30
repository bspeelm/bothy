package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The rule this file enforces: **the only colour values in this repository are
// open Dracula's.**
//
// bothy ships one palette, the MIT-licensed open Dracula one in theme.go. Every
// other palette — including any a user has licensed — arrives as a file on
// their machine that bothy reads at install time and never carries.
//
// Note how this is checked. An earlier version of this test listed the values
// it wanted to forbid, which meant the test forbidding a palette was itself a
// copy of it. Inverting the check removes that: nothing here names a value that
// is not already open Dracula's, and the guard is stronger for it, because it
// catches *any* stray palette rather than one particular vendor's.

var hexInText = regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)

// allowedColours is the open Dracula palette, plus the few neutral values the
// documentation legitimately mentions.
func allowedColours() map[string]string {
	p := Open()
	allowed := map[string]string{
		p.Fg: "open Dracula fg", p.Bg: "open Dracula bg",
		p.Comment: "open Dracula comment", p.Selection: "open Dracula selection",
		p.Cyan: "open Dracula cyan", p.Green: "open Dracula green",
		p.Orange: "open Dracula orange", p.Pink: "open Dracula pink",
		p.Purple: "open Dracula purple", p.Red: "open Dracula red",
		p.Yellow: "open Dracula yellow",

		// GitHub's own page backgrounds, used in the logo derivation notes.
		"#0D1117": "GitHub dark background",
		"#FFFFFF": "white",
		"#000000": "black",
	}
	return allowed
}

// shippedExtensions are the text files this project publishes.
var shippedExtensions = map[string]bool{
	".go": true, ".tmpl": true, ".toml": true, ".md": true,
	".kdl": true, ".lua": true, ".vim": true, ".sh": true, ".yml": true,
}

func TestOnlyOpenDraculaColoursAreShipped(t *testing.T) {
	allowed := allowedColours()

	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "dist", "node_modules", "testdata", "fixtures":
				return filepath.SkipDir
			}
			return nil
		}
		if !shippedExtensions[filepath.Ext(path)] || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel("../..", path)

		seen := map[string]bool{}
		for _, m := range hexInText.FindAllString(string(body), -1) {
			up := strings.ToUpper(m)
			if allowed[up] != "" || seen[up] {
				continue
			}
			seen[up] = true
			t.Errorf("%s contains %s, which is not part of the open Dracula palette.\n"+
				"        bothy ships one palette and reads every other from a file on the "+
				"user's machine;\n        a colour appearing here means one was pasted in. "+
				"See docs/decisions.md ADR-006.", rel, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The example palette handed to users must be blank. Pre-filling it with any
// real palette would put those values back into the repository by the back door.
func TestExamplePaletteCarriesNoColours(t *testing.T) {
	if found := hexInText.FindAllString(ExampleFile, -1); len(found) > 0 {
		t.Errorf("the example palette contains colour values: %v", found)
	}
	// It must still be a usable starting point.
	for _, tok := range paletteTokens {
		if !strings.Contains(ExampleFile, tok+" ") && !strings.Contains(ExampleFile, tok+"=") {
			t.Errorf("the example palette has no %q line to fill in", tok)
		}
	}
}

// Open() is the one palette with values, so it had better be complete.
func TestOpenPaletteIsComplete(t *testing.T) {
	if miss := Open().missing(); len(miss) > 0 {
		t.Errorf("the built-in palette is missing %v", miss)
	}
}
