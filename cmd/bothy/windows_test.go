//go:build windowsjob

// The Windows end-to-end job. ADR-012 says a platform is supported when CI
// installs it, runs the doctor and matches an expected table; ADR-054 says
// Windows is beta until this has been green for a while.
//
// The tag is `windowsjob` rather than `windows` on purpose: //go:build windows
// is satisfied by GOOS, so a plain `go test ./...` on a Windows machine would
// compile this and the opt-in gate would not exist. Run it by hand with:
//
//	go build -o bothy.exe ./cmd/bothy
//	go test ./cmd/bothy/ -tags windowsjob -v
//
// Like the macOS job it installs into a temporary home on the machine itself
// and walks it, sharing the three assertions in harness_test.go so the jobs
// cannot drift apart in what they demand.
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

// What the doctor must report on Windows after a full install. A new check
// fails here until somebody decides what it means on Windows, rather than
// quietly going uncovered.
var windowsExpectation = map[string]doctor.Severity{
	"yazi-config-discarded": doctor.Pass,
	"yazi-version":          doctor.Pass,
	"yazi-config-keys":      doctor.Pass,
	"yazi-plugins":          doctor.Pass,
	// Off, and correctly: Yazi sends the iTerm2 protocol on the terminals
	// Windows has, and no multiplexer bothy supports carries it. The config
	// and the verdict agree, which is what this check compares.
	"image-previews":      doctor.Warn,
	"profile-renders":     doctor.Pass,
	"layout-built":        doctor.Skip, // only meaningful inside a live session
	"pane-fields":         doctor.Skip, // same: there are no panes to describe
	"one-client":          doctor.Skip, // same: no session to count clients on
	"terminal-capability": doctor.Warn, // a CI runner draws nothing
	"passthrough":         doctor.Skip,
	"isolation":           doctor.Pass,
	"confine":             doctor.Skip, // podman; Linux and macOS only
	"sshfs":               doctor.Skip, // FUSE; Linux and macOS only
	"tool-data":           doctor.Pass,
	"quarantine":          doctor.Skip, // an Apple flag
	"config-schema":       doctor.Pass,
	"config-keys":         doctor.Pass,
	// The job builds with the Makefile's flags and not its version stamp, so
	// bothy reports itself a source build and declines the comparison.
	"config-age":      doctor.Skip,
	"watermark-image": doctor.Skip, // none configured
	"mux-config":      doctor.Pass,
	// No infocmp on Windows, so the question cannot be answered. A correct
	// skip rather than a gap: this is the one check whose macOS Pass cannot
	// hold here.
	"terminfo": doctor.Skip,
	// The check this job exists to prove. Yazi's Windows build links the
	// Microsoft C runtime dynamically and a fresh Windows does not ship it;
	// without this the browser pane is empty with nothing on screen to say
	// why. The runner image has it.
	"msvc-runtime": doctor.Pass,
	// `start` is a cmd builtin, so what must exist is the shell Yazi runs the
	// opener through -- the Windows cousin of the macOS `open` check.
	"opener":              doctor.Pass,
	"xdg-open-shim-guard": doctor.Skip, // freedesktop; nothing to guard here
	"agent":               doctor.Skip, // slots.agent none, see below
	"editor":              doctor.Skip, // slots.editor none, for the same reason
	"tool-provenance":     doctor.Pass,
	"tools-reachable":     doctor.Pass,
	"theme-palette":       doctor.Skip, // the built-in palette
	"theme-reached":       doctor.Pass,
	"session-named":       doctor.Skip, // nothing is inside a live session here
}

func TestBothyInstallsOnWindows(t *testing.T) {
	w := startWindows(t)

	if out, code := w.run("version"); code != 0 {
		t.Fatalf("bothy version exited %d: %s", code, out)
	}

	for _, slot := range []string{"slots.agent", "slots.editor"} {
		if out, code := w.run("config", "set", slot, "none"); code != 0 {
			t.Fatalf("config set %s exited %d: %s", slot, code, out)
		}
	}

	if out, code := w.run("install"); code != 0 {
		t.Fatalf("install exited %d:\n%s", code, out)
	}

	assertNothingUnexplained(t, w.snapshot())
	assertReport(t, w.doctor(), windowsExpectation)

	// The seam this port turned on. Before it, every tool read "not installed"
	// on a machine where they were demonstrably present: the lookup wanted a
	// name without .exe, and the mode test wanted an execute bit Windows does
	// not have.
	out, _ := w.run("tools")
	for _, name := range []string{"zellij", "yazi"} {
		if !strings.Contains(out, name) || !strings.Contains(out, "supplied by bothy") {
			t.Errorf("%s did not come from bothy on a bare PATH:\n%s", name, out)
		}
	}
	if strings.Contains(out, "not installed") {
		t.Errorf("a tool is still missing after install:\n%s", out)
	}

	// The mirror of the macOS #73 assertion: a .desktop file is a freedesktop
	// convention and Windows has none.
	entry, code := w.run("desktop-entry")
	if code == 0 {
		t.Errorf("desktop-entry succeeded on Windows, where the file means nothing:\n%s", entry)
	}

	if out, code := w.run("layout"); code != 0 || !strings.Contains(out, "pane") {
		t.Errorf("layout exited %d without rendering panes:\n%s", code, out)
	}

	// With the agent back, `agent` must be the only thing that fails, which
	// proves the checks still fire rather than having been configured away.
	if out, code := w.run("config", "set", "slots.agent", "claude-code"); code != 0 {
		t.Fatalf("config set exited %d: %s", code, out)
	}
	for _, r := range w.doctor().Results {
		if r.ID == "agent" && r.Severity != doctor.Fail {
			t.Errorf("with an agent configured and none installed, agent = %s, want fail", r.Severity)
		}
	}

	out, code = w.run("uninstall")
	if code != 0 {
		t.Fatalf("uninstall exited %d:\n%s", code, out)
	}
	// Windows holds a running program's image open, so this process cannot
	// delete its own executable. What uninstall owes the user there is to say
	// so and name the file, which is what this asserts and what the walk
	// below therefore leaves out.
	if !strings.Contains(out, "bothy.exe") {
		t.Errorf("uninstall did not name the binary it could not remove:\n%s", out)
	}
	assertGone(t, without(w.snapshot(), ".local/bin/bothy.exe"))
}

// without is files with one path dropped.
func without(files []string, drop string) []string {
	var out []string
	for _, f := range files {
		if f != drop {
			out = append(out, f)
		}
	}
	return out
}

// win is bothy installed into a temporary home on this machine.
type win struct {
	t    *testing.T
	home string
	bin  string
}

func startWindows(t *testing.T) *win {
	t.Helper()
	built, err := filepath.Abs("../../bothy.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(built); err != nil {
		t.Fatal("build bothy.exe first: this test drives the built binary")
	}

	home := t.TempDir()
	// Windows derives LocalAppData by expanding %USERPROFILE%, and verifies
	// the result exists. A temporary home without an AppData tree therefore
	// has no cache directory at all, and yazi and zellij both abort on
	// startup instead of reading the config this test is here to check.
	for _, d := range []string{"Local", "Roaming"} {
		if err := os.MkdirAll(filepath.Join(home, "AppData", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(home, ".local", "bin", "bothy.exe")
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
	return &win{t: t, home: home, bin: bin}
}

func (w *win) run(args ...string) (string, int) {
	w.t.Helper()
	cmd := exec.Command(w.bin, args...)
	// USERPROFILE, not HOME: that is what os.UserHomeDir reads on Windows, and
	// bothy's whole tree hangs off it. A PATH with only the system directories
	// so bothy has to supply every tool -- it cannot be empty, because the
	// runner's git and the shell live there.
	cmd.Env = []string{
		"USERPROFILE=" + w.home,
		"APPDATA=" + filepath.Join(w.home, "AppData", "Roaming"),
		"LOCALAPPDATA=" + filepath.Join(w.home, "AppData", "Local"),
		"SystemRoot=" + os.Getenv("SystemRoot"),
		"PATH=" + os.Getenv("SystemRoot") + `\System32;` + os.Getenv("SystemRoot") +
			`;C:\Program Files\Git\cmd`,
		// A Windows console sets no TERM, and without one the terminfo check
		// answers that instead of the question this table records. WezTerm
		// sets it, so this is what a Windows user's doctor looks at.
		"TERM=xterm-256color",
		"TEMP=" + os.Getenv("TEMP"),
		"TMP=" + os.Getenv("TMP"),
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		w.t.Fatalf("running bothy: %v", err)
	}
	return string(out), code
}

func (w *win) doctor() doctor.Report {
	w.t.Helper()
	out, _ := w.run("doctor", "--json")
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		w.t.Fatalf("doctor --json did not produce a report: %v\n%s", err, out)
	}
	return rep
}

// snapshot is everything under the temporary home except AppData, which is
// Windows' own per-user tree rather than bothy's. zellij and yazi resolve
// their cache and data directories through it whatever bothy sets -- the
// escape ADR-054 records -- and it is inside this home only because Windows
// derives it from USERPROFILE.
func (w *win) snapshot() []string {
	var out, windows []string
	for _, f := range filesUnder(w.t, w.home) {
		if strings.HasPrefix(f, "AppData/") {
			windows = append(windows, f)
			continue
		}
		out = append(out, f)
	}
	if len(windows) > 0 {
		w.t.Logf("%d file(s) under AppData, which belongs to Windows:\n  %s",
			len(windows), strings.Join(windows, "\n  "))
	}
	return out
}
