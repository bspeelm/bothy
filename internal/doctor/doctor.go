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
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/probe"
	"github.com/bothy-dev/bothy/internal/theme"
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
		{ID: "image-previews", Run: checkImagePreviews},
		{ID: "ghostty-config-name", Run: checkGhosttyConfigName},
		{ID: "watermark-image", Run: checkWatermarkImage},
		{ID: "zellij-config", Run: checkZellijConfig},
		{ID: "terminfo", Run: checkTerminfo},
		{ID: "editor-env", Run: checkEditorEnv},
		{ID: "vim-colorscheme", Run: checkVimColorscheme},
		{ID: "vim-colorscheme-location", Run: checkVimColorschemeLocation},
		{ID: "opener", Run: checkOpener},
		{ID: "xdg-open-shim-guard", Run: checkXdgOpenShimGuard},
		{ID: "agent", Run: checkAgent},
		{ID: "path-shadowing", Run: checkPathShadowing},
		{ID: "local-bin-on-path", Run: checkLocalBinOnPath},
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
	bin, err := exec.LookPath("yazi")
	if err != nil {
		return fail("yazi is not installed",
			"the browser pane would open an empty pane",
			"run 'bothy install' to install it")
	}
	// --clear-cache does the config parse and exits, without needing a terminal.
	out, _ := exec.Command(bin, "--clear-cache").CombinedOutput()
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
	out, err := exec.Command("yazi", "--version").Output()
	if err != nil {
		return skip("yazi is not installed")
	}
	ok, v, err := probe.AtLeast(string(out), MinYaziForPlugins)
	if err != nil {
		return warn("could not read the yazi version", string(out), "")
	}
	plugins := filepath.Join(env.Platform.ConfigDir, "yazi", "plugins")
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
	if env.Config.Slots.Browser != "yazi" {
		return skip("browser slot is not yazi")
	}
	var stale []string
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(env.Platform.ConfigDir, "yazi", name))
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
	mux := env.Config.Slots.Mux
	if mux == "none" {
		mux = ""
	}
	g := probe.CheckGraphics(mux, env.Platform.Terminal)

	yaziToml := filepath.Join(env.Platform.ConfigDir, "yazi", "yazi.toml")
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

// checkGhosttyConfigName catches a file that looks right and does nothing.
// Ghostty reads exactly ~/.config/ghostty/config; a config.ghostty or
// config.toml beside it is ignored without a word.
func checkGhosttyConfigName(env Env) Result {
	if env.Config.Slots.Terminal != "ghostty" {
		return skip("terminal slot is not ghostty")
	}
	dir := filepath.Join(env.Platform.ConfigDir, "ghostty")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return skip("no ghostty config directory")
	}
	var nearMiss []string
	hasConfig := false
	for _, e := range entries {
		switch {
		case e.Name() == "config":
			hasConfig = true
		case strings.HasPrefix(e.Name(), "config."):
			nearMiss = append(nearMiss, e.Name())
		}
	}
	if len(nearMiss) > 0 {
		return warn("a ghostty config file is being silently ignored",
			strings.Join(nearMiss, ", ")+" — ghostty reads only the file named exactly 'config'",
			"delete it, or rename it to 'config' (no extension)")
	}
	if !hasConfig {
		return fail("ghostty has no config file",
			dir+"/config does not exist",
			"run 'bothy install'")
	}
	return pass("ghostty config is at the filename ghostty reads")
}

// checkWatermarkImage catches a watermark that is switched on but pointing at
// nothing. Ghostty does not complain about a missing background-image — it just
// draws no image, which looks exactly like "the opacity is too low" and sends
// people tuning a setting that was never the problem.
func checkWatermarkImage(env Env) Result {
	if !env.Config.Workspace.Watermark {
		return skip("watermark is off")
	}
	if env.Config.Slots.Terminal != "ghostty" {
		return skip("watermark needs ghostty")
	}
	path := filepath.Join(env.Platform.ConfigDir, "ghostty", "watermark.png")
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

func checkZellijConfig(env Env) Result {
	if env.Config.Slots.Mux != "zellij" {
		return skip("mux slot is not zellij")
	}
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return fail("zellij is not installed", "", "run 'bothy install'")
	}
	out, err := exec.Command(bin, "setup", "--check").CombinedOutput()
	if err != nil {
		return fail("zellij rejects its configuration",
			strings.TrimSpace(string(out)),
			"run 'bothy install' to regenerate ~/.config/zellij/config.kdl")
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

// checkEditorEnv catches Fedora's nano-default-editor, which exports
// EDITOR=nano from /etc/profile.d and quietly wins over an unset EDITOR.
func checkEditorEnv(env Env) Result {
	want := env.Config.Slots.Editor
	if want == "" || want == "none" {
		return skip("no editor slot configured")
	}
	got := os.Getenv("EDITOR")
	if got == "" {
		return fail("EDITOR is not set",
			"yazi, lazygit and git all shell out to it",
			"start a new shell so ~/.bashrc.d/bothy.sh is sourced, or run 'bothy install'")
	}
	if !strings.Contains(filepath.Base(got), strings.TrimSuffix(want, "-code")) {
		return warn("EDITOR is "+got+", not the configured editor ("+want+")",
			"Fedora's nano-default-editor package sets this from /etc/profile.d",
			"start a new shell so ~/.bashrc.d/bothy.sh is sourced")
	}
	return pass("EDITOR is " + got)
}

// checkVimColorscheme tests that the colorscheme actually loaded, rather than
// that its file exists. The distinction matters: a colorscheme in
// ~/.vim/pack/*/start is on 'runtimepath' only *after* .vimrc is sourced, so
// the `colorscheme` line fails silently and vim comes up in default colours.
//
// The -u is not optional. `vim -es` alone does not source ~/.vimrc, so without
// it this test passes while testing nothing.
func checkVimColorscheme(env Env) Result {
	if env.Config.Slots.Editor != "vim" {
		return skip("editor slot is not vim")
	}
	if _, err := exec.LookPath("vim"); err != nil {
		return skip("vim is not installed")
	}
	vimrc := filepath.Join(env.Platform.Home, ".vimrc")
	if _, err := os.Stat(vimrc); err != nil {
		return skip("no ~/.vimrc")
	}

	out, err := os.CreateTemp("", "bothy-colors-*")
	if err != nil {
		return skip("could not create a temporary file")
	}
	path := out.Name()
	out.Close()
	defer os.Remove(path)

	cmd := exec.Command("vim", "-es", "-u", vimrc,
		"-c", fmt.Sprintf("call writefile([get(g:,'colors_name','NONE')],'%s')", path),
		"-c", "q")
	cmd.Stdin = strings.NewReader("")
	_ = cmd.Run()

	b, _ := os.ReadFile(path)
	name := strings.TrimSpace(string(b))
	if name == "" || name == "NONE" {
		return fail("vim's colorscheme did not load",
			"g:colors_name is unset after sourcing ~/.vimrc — the colorscheme file was not found",
			"colorschemes belong in ~/.vim/colors/, not ~/.vim/pack/*/start/")
	}
	return pass("vim colorscheme " + name + " loads")
}

// checkVimColorschemeLocation warns about the arrangement the previous check
// would catch only once it has already broken.
func checkVimColorschemeLocation(env Env) Result {
	if env.Config.Slots.Editor != "vim" {
		return skip("editor slot is not vim")
	}
	pack := filepath.Join(env.Platform.Home, ".vim", "pack")
	var found []string
	_ = filepath.WalkDir(pack, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "colors" && strings.HasSuffix(path, ".vim") {
			found = append(found, path)
		}
		return nil
	})
	if len(found) > 0 {
		return warn("colorschemes found under ~/.vim/pack",
			fmt.Sprintf("%d file(s); pack/*/start joins 'runtimepath' after .vimrc is sourced", len(found)),
			"copy them to ~/.vim/colors/ instead")
	}
	return pass("no colorschemes in the wrong place")
}

// checkOpener catches "Enter on a png says No such file or directory": Yazi's
// default opener is xdg-open, which a container has neither the binary nor any
// application for.
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
	shim := filepath.Join(env.Platform.LocalBin, "xdg-open")
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

func checkPathShadowing(env Env) Result {
	var shadowed []string
	for _, tool := range shadowable {
		paths := whichAll(tool)
		if len(paths) > 1 {
			shadowed = append(shadowed, fmt.Sprintf("%s (%s)", tool, strings.Join(paths, " before ")))
		}
	}
	if len(shadowed) > 0 {
		return warn("more than one copy of some tools is on PATH",
			strings.Join(shadowed, "; "),
			"check which one wins with 'command -v <tool>'; the first is used")
	}
	return pass("no duplicate tools on PATH")
}

func checkLocalBinOnPath(env Env) Result {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == env.Platform.LocalBin {
			return pass("~/.local/bin is on PATH")
		}
	}
	return fail("~/.local/bin is not on PATH",
		"bothy installs binaries there and nothing would find them",
		`add it: export PATH="$HOME/.local/bin:$PATH"`)
}

// checkThemePalette verifies a custom palette file still loads. It lives
// outside anything bothy manages, so it can be moved or edited into an invalid
// state long after the install that read it.
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
