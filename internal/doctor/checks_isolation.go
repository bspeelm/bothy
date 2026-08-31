package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
)

// Checks that bothy's own tree is intact and that nothing has been asked to
// live outside it. ADR-009 as a runtime assertion rather than a test.

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
