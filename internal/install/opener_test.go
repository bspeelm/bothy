package install

import (
	"strings"
	"testing"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/render"
)

// The generated yazi config named xdg-open on every machine. macOS has `open`
// instead, so pressing Enter on an image or a PDF in the file pane called a
// binary that is not there -- and text files opened in the editor, which is
// why nobody noticed.
func TestOpenerFitsTheMachine(t *testing.T) {
	for _, tc := range []struct {
		name   string
		p      platform.Info
		run    string
		binary string
	}{
		{"linux", platform.Info{OS: "linux"}, "xdg-open", "xdg-open"},
		{"darwin", platform.Info{OS: "darwin"}, "open", "open"},
		{
			// The app databases live on the host, so a local xdg-open would be
			// a working binary with no viewers behind it.
			"in a container",
			platform.Info{OS: "linux", Container: platform.Toolbx},
			"flatpak-spawn --host xdg-open", "flatpak-spawn",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := opener(tc.p); got != tc.run {
				t.Errorf("opener = %q, want %q", got, tc.run)
			}
			if got := OpenerBinary(tc.p); got != tc.binary {
				t.Errorf("OpenerBinary = %q, want %q", got, tc.binary)
			}
		})
	}
}

// A container on macOS is a virtual machine rather than a shared home, so the
// container answer wins over the darwin one and nothing tries to spawn a host
// process that is not there.
func TestContainerOpenerWinsOverThePlatform(t *testing.T) {
	p := platform.Info{OS: "darwin", Container: platform.Generic}
	if got := opener(p); !strings.HasPrefix(got, "flatpak-spawn") {
		t.Errorf("opener in a container = %q, want the host hop", got)
	}
}

// And the rendered config carries it, which is the thing that was broken --
// the check and the template have to name the same program.
func TestRenderedYaziConfigUsesTheMachinesOpener(t *testing.T) {
	p := platform.Info{OS: "darwin", Home: t.TempDir()}
	data := Data{Opener: opener(p), OpenerDesc: openerDesc(p)}

	tmpl, err := bothy.Templates.ReadFile("templates/browser/yazi/yazi.toml.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	w := render.NewWriter(t.TempDir(), "")
	rendered, err := w.Render("yazi.toml", "yazi", "yazi.toml.tmpl", string(tmpl), data)
	if err != nil {
		t.Fatal(err)
	}
	out := string(rendered)
	// Scoped to the block that runs, not the whole file: the comment above it
	// names xdg-open on purpose, to say what was chosen instead and why.
	block := openerBlock(t, out)
	if !strings.Contains(block, `run = 'open "$@"'`) {
		t.Errorf("the darwin config does not open files with `open`:\n%s", block)
	}
	if strings.Contains(block, "xdg-open") {
		t.Errorf("the darwin config still runs xdg-open:\n%s", block)
	}
}

func openerBlock(t *testing.T, s string) string {
	t.Helper()
	i := strings.Index(s, "[opener]")
	if i < 0 {
		t.Fatal("the rendered yazi config has no [opener] section at all")
	}
	return s[i:min(i+200, len(s))]
}
