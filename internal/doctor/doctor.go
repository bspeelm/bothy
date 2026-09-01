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
	"os/exec"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
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
	// RunsIn is the container bothy will hop into to launch the workspace, or
	// "" when it runs here. Set on the host when an install happened inside a
	// toolbox: the tools live there, so checking for them here reports every
	// one of them missing and every report is wrong.
	RunsIn string
	// MuxBin is the multiplexer binary bothy will actually launch, resolved
	// through its own bin first. Checking the system's copy instead reports
	// confidently about a binary that is not the one being used.
	MuxBin string
	// Version is the running binary's version, for comparison with the one
	// recorded in the manifest. Passed in rather than read here, so that
	// internal/doctor keeps knowing nothing about package main.
	Version string
	// ToolEnv is the environment bothy's session runs tools with. Checks that
	// invoke a tool must use it, or they interrogate the user's config instead
	// of bothy's — a check that confidently reports on the wrong file is worse
	// than no check at all. The caller supplies it via install.SessionEnv.
	ToolEnv []string
}

// lookPath resolves a binary the way bothy's session will: its own bin first,
// then the system PATH.
//
// This exists because forgetting it has now caused the same bug four times.
// A check that resolves through the ambient PATH reports on a binary bothy is
// not going to run — and in a container where bothy supplied every tool, that
// meant the doctor announcing "0 from your system, 9 supplied by bothy" and
// "zellij is not installed" in the same report.
func (e Env) lookPath(name string) (string, error) {
	if own, ok := install.InstalledBinary(e.Platform, name); ok {
		return own, nil
	}
	return exec.LookPath(name)
}

// elsewhere reports that the workspace runs in another container, so a check
// that inspects tools here would be inspecting the wrong machine.
//
// bothy straddles a host and a container that share a home directory, so
// "where am I" and "where does the work happen" are different questions. A
// check that conflates them is confidently wrong rather than merely silent.
func (e Env) elsewhere() (Result, bool) {
	if e.RunsIn == "" {
		return Result{}, false
	}
	enter := "toolbox run -c " + e.RunsIn + " bothy doctor"
	if e.Platform.Container == platform.Distrobox {
		enter = "distrobox enter " + e.RunsIn + " -- bothy doctor"
	}
	return skip("the workspace runs in " + e.RunsIn + "; check there with '" + enter + "'"), true
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
		{ID: "config-keys", Run: checkConfigKeys},
		{ID: "config-age", Run: checkConfigAge},
		{ID: "watermark-image", Run: checkWatermarkImage},
		{ID: "zellij-config", Run: checkZellijConfig},
		{ID: "terminfo", Run: checkTerminfo},
		{ID: "opener", Run: checkOpener},
		{ID: "xdg-open-shim-guard", Run: checkXdgOpenShimGuard},
		{ID: "agent", Run: checkAgent},
		{ID: "editor", Run: checkEditor},
		{ID: "tool-provenance", Run: checkToolProvenance},
		{ID: "tools-reachable", Run: checkToolsReachable},
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
