package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
)

func themeSandbox(t *testing.T, fg string) Env {
	t.Helper()
	home := t.TempDir()
	p := platform.Info{Home: home, DataDir: filepath.Join(home, ".local", "share")}
	if err := os.MkdirAll(install.YaziDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install.YaziDir(p), "theme.toml"),
		[]byte("fg = \""+fg+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Env{Platform: p, Config: config.Default()}
}

// #110. theme-palette resolves a custom palette file and skips for everyone
// using the built-in one, so the theme capability was claimed by a check that
// almost never ran. This one asks whether the colours arrived.
func TestThemeReachedNoticesAConfigWithoutThePalette(t *testing.T) {
	if res := checkThemeReached(themeSandbox(t, "#F8F8F2")); res.Severity != Pass {
		t.Errorf("a config carrying the palette = %s: %s", res.Severity, res.Summary)
	}
	res := checkThemeReached(themeSandbox(t, "#000000"))
	if res.Severity != Fail {
		t.Errorf("a config without the palette = %s, want fail", res.Severity)
	}
	if res.Fix == "" {
		t.Error("a failing check must say what to do about it")
	}
}

// A slot you passed through is yours, and bothy does not write its theme --
// so demanding the palette there would fail an install that is working as
// asked.
func TestThemeReachedLeavesPassedThroughSlotsAlone(t *testing.T) {
	env := themeSandbox(t, "#000000")
	env.Config.Passthrough = []string{"browser"}
	if res := checkThemeReached(env); res.Severity == Fail {
		t.Errorf("a passed-through slot was held to bothy's palette: %s", res.Detail)
	}
}

// #109. An anonymous session cannot be picked out by `bothy attach`, and
// nothing said so: attach took whichever the multiplexer offered.
func TestSessionNamedNoticesAnAnonymousSession(t *testing.T) {
	env := Env{Platform: platform.Info{Home: t.TempDir()}, SessionName: "bothy-work"}

	t.Setenv("ZELLIJ_SESSION_NAME", "bothy-work")
	if res := checkSessionIsNamed(env); res.Severity != Pass {
		t.Errorf("the session bothy would attach to = %s: %s", res.Severity, res.Summary)
	}

	t.Setenv("ZELLIJ_SESSION_NAME", "polite-galaxy")
	res := checkSessionIsNamed(env)
	if res.Severity != Warn {
		t.Errorf("an anonymous session = %s, want warn", res.Severity)
	}
	if res.Fix == "" {
		t.Error("a warning must say what to do about it")
	}
}

// Outside a session there is nothing to report, which is not the same as
// something being wrong.
func TestSessionNamedIsQuietOutsideASession(t *testing.T) {
	t.Setenv("ZELLIJ_SESSION_NAME", "")
	env := Env{Platform: platform.Info{Home: t.TempDir()}, SessionName: "bothy-work"}
	if res := checkSessionIsNamed(env); res.Severity != Skip {
		t.Errorf("outside a session = %s, want skip", res.Severity)
	}
}
