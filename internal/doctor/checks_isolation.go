package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/fetch"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/state"
)

// Checks that bothy's own tree is intact and that nothing has been asked to
// live outside it. ADR-009 as a runtime assertion rather than a test.

// checkConfigAge catches the upgrade nobody finishes.
//
// Templates and profiles are compiled into the binary, and a launch does not
// re-render -- ensureInstalled returns as soon as the config root exists. So a
// newer bothy runs against whatever an older one wrote, indefinitely, and the
// only thing that fixes it is typing `bothy install`, which nothing asks you
// to do. This is the check that asks.
//
// Warn rather than fail: stale configs work, they are just not what this
// binary would generate. Failing would exit non-zero on every machine that had
// not yet re-installed, which is most of them the day this ships.
func checkConfigAge(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	// A source build is ahead of the tag it describes, and a developer
	// rebuilding all day does not need to be told their configs are one commit
	// old. Note this is not `== "dev"`: make install-binary stamps git
	// describe, so the real shape is v0.1.5-3-gabc1234-dirty.
	if fetch.IsSourceBuild(env.Version) {
		return skip("running a source build; version comparison would be noise")
	}
	m, err := state.Load(env.Platform.StateDir())
	if err != nil {
		return skip("no manifest to read a version from")
	}
	switch {
	case m.BothyVer == "":
		return warn("the configs were written by a bothy that predates this check",
			"bothy did not record its version until 0.1.5, so this cannot say which one",
			"bothy install")
	case m.BothyVer != env.Version:
		return warn("the configs were written by bothy "+m.BothyVer,
			"you are running "+env.Version+", whose templates may differ; a launch does not re-render",
			"bothy install")
	}
	return pass("the configs were written by this bothy")
}

// checkConfigKeys catches the typo that used to cost nothing to make and
// everything to find. `toml.Unmarshal` over the defaults accepted any key, so
// `slots.borwser = "yazi"` loaded cleanly, did nothing, and kept doing nothing
// -- on every machine, because the README says to carry ~/.config/bothy in git.
func checkConfigKeys(env Env) Result {
	unknown := env.Config.Unknown
	if len(unknown) == 0 {
		return pass("every key in config.toml is recognised")
	}
	var lines []string
	for _, k := range unknown {
		if near := config.Nearest(k); near != "" {
			lines = append(lines, fmt.Sprintf("%q — did you mean %q?", k, near))
			continue
		}
		// Nothing close. The other reason a key is unrecognised is that it
		// was written by a bothy newer than this one, and saying so is
		// cheaper than the confused issue it would otherwise produce.
		lines = append(lines, fmt.Sprintf("%q — written by a newer bothy?", k))
	}
	fix := "remove the line, or set it properly: bothy config set <key> <value>"
	if near := config.Nearest(unknown[0]); near != "" {
		fix = "bothy config set " + near + " <value>, and delete the old line"
	}
	return warn(fmt.Sprintf("%d unrecognised key(s) in config.toml", len(unknown)),
		strings.Join(lines, "\n    "), fix)
}

// checkIsolation is the promise in ADR-009, checked rather than asserted:
// bothy's tree exists, and the files revision 1 used to write are not there
// with bothy's marker on them.
func checkIsolation(env Env) Result {
	root := env.Platform.ConfigRoot()
	if _, err := os.Stat(root); err != nil {
		return fail("bothy's config tree is missing", root+" does not exist",
			"run 'bothy install'")
	}
	// Files a previous revision wrote outside the tree. Finding one means an
	// upgrade left something behind that nothing will now maintain.
	stale := []string{
		filepath.Join(env.Platform.Home, ".bashrc.d", "bothy.sh"),
		filepath.Join(env.Platform.LocalBin, "xdg-open"),
	}
	var found []string
	for _, f := range stale {
		if b, err := os.ReadFile(f); err == nil && strings.Contains(string(b), "bothy") {
			found = append(found, f)
		}
	}
	if len(found) > 0 {
		return warn("files from an older bothy are still outside its tree",
			strings.Join(found, ", "),
			"delete them; bothy no longer writes outside "+root)
	}
	return pass("everything bothy manages is inside " + root)
}

// checkPassthrough states plainly which slots use your configs rather than
// bothy's, and what that turns off. Passthrough is a reasonable thing to want
// and a confusing thing to forget.
func checkPassthrough(env Env) Result {
	if len(env.Config.Passthrough) == 0 {
		return skip("no slots are passed through")
	}
	var lost []string
	for _, slot := range env.Config.Passthrough {
		switch slot {
		case "yazi":
			lost = append(lost, "yazi: bothy's image-preview handling and container-aware opener do not apply")
		case "zellij":
			lost = append(lost, "zellij: bothy's theme does not apply; your own keybindings do")
		}
	}
	return warn("using your own config for: "+strings.Join(env.Config.Passthrough, ", "),
		strings.Join(lost, "; "),
		"remove it from passthrough in ~/.config/bothy/config.toml to use bothy's")
}

// checkProfileRenders catches a broken profile before launch rather than at it.
// A custom profile in ~/.config/bothy/profiles is hand-written and therefore
// the most likely thing here to be wrong.
func checkProfileRenders(env Env) Result {
	name := env.ProfileName
	if name == "" {
		name = "cockpit"
	}
	prof, err := install.LoadProfile(env.Platform, name)
	if err != nil {
		return fail("the layout profile does not load", err.Error(),
			"fix it, or switch back: bothy config set profile cockpit")
	}
	if _, err := layout.Render(prof, install.Commands(env.Config)); err != nil {
		return fail("the layout profile does not render", err.Error(),
			"a pane probably names a slot nothing fills")
	}
	return pass(fmt.Sprintf("profile %q renders (%d panes)", name, prof.PaneCount()))
}
