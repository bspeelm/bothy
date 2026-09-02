package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/probe"
)

// yaziComplaint matches the noise Yazi makes when it rejects a config. On a
// bad config Yazi prints "Press <Enter> to continue with preset settings" and
// runs with none of your configuration — the whole file, not the bad key.
var yaziComplaint = regexp.MustCompile(`(?i)must be|invalid|preset|cannot|failed`)

// staleTable matches a real [manager] table header, not a mention of one.
var staleTable = regexp.MustCompile(`(?m)^\s*\[manager\]`)

// staleFiletypeRule matches the pre-26.x `name = "*"` form of a filetype rule.
// It is anchored the same way and for the same reason.
var staleFiletypeRule = regexp.MustCompile(`(?m)^\s*name\s*=\s*"[*/]`)

func checkYaziConfigDiscarded(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	if env.Config.PassesThrough("browser") {
		return skip("yazi is passed through to your own config")
	}
	if _, err := env.lookPath("yazi"); err != nil {
		return fail("yazi is not installed",
			"the browser pane would open an empty pane",
			"run 'bothy install' to install it")
	}
	// `ya cache clear` parses the config and exits without needing a terminal,
	// so this is a config check rather than a cache one. `ya env`
	// names the question better and cannot be used: it demands a tty, and the
	// doctor has none in CI. ya ships inside yazi's own archive, so its absence
	// is a partial install rather than a missing feature.
	bin, err := env.lookPath("ya")
	if err != nil {
		return warn("yazi is installed but ya is not, so its config is unchecked",
			"ya ships alongside yazi and reads the same config",
			"run 'bothy install' to supply it")
	}
	out, _ := env.tool(bin, "cache", "clear").CombinedOutput()
	if yaziComplaint.Match(out) {
		return fail("yazi is ignoring its entire config",
			strings.TrimSpace(string(out)),
			"fix the reported key; on a bad config Yazi silently falls back to preset settings")
	}
	return pass("yazi accepts its config")
}

// MinYaziForPlugins is the first Yazi that current yazi-rs plugins load on.
// Older Yazi refuses every one of them with "requires at least Yazi 26.x".
var MinYaziForPlugins = probe.Version{Major: 26}

func checkYaziVersion(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	yaziBin, err := env.lookPath("yazi")
	if err != nil {
		return skip("yazi is not installed")
	}
	out, err := env.tool(yaziBin, "--version").Output()
	if err != nil {
		return skip("yazi is not installed")
	}
	ok, v, err := probe.AtLeast(string(out), MinYaziForPlugins)
	if err != nil {
		return warn("could not read the yazi version", string(out), "")
	}
	plugins := filepath.Join(env.Platform.ConfigRoot(), "yazi", "plugins")
	if entries, err := os.ReadDir(plugins); err == nil && len(entries) > 0 && !ok {
		return fail(fmt.Sprintf("yazi %s is too old for the installed plugins", v),
			"every current yazi-rs plugin refuses to load below 26.x",
			"install a newer yazi with 'bothy install'; distro packages are often years behind")
	}
	if !ok {
		return warn(fmt.Sprintf("yazi %s predates the plugin ecosystem", v), "", "run 'bothy install' for a current build")
	}
	return pass(fmt.Sprintf("yazi %s", v))
}

// checkYaziConfigKeys catches the two 26.x renames that a config written for
// 25.x still parses around — [manager] became [mgr], and filetype rules and
// fetchers take `url` where they used to take `name` and `id`.
func checkYaziConfigKeys(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if env.Config.Slots.Browser != "yazi" || env.Config.PassesThrough("browser") {
		return skip("bothy is not managing yazi's config")
	}
	var stale []string
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(env.Platform.ConfigRoot(), "yazi", name))
		return string(b)
	}
	// Anchored to the start of a line: bothy's own configs document the rename
	// in a comment, and matching that comment would fail them.
	if staleTable.MatchString(read("yazi.toml")) {
		stale = append(stale, "[manager] should be [mgr] (renamed in 25.4)")
	}
	for _, f := range []string{"yazi.toml", "theme.toml"} {
		if staleFiletypeRule.MatchString(read(f)) {
			stale = append(stale, f+": filetype rules take `url`, not `name`, since 26.x")
		}
	}
	if len(stale) > 0 {
		return fail("yazi config uses pre-26.x key names",
			strings.Join(stale, "; "),
			"run 'bothy install' to regenerate, or fix the keys by hand")
	}
	return pass("yazi config uses current key names")
}

// checkYaziPlugins reports plugins bothy's config wants and does not have.
// A warning, not a failure: the generated config matches what is installed.
// It needs its own check because `ya cache clear`, which the config check uses,
// does not execute init.lua, so a missing plugin surfaces only at launch.
func checkYaziPlugins(env Env) Result {
	if env.Config.Slots.Browser != "yazi" || env.Config.PassesThrough("browser") {
		return skip("bothy is not managing yazi's config")
	}
	plugins, err := install.YaziPlugins()
	if err != nil {
		return warn("could not read the plugin list", err.Error(), "")
	}
	var missing []string
	for _, pl := range plugins {
		if !install.PluginInstalled(env.Platform, pl.Name) {
			missing = append(missing, pl.Name+" ("+pl.Gives+")")
		}
	}
	if len(missing) > 0 {
		fix := "run 'bothy install' with a network connection"
		if _, err := exec.LookPath("git"); err != nil {
			fix = "install git — 'ya pkg' clones these from GitHub — then run 'bothy install'"
		}
		return warn(fmt.Sprintf("%d yazi plugin(s) are not installed", len(missing)),
			strings.Join(missing, "; "), fix)
	}
	return pass(fmt.Sprintf("all %d yazi plugins installed", len(plugins)))
}

// checkImagePreviews reports which side of the ADR-007 gate this machine is on. Never a failure.
func checkImagePreviews(env Env) Result {
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	mg := probe.MuxGraphics{None: true}
	if env.Mux != nil && env.MuxBin != "" {
		carries, reason := env.Mux.Graphics(env.MuxBin)
		mg = probe.MuxGraphics{Carries: carries, Reason: reason}
	}
	g := probe.CheckGraphics(env.Platform.Terminal, mg)

	yaziToml := filepath.Join(env.Platform.ConfigRoot(), "yazi", "yazi.toml")
	b, err := os.ReadFile(yaziToml)
	if err != nil {
		return skip("yazi is not configured yet")
	}
	workaroundPresent := strings.Contains(string(b), "enter-hint")

	switch {
	case g.Supported && workaroundPresent:
		return warn("image previews are disabled but this machine can show them",
			g.Reason,
			"run 'bothy install' to regenerate yazi.toml and turn previews on")
	case !g.Supported && !workaroundPresent:
		return warn("image previews are enabled but this machine cannot show them properly",
			g.Reason+"; expect block art and a phantom \"Find next\" keypress",
			"run 'bothy install' to regenerate yazi.toml with the placeholder previewer")
	case g.Supported:
		return pass("image previews are on — " + g.Reason)
	default:
		return pass("image previews are off — " + g.Reason)
	}
}
