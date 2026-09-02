package doctor

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
)

// A key can be unrecognised for three reasons, and bothy could name only two:
// a typo, or a newer bothy that wrote it. The third is a key an older bothy
// wrote and a newer one retired — which happened the moment watermark became
// background_image, and produced "written by a newer bothy?" about a key
// written by an older one.
func TestARetiredKeyIsNamedAsRetired(t *testing.T) {
	cfg := config.Default()
	cfg.Unknown = []string{"workspace.watermark"}
	env := Env{Platform: platform.Info{Home: t.TempDir()}, Config: cfg}

	res := checkConfigKeys(env)
	if !strings.Contains(res.Detail, "renamed to") {
		t.Errorf("detail = %q, want the replacement named", res.Detail)
	}
	if strings.Contains(res.Detail, "newer bothy") {
		t.Errorf("a key this bothy retired was blamed on a newer one: %q", res.Detail)
	}
	if !strings.Contains(res.Fix, "workspace.background_image") {
		t.Errorf("fix = %q, want the key that replaced it", res.Fix)
	}
}

// The other two readings still work, and a typo is still a typo.
func TestATypoIsStillATypo(t *testing.T) {
	cfg := config.Default()
	cfg.Unknown = []string{"workspace.pane_frame"}
	env := Env{Platform: platform.Info{Home: t.TempDir()}, Config: cfg}

	res := checkConfigKeys(env)
	if !strings.Contains(res.Detail, "did you mean") {
		t.Errorf("detail = %q, want a suggestion", res.Detail)
	}
}

// Every retired key names a replacement that exists, or names none at all.
// A rename pointing at a key bothy does not have would send someone to a
// command that fails.
func TestRetiredKeysPointSomewhereReal(t *testing.T) {
	keys := config.Keys()
	for old, replacement := range config.Retired {
		if replacement == "" {
			continue
		}
		found := false
		for _, k := range keys {
			if k == replacement {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is said to be renamed to %q, which is not a key bothy accepts",
				old, replacement)
		}
	}
}
