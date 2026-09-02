package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/install"
)

// checkThemeReached verifies the palette arrived in the files the tools read.
//
// theme-palette resolves a *custom* palette file and skips for everyone using
// the built-in one, which is nearly everyone -- so the theme capability was
// claimed by a check that almost never ran (#110). ADR-007 has the rule this
// broke: test the effect, not the artefact.
//
// The foreground colour is the probe because every one of these files carries
// it. The background deliberately does not travel: zellij's `bg` is the UI
// chrome rather than the terminal background, so it takes a different value on
// purpose, and looking for it would fail on a correct install.
func checkThemeReached(env Env) Result {
	pal, err := env.Config.Palette(env.Platform)
	if err != nil {
		return skip("no palette resolved: " + err.Error())
	}

	type target struct {
		slot string
		path string
	}
	targets := []target{
		{"browser", filepath.Join(install.YaziDir(env.Platform), "theme.toml")},
		{"terminal", install.GhosttyConf(env.Platform)},
	}
	// The zellij theme is named after the palette, so it is found rather than
	// constructed.
	if themes, _ := filepath.Glob(filepath.Join(install.ZellijDir(env.Platform), "themes", "*.kdl")); len(themes) > 0 {
		targets = append(targets, target{"mux", themes[0]})
	}

	var missing []string
	checked := 0
	for _, t := range targets {
		if env.Config.PassesThrough(t.slot) {
			continue
		}
		body, err := os.ReadFile(t.path)
		if err != nil {
			continue
		}
		checked++
		if !strings.Contains(strings.ToUpper(string(body)), strings.ToUpper(pal.Fg)) {
			missing = append(missing, t.path)
		}
	}

	switch {
	case checked == 0:
		return skip("nothing themed is installed yet")
	case len(missing) > 0:
		return fail("the palette did not reach every tool",
			strings.Join(missing, ", ")+" carries no "+pal.Fg,
			"run 'bothy install' to write them again")
	}
	return pass(fmt.Sprintf("the palette reached %d generated config(s)", checked))
}

// checkSessionIsNamed verifies that this workspace can be found again.
//
// A session bothy names after its project is one `bothy attach` can pick out
// of several. An anonymous one -- started by hand, or by a bothy from before
// sessions had names -- cannot be chosen between, and nothing said so: attach
// simply took whichever the multiplexer offered (#109).
//
// This is what stands behind the sessions capability. It is a property of the
// session rather than of the machine, so outside one there is nothing to
// report.
func checkSessionIsNamed(env Env) Result {
	running := os.Getenv("ZELLIJ_SESSION_NAME")
	if running == "" {
		return skip("not inside a workspace session")
	}
	if env.SessionName == "" {
		return skip("no session name to expect")
	}
	if running != env.SessionName {
		return warn("this session is not the one bothy would attach to",
			fmt.Sprintf("running in %q; bothy names this project's session %q", running, env.SessionName),
			"exit and run bothy again — the new session takes the name, and the old one keeps running until you kill it")
	}
	return pass("session " + running + " is named, so attach can find it")
}
