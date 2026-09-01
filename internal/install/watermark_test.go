package install

import (
	"strings"
	"testing"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/render"
	"github.com/bspeelm/bothy/internal/theme"
)

func ghosttyConfig(t *testing.T, data Data) string {
	t.Helper()
	tmpl, err := bothy.Templates.ReadFile("templates/terminal/ghostty/config.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	w := render.NewWriter(t.TempDir(), "")
	out, err := w.Render("ghostty.conf", "ghostty", "config.tmpl", string(tmpl), data)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// The watermark was a boolean and a Tux that bothy shipped, uncredited, and
// composited for one screen size. It is a path to art of your own now, so the
// generated config says nothing at all unless you have named one.
func TestNoWatermarkMeansNoBackgroundImage(t *testing.T) {
	out := ghosttyConfig(t, Data{})
	if strings.Contains(out, "background-image") {
		t.Errorf("ghostty is told about a background image nobody asked for:\n%s", out)
	}
}

func TestWatermarkPointsAtTheFileYouNamed(t *testing.T) {
	out := ghosttyConfig(t, Data{Watermark: "/home/me/art.png", WatermarkOpacity: "0.05"})
	if !strings.Contains(out, "background-image = /home/me/art.png") {
		t.Errorf("the image path is not in the config:\n%s", out)
	}
	// stretch, not none: a corner anchor positions in absolute pixels and is
	// correct on exactly one monitor.
	if !strings.Contains(out, "background-image-fit = stretch") {
		t.Errorf("the fit is not stretch:\n%s", out)
	}
}

// A path is somewhere a person types, so it can start with a tilde.
func TestWatermarkExpandsATilde(t *testing.T) {
	cfg := config.Default()
	cfg.Workspace.Watermark = "~/pictures/art.png"
	p := platform.Info{Home: "/home/me", DataDir: "/home/me/.local/share"}

	data := buildData(p, cfg, theme.Palette{})
	if data.Watermark != "/home/me/pictures/art.png" {
		t.Errorf("Watermark = %q, want the tilde expanded", data.Watermark)
	}
}
