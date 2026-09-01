package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/fetch"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/state"
	"github.com/bspeelm/bothy/internal/tools"
)

// Checks that bothy's own tree is intact and that nothing lives outside it.

// checkConfigAge catches the upgrade nobody finishes. Templates are compiled
// into the binary and a launch does not re-render, so a newer bothy runs
// against whatever an older one wrote until someone types `bothy install`.
// Warn, not fail: stale configs work.
func checkConfigAge(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	// A source build is ahead of the tag it describes. Not `== "dev"`:
	// make install-binary stamps git describe, so the real shape is
	// v0.1.5-3-gabc1234-dirty.
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

// checkConfigKeys reports unrecognised keys. Unmarshal accepts any key, so a
// typo such as `slots.borwser` loads cleanly and silently does nothing.
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
		// Nothing close: the other reason a key is unrecognised is that a
		// newer bothy wrote it.
		lines = append(lines, fmt.Sprintf("%q — written by a newer bothy?", k))
	}
	fix := "remove the line, or set it properly: bothy config set <key> <value>"
	if near := config.Nearest(unknown[0]); near != "" {
		fix = "bothy config set " + near + " <value>, and delete the old line"
	}
	return warn(fmt.Sprintf("%d unrecognised key(s) in config.toml", len(unknown)),
		strings.Join(lines, "\n    "), fix)
}

// checkIsolation checks ADR-009 at runtime: bothy's tree exists, and no file
// outside it carries bothy's marker.
func checkIsolation(env Env) Result {
	root := env.Platform.ConfigRoot()
	if _, err := os.Stat(root); err != nil {
		return fail("bothy's config tree is missing", root+" does not exist",
			"run 'bothy install'")
	}
	// Paths an older bothy wrote outside the tree; nothing maintains them now.
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

// checkPassthrough states which slots use your configs rather than bothy's, and what that turns off.
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

// checkProfileRenders catches a broken custom profile before launch.
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

// checkToolData reports what the tools bothy runs keep outside bothy's tree.
//
// Only XDG_CACHE_HOME is pointed inside it. A cache is a tool's own scratch
// space: losing it costs a rebuild, so keeping it in the tree makes uninstall
// complete without taking anything from anyone. Data and state are not that.
// Neovim's plugins, zoxide's learned directories and lazygit's state are the
// user's, and redirecting them hid all of it from the tools running in the
// workspace.
//
// So they stay where the tool would have put them, and this says which. It is
// the form of ADR-009 that is true: bothy writes nothing outside its own tree,
// and names what the programs it starts write outside theirs.
func checkToolData(env Env) Result {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		state = filepath.Join(env.Platform.Home, ".local", "state")
	}

	// Derived from the tools bothy may supply, plus the editor it launches, so
	// that a tool added later is covered without a second list to remember.
	names := map[string]bool{install.EditorBinary(env.Config.Slots.Editor): true}
	if all, err := tools.Load(); err == nil {
		for _, t := range all {
			names[t.Name] = true
		}
	}
	delete(names, "")

	var found []string
	for name := range names {
		for _, dir := range []string{env.Platform.DataDir, state} {
			path := filepath.Join(dir, name)
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				found = append(found, path)
			}
		}
	}
	if len(found) == 0 {
		return pass("the tools bothy runs keep nothing outside its tree")
	}
	sort.Strings(found)
	return note(fmt.Sprintf("%d tool director(ies) live outside bothy's tree, on purpose", len(found)),
		strings.Join(found, ", ")+" — your data, kept where the tool keeps it. "+
			"'bothy uninstall' does not remove these")
}
