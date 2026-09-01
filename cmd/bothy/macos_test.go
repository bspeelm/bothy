//go:build macos

// The macOS end-to-end job. ADR-012 says a platform is supported when CI
// installs it, runs the doctor and matches an expected table; the README has
// listed macOS since v0.1.0 and nothing has ever proved it.
//
// Behind a build tag because it needs a Mac and a network, and `make check`
// must need neither. Run it by hand on one with:
//
//	make build && go test ./cmd/bothy/ -tags macos -v
//
// Unlike the container job this cannot bind-mount a home and inspect it from
// outside, because there is no container: it installs into a temporary HOME on
// the machine itself and walks that. The assertions are the same three, shared
// in harness_test.go, so the two jobs cannot drift apart in what they demand.
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/doctor"
)

// What the doctor must report on a Mac after a full install: no graphics
// terminal, no agent, every tool supplied by bothy.
//
// As in the container job, this map's keys must equal the report's IDs. A new
// check fails here until somebody decides what it means on a Mac, rather than
// quietly going uncovered.
var macExpectation = map[string]doctor.Severity{
	"yazi-config-discarded": doctor.Pass,
	"yazi-version":          doctor.Pass,
	"yazi-config-keys":      doctor.Pass,
	"yazi-plugins":          doctor.Pass, // git ships with the Xcode command line tools
	"image-previews":        doctor.Pass, // no graphics terminal and no preview block: they agree
	"profile-renders":       doctor.Pass,
	"layout-built":          doctor.Skip, // only meaningful inside a live zellij session
	"terminal-capability":   doctor.Warn, // a CI runner draws nothing
	"passthrough":           doctor.Skip,
	"isolation":             doctor.Pass,
	"tool-data":             doctor.Pass,
	"config-keys":           doctor.Pass,
	"config-age":            doctor.Pass,
	"watermark-image":       doctor.Skip, // off by default
	"zellij-config":         doctor.Pass,
	"terminfo":              doctor.Pass, // infocmp is in /usr/bin, and TERM is set below
	// The check this job exists to prove. macOS has `open`, not xdg-open, and
	// until #95 the generated yazi config named the wrong one on every Mac.
	"opener":              doctor.Pass,
	"xdg-open-shim-guard": doctor.Skip, // no shared home, so no shim to guard
	"agent":               doctor.Skip, // slots.agent none, see the config below
	"editor":              doctor.Skip, // slots.editor none, for the same reason
	"tool-provenance":     doctor.Pass,
	"tools-reachable":     doctor.Pass,
	"theme-palette":       doctor.Skip, // the built-in palette
}

func TestBothyInstallsOnMacOS(t *testing.T) {
	m := startMac(t)

	// Catches an architecture problem in one line, before anything expensive.
	if out, code := m.run("version"); code != 0 {
		t.Fatalf("bothy version exited %d: %s", code, out)
	}

	// bothy installs neither, and a job demanding them would be testing
	// Homebrew and npm rather than bothy. The assertion at the end pays for
	// this by proving the checks still fire.
	for _, slot := range []string{"slots.agent", "slots.editor"} {
		if out, code := m.run("config", "set", slot, "none"); code != 0 {
			t.Fatalf("config set %s exited %d: %s", slot, code, out)
		}
	}

	if out, code := m.run("install"); code != 0 {
		t.Fatalf("install exited %d:\n%s", code, out)
	}

	assertNothingUnexplained(t, m.snapshot())
	assertReport(t, m.doctor(), macExpectation)

	// With only the system directories on PATH, the tools bothy had to fetch
	// are the darwin half of bothy.lock under test: asset names, checksums and
	// archive layouts. macOS ships its own jq, which bothy leaves alone --
	// fill gaps, never replace -- so the claim is about the two it certainly
	// does not ship rather than a count.
	out, _ := m.run("tools")
	for _, name := range []string{"zellij", "yazi"} {
		if !strings.Contains(out, name) || !strings.Contains(out, "supplied by bothy") {
			t.Errorf("%s did not come from bothy on a bare PATH:\n%s", name, out)
		}
	}
	if strings.Contains(out, "not installed") {
		t.Errorf("a tool is still missing after install:\n%s", out)
	}

	if out, code := m.run("layout"); code != 0 || !strings.Contains(out, "pane") {
		t.Errorf("layout exited %d without rendering panes:\n%s", code, out)
	}

	// With the agent back, `agent` must be the only thing that fails. That
	// proves the check still fires rather than having been configured away.
	if out, code := m.run("config", "set", "slots.agent", "claude-code"); code != 0 {
		t.Fatalf("config set exited %d: %s", code, out)
	}
	rep := m.doctor()
	for _, r := range rep.Results {
		if r.ID == "agent" && r.Severity != doctor.Fail {
			t.Errorf("with an agent configured and none installed, agent = %s, want fail", r.Severity)
		}
	}

	if out, code := m.run("uninstall"); code != 0 {
		t.Fatalf("uninstall exited %d:\n%s", code, out)
	}
	assertGone(t, m.snapshot())
}

// mac is bothy installed into a temporary home on this machine.
type mac struct {
	t    *testing.T
	home string
	bin  string
}

func startMac(t *testing.T) *mac {
	t.Helper()
	if _, err := os.Stat("../../bothy"); err != nil {
		t.Fatal("run `make build` first: this test drives the built binary")
	}
	built, err := filepath.Abs("../../bothy")
	if err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	// Installed where a user would have it, so that uninstall removes the
	// binary it is actually running and the assertion afterwards means
	// something.
	bin := filepath.Join(home, ".local", "bin", "bothy")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, body, 0o755); err != nil {
		t.Fatal(err)
	}
	return &mac{t: t, home: home, bin: bin}
}

func (m *mac) run(args ...string) (string, int) {
	m.t.Helper()
	cmd := exec.Command(m.bin, args...)
	// A bare PATH, so bothy has to supply every tool rather than finding
	// whatever the runner image happens to carry. TERM so that infocmp has
	// something to look up; the runner sets none.
	cmd.Env = []string{
		"HOME=" + m.home,
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"TERM=xterm-256color",
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		m.t.Fatalf("running bothy: %v", err)
	}
	return string(out), code
}

func (m *mac) doctor() doctor.Report {
	m.t.Helper()
	out, _ := m.run("doctor", "--json")
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		m.t.Fatalf("doctor --json did not produce a report: %v\n%s", err, out)
	}
	return rep
}

func (m *mac) snapshot() []string { return filesUnder(m.t, m.home) }
