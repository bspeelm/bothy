package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// proColours are the Dracula PRO palette values. They are listed here — and
// only here, in the test that forbids them — so that ADR-006 is enforced by the
// build rather than by remembering.
//
// bothy supports Dracula PRO by reading the user's own licensed pack at install
// time. Shipping any of these values, in code, templates or documentation,
// would be redistributing part of a paid product.
var proColours = []string{
	"22212C", "454158", "7970A9", "504C67", // pro background, selection, comment, bright black
	"FF9580", "8AFF80", "FFFF80", "9580FF", // red, green, yellow, purple
	"FF80BF", "80FFEA", "FFCA80", // pink, cyan, orange
	"212C2A", "2A212C", "2C2A21", "2C2122", "0B0D0F", // variant backgrounds
}

// allowed lists files permitted to mention a PRO value, with the reason.
var allowed = map[string]string{
	// The project's own planning document quotes one value in a sentence
	// instructing the implementation not to use it.
	"PLAN.md": "instructional mention telling the port to use open Dracula instead",
	// This test names them in order to forbid them.
	filepath.Join("internal", "theme", "licence_test.go"): "the list being enforced",
}

func TestNoDraculaProColoursInRepository(t *testing.T) {
	root := "../.."

	// Only text bothy ships. A build artefact or a vendored dependency is not
	// something this project is publishing.
	shipped := map[string]bool{
		".go": true, ".tmpl": true, ".toml": true, ".md": true,
		".kdl": true, ".lua": true, ".vim": true, ".sh": true, ".yml": true,
	}

	var offences []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !shipped[filepath.Ext(path)] {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if _, ok := allowed[rel]; ok {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		upper := strings.ToUpper(string(body))
		for _, colour := range proColours {
			if strings.Contains(upper, "#"+colour) {
				offences = append(offences, rel+" contains #"+colour)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, o := range offences {
		t.Errorf("Dracula PRO colour in a shipped file: %s", o)
	}
	if len(offences) > 0 {
		t.Log("bothy ships only the open Dracula palette; PRO is read from the " +
			"user's own pack at install time (docs/decisions.md ADR-006)")
	}
}

// The fixture must not be a copy of the real pack either — it exists to test
// the parser against the pack's *shape*, not its contents.
func TestFixturePackIsSynthetic(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(fixturePack, "design", "palette.md"))
	if err != nil {
		t.Fatal(err)
	}
	upper := strings.ToUpper(string(body))
	for _, colour := range proColours {
		if strings.Contains(upper, "#"+colour) {
			t.Errorf("the test fixture contains the real PRO colour #%s", colour)
		}
	}
}
