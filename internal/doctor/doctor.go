// Package doctor detects the ways this workspace is known to break.
//
// Every check here exists because the failure it looks for actually happened
// and cost someone an afternoon — most of them are drawn from
// docs/origin-cheatsheet.md. The common thread is that these failures are
// *silent*: Yazi discards an entire config and carries on, vim ignores a
// colorscheme without a word, Ghostty ignores a config file whose name is one
// character wrong. A check that only restates an error the tool already prints
// loudly is not worth adding.
//
// Per PLAN.md §0: when a setup bug is fixed, the fix ships with a check that
// detects it.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/install"
	"github.com/bothy-dev/bothy/internal/layout"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/probe"
	"github.com/bothy-dev/bothy/internal/theme"
	"github.com/bothy-dev/bothy/internal/tools"
)

// Severity distinguishes "this is broken" from "this is not what you asked for".
type Severity string

const (
	Fail Severity = "fail" // the workspace is broken; exit non-zero
	Warn Severity = "warn" // works, but not as intended
	Pass Severity = "pass"
	Skip Severity = "skip" // not applicable here
)

// Result is one check's verdict.
type Result struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
	// Detail explains what was actually observed.
	Detail string `json:"detail,omitempty"`
	// Fix is a single actionable line. Every failing check must have one:
	// a diagnosis without a fix is just a nicer error message.
	Fix string `json:"fix,omitempty"`
}

// Report is a full run.
type Report struct {
	Results []Result `json:"results"`
}

// Failed reports whether any check failed, which is the process exit code.
func (r Report) Failed() bool {
	for _, res := range r.Results {
		if res.Severity == Fail {
			return true
		}
	}
	return false
}

// Counts summarises a report.
func (r Report) Counts() (pass, warn, fail, skip int) {
	for _, res := range r.Results {
		switch res.Severity {
		case Pass:
			pass++
		case Warn:
			warn++
		case Fail:
			fail++
		case Skip:
			skip++
		}
	}
	return
}

// Check is one diagnostic.
type Check struct {
	ID  string
	Run func(Env) Result
}

// Env is everything the checks are allowed to look at.
type Env struct {
	Platform platform.Info
	Config   config.Config
	// Profile is the layout profile in use, for the pane-count check.
	ProfileName string
	PaneCount   int
	// MuxBin is the multiplexer binary bothy will actually launch, resolved
	// through its own bin first. Checking the system's copy instead reports
	// confidently about a binary that is not the one being used.
	MuxBin string
	// ToolEnv is the environment bothy's session runs tools with. Checks that
	// invoke a tool must use it, or they interrogate the user's config instead
	// of bothy's — a check that confidently reports on the wrong file is worse
	// than no check at all. The caller supplies it via install.SessionEnv.
	ToolEnv []string
}

// tool builds a command that runs the way bothy's session would.
func (e Env) tool(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	if e.ToolEnv != nil {
		cmd.Env = e.ToolEnv
	}
	return cmd
}

// Run executes every applicable check.
func Run(env Env) Report {
	var rep Report
	for _, c := range Checks() {
		res := c.Run(env)
		res.ID = c.ID
		rep.Results = append(rep.Results, res)
	}
	return rep
}

// Checks is the full list, in the order they are reported.
func Checks() []Check {
	return []Check{
		{ID: "yazi-config-discarded", Run: checkYaziConfigDiscarded},
		{ID: "yazi-version", Run: checkYaziVersion},
		{ID: "yazi-config-keys", Run: checkYaziConfigKeys},
		{ID: "yazi-plugins", Run: checkYaziPlugins},
		{ID: "image-previews", Run: checkImagePreviews},
		{ID: "profile-renders", Run: checkProfileRenders},
		{ID: "layout-built", Run: checkLayoutBuilt},
		{ID: "terminal-capability", Run: checkTerminalCapability},
		{ID: "passthrough", Run: checkPassthrough},
		{ID: "isolation", Run: checkIsolation},
		{ID: "watermark-image", Run: checkWatermarkImage},
		{ID: "zellij-config", Run: checkZellijConfig},
		{ID: "terminfo", Run: checkTerminfo},
		{ID: "opener", Run: checkOpener},
		{ID: "xdg-open-shim-guard", Run: checkXdgOpenShimGuard},
		{ID: "agent", Run: checkAgent},
		{ID: "tool-provenance", Run: checkToolProvenance},
		{ID: "theme-palette", Run: checkThemePalette},
	}
}

func pass(summary string) Result { return Result{Severity: Pass, Summary: summary} }
func skip(summary string) Result { return Result{Severity: Skip, Summary: summary} }
func fail(summary, detail, fix string) Result {
	return Result{Severity: Fail, Summary: summary, Detail: detail, Fix: fix}
}
func warn(summary, detail, fix string) Result {
	return Result{Severity: Warn, Summary: summary, Detail: detail, Fix: fix}
}

// yaziComplaint matches the noise Yazi makes when it rejects a config.
//
// This is the single highest-value check bothy has. On a bad config Yazi prints
// "Press <Enter> to continue with preset settings" and then runs with *none* of
// your configuration — not the broken key, the whole file. Everything looks
// almost right, which is far worse than a crash.
var yaziComplaint = regexp.MustCompile(`(?i)must be|invalid|preset|cannot|failed`)

// staleTable matches a real [manager] table header, not a mention of one.
var staleTable = regexp.MustCompile(`(?m)^\s*\[manager\]`)

// staleFiletypeRule matches the pre-26.x `name = "*"` form of a filetype rule.
// It is anchored the same way and for the same reason.
var staleFiletypeRule = regexp.MustCompile(`(?m)^\s*name\s*=\s*"[*/]`)

func checkYaziConfigDiscarded(env Env) Result {
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	if env.Config.PassesThrough("yazi") {
		return skip("yazi is passed through to your own config")
	}
	bin, err := exec.LookPath("yazi")
	if err != nil {
		return fail("yazi is not installed",
			"the browser pane would open an empty pane",
			"run 'bothy install' to install it")
	}
	// --clear-cache does the config parse and exits, without needing a terminal.
	out, _ := env.tool(bin, "--clear-cache").CombinedOutput()
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
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	out, err := env.tool("yazi", "--version").Output()
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
	if env.Config.Slots.Browser != "yazi" || env.Config.PassesThrough("yazi") {
		return skip("bothy is not managing yazi's config")
	}
	var stale []string
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(env.Platform.ConfigRoot(), "yazi", name))
		return string(b)
	}
	// Anchored to the start of a line: a config that *documents* the rename in
	// a comment is correct, and matching that comment would fail every config
	// bothy itself writes.
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

// checkImagePreviews reports which side of the ADR-007 gate this machine is on.
// It is deliberately never a failure: both answers are correct, and the value
// is in saying which one is in force and why.
func checkImagePreviews(env Env) Result {
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	mux := env.MuxBin
	if env.Config.Slots.Mux == "none" {
		mux = ""
	}
	g := probe.CheckGraphics(mux, env.Platform.Terminal)

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

func checkWatermarkImage(env Env) Result {
	if !env.Config.Workspace.Watermark {
		return skip("watermark is off")
	}
	if env.Config.Slots.Terminal != "ghostty" {
		return skip("watermark needs ghostty")
	}
	path := filepath.Join(env.Platform.ConfigRoot(), "watermark.png")
	fi, err := os.Stat(path)
	if err != nil {
		return fail("the watermark image is missing",
			path+" does not exist; ghostty will silently draw nothing",
			"run 'bothy install' to write it")
	}
	if fi.Size() == 0 {
		return fail("the watermark image is empty", path+" is zero bytes",
			"run 'bothy install' to rewrite it")
	}
	return pass("watermark image is in place")
}

// checkTerminalCapability reports where bothy will run. A terminal that cannot
// draw images is not an error — it is a reason previews will be off, and
// saying so beats letting someone wonder why their config "did not work".
// checkYaziPlugins reports plugins bothy's config wants and does not have.
//
// This is a warning rather than a failure because the generated config is
// written to match what is installed, so a missing plugin costs a feature
// rather than breaking the workspace. That was not always true: an earlier
// version referenced all four unconditionally and installed none, producing a
// config that failed only at launch — and `yazi --clear-cache`, which the
// config check uses, does not execute init.lua, so nothing caught it.
func checkYaziPlugins(env Env) Result {
	if env.Config.Slots.Browser != "yazi" || env.Config.PassesThrough("yazi") {
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
		return warn(fmt.Sprintf("%d yazi plugin(s) are not installed", len(missing)),
			strings.Join(missing, "; "),
			"run 'bothy install' with a network connection")
	}
	return pass(fmt.Sprintf("all %d yazi plugins installed", len(plugins)))
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

func checkTerminalCapability(env Env) Result {
	term := env.Platform.Terminal
	if term == "" {
		term = "an unrecognised terminal"
	}
	g := probe.CheckGraphics("", env.Platform.Terminal)
	if g.Supported {
		return pass("running in " + term + ", which can draw images")
	}
	// Not a failure: bothy opens a Ghostty window when the current terminal
	// cannot draw, so this is a statement of what will happen rather than a
	// problem to fix. It becomes one only if there is no Ghostty to open.
	if _, err := exec.LookPath("ghostty"); err != nil {
		return warn("this terminal cannot draw inline images, and ghostty is not installed",
			g.Reason+"; previews will fall back to block art",
			"install ghostty, or accept block-art previews")
	}
	return pass("this terminal cannot draw images, so bothy will open a Ghostty window — " + g.Reason)
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

func checkZellijConfig(env Env) Result {
	if env.Config.Slots.Mux != "zellij" {
		return skip("mux slot is not zellij")
	}
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return fail("zellij is not installed", "", "run 'bothy install'")
	}
	out, err := env.tool(bin, "setup", "--check").CombinedOutput()
	if err != nil {
		return fail("zellij rejects its configuration",
			strings.TrimSpace(string(out)),
			"run 'bothy install' to regenerate bothy's zellij config")
	}
	return pass("zellij accepts its config")
}

// checkTerminfo catches the container trap: the toolbox image has no
// xterm-ghostty entry, so entering it greets you with a terminfo error and
// leaves the terminal degraded.
func checkTerminfo(env Env) Result {
	term := env.Platform.Term
	if term == "" {
		return warn("$TERM is not set", "", "")
	}
	if err := exec.Command("infocmp", term).Run(); err != nil {
		fix := "install the terminfo entry for " + term
		if env.Platform.InContainer() {
			fix = "copy it in from the host: mkdir -p ~/.terminfo/" + term[:1] +
				" && cp /run/host/usr/share/terminfo/" + term[:1] + "/" + term + " ~/.terminfo/" + term[:1] + "/"
		}
		return fail("no terminfo entry for $TERM ("+term+")",
			"the terminal will fall back to a degraded mode", fix)
	}
	return pass("terminfo entry for " + term + " is present")
}

func checkOpener(env Env) Result {
	if _, err := exec.LookPath("xdg-open"); err != nil {
		if env.Platform.InContainer() {
			return fail("xdg-open is not on PATH",
				"pressing Enter on a file in yazi will report 'No such file or directory'",
				"run 'bothy install' to place the host-forwarding shim in ~/.local/bin")
		}
		return warn("xdg-open is not on PATH", "", "install xdg-utils")
	}
	return pass("xdg-open resolves")
}

// checkXdgOpenShimGuard is the check that stops a fix from becoming a worse
// bug. Home is shared between host and container, so the shim in ~/.local/bin
// is on the host's PATH too — without its containerenv guard, the host execs
// itself forever.
func checkXdgOpenShimGuard(env Env) Result {
	shim := filepath.Join(env.Platform.BinDir(), "xdg-open")
	b, err := os.ReadFile(shim)
	if err != nil {
		return skip("no xdg-open shim installed")
	}
	body := string(b)
	if !strings.Contains(body, "flatpak-spawn") {
		return skip("~/.local/bin/xdg-open is not bothy's shim")
	}
	if !strings.Contains(body, "/run/.containerenv") && !strings.Contains(body, "/run/.dockerenv") {
		return fail("the xdg-open shim has no container guard",
			"home is shared with the host, so on the host this shim would exec itself forever",
			"run 'bothy install' to rewrite it")
	}
	return pass("xdg-open shim is guarded against host recursion")
}

// checkToolProvenance reports where each tool came from and whether it is
// still good enough. This replaces revision 1's PATH-shadowing check, which
// mattered only because bothy installed into ~/.local/bin; a tool bothy
// supplies now lives in its own bin and is on PATH for its session alone.
//
// The failure it exists to catch: a system tool below the minimum with nothing
// supplied to cover it, which is a workspace that will misbehave in a way the
// individual tool never complains about.
func checkToolProvenance(env Env) Result {
	names, err := tools.Required(env.Config.Slots.Mux, env.Config.Slots.Browser, env.Config.Extras)
	if err != nil {
		return warn("could not read the tool definitions", err.Error(), "")
	}

	var missing, supplied, system []string
	for _, name := range names {
		t, err := tools.Get(name)
		if err != nil {
			continue
		}
		own := filepath.Join(env.Platform.BinDir(), t.Binary)
		if fi, err := os.Stat(own); err == nil && fi.Mode()&0o111 != 0 {
			supplied = append(supplied, name)
			continue
		}
		d := tools.Resolve(t, tools.SystemLookPath, tools.SystemVersion)
		if d.Action == tools.Fetch {
			missing = append(missing, name+" ("+d.Reason+")")
			continue
		}
		system = append(system, name)
	}

	if len(missing) > 0 {
		return fail("some tools are missing or too old",
			strings.Join(missing, "; "),
			"run 'bothy install' to supply them into bothy's own bin")
	}
	return pass(fmt.Sprintf("%d tool(s) from your system, %d supplied by bothy",
		len(system), len(supplied)))
}

func checkAgent(env Env) Result {
	slot := env.Config.Slots.Agent
	if slot == "" || slot == "none" {
		return skip("no agent slot configured")
	}
	bin := map[string]string{"claude-code": "claude", "gemini-cli": "gemini"}[slot]
	if bin == "" {
		bin = slot
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fail(bin+" is not on PATH",
			"the agent pane would open empty",
			"install it, or point the slot elsewhere: bothy config set slots.agent <name>")
	}
	return pass(bin + " is on PATH")
}

// shadowable are the tools whose duplicate copies cause the most confusion when
// an unrelated package repository puts an older one earlier on PATH.
var shadowable = []string{"rg", "fd", "fzf", "jq", "delta"}

func checkThemePalette(env Env) Result {
	path := env.Config.PalettePath(env.Platform)
	if path == "" {
		return skip("using the built-in open Dracula palette")
	}
	if _, err := os.Stat(path); err != nil {
		return fail("the configured palette file is missing",
			path+" does not exist",
			"point bothy at it again: bothy config set theme.palette <path>")
	}
	pal, err := theme.Load(path)
	if err != nil {
		return fail("the palette file could not be read", err.Error(),
			"fix the file, or fall back: bothy config set theme.palette \"\"")
	}
	return pass("palette " + pal.Name + " reads cleanly")
}

// whichAll returns every copy of a binary on PATH, in order.
func whichAll(name string) []string {
	var out []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			out = append(out, p)
		}
	}
	return out
}
