// Package doctor detects the ways this workspace is known to break.
//
// The failures here are silent ones: Yazi discards a whole config and carries
// on, vim ignores a colorscheme without a word. A check restating an error the
// tool already prints loudly is not worth adding, and a fixed setup bug ships
// with a check that detects it (PLAN.md §0).
//
// Every check invoking a tool must resolve and run it the way the session will
// (lookPath, Env.ToolEnv): one reporting on a different binary than the
// launcher uses is worse than no check.
package doctor

import (
	"os/exec"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
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
// Capability is one of the five things a stack either gives you or does not
// (ADR-017). Most checks bear on none of them and leave it empty: whether
// config.toml has a typo says nothing about what the workspace can do.
type Capability string

const (
	Panes     Capability = "panes"
	Sessions  Capability = "sessions"
	Images    Capability = "images"
	Theme     Capability = "theme"
	Isolation Capability = "isolation"
)

// Capabilities is the set, in the order a report names them.
var Capabilities = []Capability{Panes, Sessions, Images, Theme, Isolation}

type Result struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	// Capability is what this check bears on, empty for the many that bear
	// on none.
	Capability Capability `json:"capability,omitempty"`
	Summary    string     `json:"summary"`
	// Detail explains what was actually observed.
	Detail string `json:"detail,omitempty"`
	// Fix is a single actionable line. Every failing check must have one.
	Fix string `json:"fix,omitempty"`
}

// Supplied is the capabilities the configured providers claim between them.
//
// A capability is a chain -- images needs a terminal that draws them, a mux
// that passes them through and a browser that asks -- so a claim is a
// contribution, not a guarantee. Only the negative direction is sound, and the
// one worth having: what nothing contributes to cannot happen.
func Supplied(c config.Config) map[Capability]bool {
	// Isolation is bothy's own doing, not a provider's.
	out := map[Capability]bool{Isolation: true}
	for _, provider := range c.Providers() {
		h, ok := slots.Get(provider)
		if !ok {
			continue
		}
		for _, name := range h.Provides {
			out[Capability(name)] = true
		}
	}
	return out
}

// Report is a full run.
type Report struct {
	Results []Result `json:"results"`
}

// Delivers reports what this stack can and cannot do, from the checks bearing
// on each capability: the worst severity wins, and a capability nothing checks
// comes back Skip. Twenty-three lines of check output answer the question only
// for a reader who already knows which lines matter.
func (r Report) Delivers() map[Capability]Severity {
	out := map[Capability]Severity{}
	for _, c := range Capabilities {
		out[c] = Skip
	}
	for _, res := range r.Results {
		if res.Capability == "" {
			continue
		}
		switch {
		case res.Severity == Fail:
			out[res.Capability] = Fail
		case res.Severity == Warn && out[res.Capability] != Fail:
			out[res.Capability] = Warn
		case res.Severity == Pass && out[res.Capability] == Skip:
			out[res.Capability] = Pass
		}
	}
	return out
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
	ID         string
	Capability Capability
	Run        func(Env) Result
}

// Env is everything the checks are allowed to look at.
type Env struct {
	Platform platform.Info
	Config   config.Config
	// ProfileName and PaneCount describe the layout profile in use.
	ProfileName string
	PaneCount   int
	// RunsIn is the container the workspace will launch in, or "" when it runs here.
	RunsIn string
	// MuxBin is the multiplexer binary bothy will actually launch, resolved through its own bin first.
	MuxBin string
	// Mux is the backend filling the mux slot, or nil when the configured name
	// has no implementation. Checks that ask a multiplexer anything go through
	// it rather than naming one.
	Mux mux.Backend
	// SessionName is what bothy would call this project's session. Passed in
	// because the naming belongs to the launcher, not to the doctor.
	SessionName string
	// Version is the running binary's version, to compare against the
	// manifest's. Passed in, so this package knows nothing about package main.
	Version string
	// ToolEnv is the session environment from install.SessionEnv; checks that invoke a tool must use it.
	ToolEnv []string
}

// lookPath resolves a binary the way bothy's session will: its own bin first, then PATH.
func (e Env) lookPath(name string) (string, error) {
	if own, ok := install.InstalledBinary(e.Platform, name); ok {
		return own, nil
	}
	return exec.LookPath(name)
}

// elsewhere reports that the workspace runs in another container, so a check
// that inspects tools here would inspect the wrong machine. Host and
// container share a home but not a PATH.
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
		res.Capability = c.Capability
		rep.Results = append(rep.Results, res)
	}
	return rep
}

// Checks is the full list, in the order they are reported.
func Checks() []Check {
	return []Check{
		{ID: "yazi-config-discarded", Capability: Isolation, Run: checkYaziConfigDiscarded},
		{ID: "yazi-version", Run: checkYaziVersion},
		{ID: "yazi-config-keys", Run: checkYaziConfigKeys},
		{ID: "yazi-plugins", Run: checkYaziPlugins},
		{ID: "image-previews", Capability: Images, Run: checkImagePreviews},
		{ID: "profile-renders", Capability: Panes, Run: checkProfileRenders},
		{ID: "layout-built", Capability: Panes, Run: checkLayoutBuilt},
		{ID: "terminal-capability", Capability: Images, Run: checkTerminalCapability},
		{ID: "passthrough", Capability: Isolation, Run: checkPassthrough},
		{ID: "isolation", Capability: Isolation, Run: checkIsolation},
		{ID: "tool-data", Capability: Isolation, Run: checkToolData},
		{ID: "config-keys", Run: checkConfigKeys},
		{ID: "config-age", Run: checkConfigAge},
		{ID: "watermark-image", Run: checkWatermarkImage},
		{ID: "zellij-config", Capability: Isolation, Run: checkZellijConfig},
		{ID: "terminfo", Run: checkTerminfo},
		{ID: "opener", Run: checkOpener},
		{ID: "xdg-open-shim-guard", Run: checkXdgOpenShimGuard},
		{ID: "agent", Run: checkAgent},
		{ID: "editor", Run: checkEditor},
		{ID: "tool-provenance", Run: checkToolProvenance},
		{ID: "tools-reachable", Run: checkToolsReachable},
		{ID: "theme-palette", Capability: Theme, Run: checkThemePalette},
		{ID: "theme-reached", Capability: Theme, Run: checkThemeReached},
		{ID: "session-named", Capability: Sessions, Run: checkSessionIsNamed},
	}
}

func pass(summary string) Result { return Result{Severity: Pass, Summary: summary} }

// note is a pass with something to say: the answer is fine, and worth knowing.
func note(summary, detail string) Result {
	return Result{Severity: Pass, Summary: summary, Detail: detail}
}
func skip(summary string) Result { return Result{Severity: Skip, Summary: summary} }
func fail(summary, detail, fix string) Result {
	return Result{Severity: Fail, Summary: summary, Detail: detail, Fix: fix}
}
func warn(summary, detail, fix string) Result {
	return Result{Severity: Warn, Summary: summary, Detail: detail, Fix: fix}
}
