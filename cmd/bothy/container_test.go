//go:build container

// A container test, run in CI against Fedora and Ubuntu, for the thing
// docs/PLAN.md §10 named as remaining before v0.2.0: proof that bothy installs
// and works on a distribution that is not Fedora.
//
// Behind a build tag because it needs a container runtime and a network, and
// `make check` must need neither. Run it by hand with:
//
//	make build && go test ./cmd/bothy/ -tags container -v
//
// The container's $HOME is a bind-mounted host directory, so every filesystem
// assertion is a plain WalkDir on this side -- no `docker diff`, and no
// dependence on what the base image happens to carry. That matters most for
// the assertion this test exists to make: that after the real tools have
// really run, nothing landed outside bothy's tree. The unit tests in
// internal/install can only prove that about the files bothy writes itself;
// this covers what `ya pkg`, `yazi` and `zellij` decide to write too.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/doctor"
)

var images = []string{"fedora:44", "ubuntu:24.04", "debian:trixie", "archlinux:latest"}

// The one distro-specific thing in this test, kept in one visible place
// because it is exactly the README's "what you need first": a missing entry
// here is a missing entry there.
//
// ca-certificates because without it Go finds no TLS roots and every download
// fails. ncurses because infocmp lives there on Fedora and checkTerminfo
// shells out to it.
var prep = map[string]string{
	"fedora": "dnf -y install git ncurses",
	"ubuntu": "apt-get update && apt-get install -y --no-install-recommends git ca-certificates ncurses-bin",
	// Debian ships infocmp and neither git nor TLS roots.
	"debian": "apt-get update && apt-get install -y --no-install-recommends git ca-certificates",
	// Arch ships ncurses and TLS roots in its base image; only git is missing.
	"archlinux": "pacman -Sy --noconfirm git",
}

// What the doctor must report inside a headless container after a full
// install: no display, no ghostty, no agent, every tool supplied by bothy.
//
// The test asserts this map's keys equal the report's IDs, not merely that
// these entries match. A twentieth check therefore fails here until somebody
// decides what it means on a headless machine, rather than quietly not being
// covered -- the same lesson the isolation job in ci.yml already encodes.
var onlineExpectation = map[string]doctor.Severity{
	"tool-data":             doctor.Pass, // named, not prevented: see ADR-022
	"yazi-config-discarded": doctor.Pass, // `ya cache clear` parses the config without a terminal
	"yazi-version":          doctor.Pass,
	"yazi-config-keys":      doctor.Pass,
	"yazi-plugins":          doctor.Pass, // needs git, which is why prep installs it
	"image-previews":        doctor.Pass, // no graphics terminal and no preview block: they agree
	"profile-renders":       doctor.Pass,
	"layout-built":          doctor.Skip, // only meaningful inside a live zellij session
	"terminal-capability":   doctor.Warn, // nothing here can draw images: the one expected warning
	"passthrough":           doctor.Skip, // none configured
	"isolation":             doctor.Pass,
	"confine":               doctor.Skip, // opt-in; nothing has asked for it here
	"quarantine":            doctor.Skip,
	"config-schema":         doctor.Pass,
	"config-keys":           doctor.Pass, // the only config is the one the test wrote
	// The test installs and checks in one run with one binary, so the version
	// the manifest records is the version doing the checking.
	"config-age":      doctor.Pass,
	"watermark-image": doctor.Skip, // none configured
	"mux-config":      doctor.Pass,
	"terminfo":        doctor.Pass, // only because prep installed infocmp
	// Warn: in a container the opener forwards to the host,
	// and neither base image ships flatpak-spawn to forward with. There is
	// nothing to open a file with here and nothing bothy can do about it.
	"opener":              doctor.Warn,
	"xdg-open-shim-guard": doctor.Skip, // no shared home, so no shim to guard
	"agent":               doctor.Skip, // slots.agent none, see runOnline
	// slots.editor none, for the same reason as the agent: bothy supplies no
	// editor, so demanding one would assert what the base image ships.
	"editor":          doctor.Skip,
	"tool-provenance": doctor.Pass,
	"tools-reachable": doctor.Pass,
	"theme-palette":   doctor.Skip, // the built-in palette
	"theme-reached":   doctor.Pass, // the palette reaches the files the tools read
	"session-named":   doctor.Skip, // nothing is inside a live session here
}

func TestBothyInstallsInAContainer(t *testing.T) {
	for _, image := range images {
		image := image
		t.Run(image+"/offline", func(t *testing.T) { runOffline(t, image) })
		t.Run(image+"/online", func(t *testing.T) { runOnline(t, image) })
	}
}

// runOffline is first because it is hermetic and takes seconds: it proves the
// config tree renders on this distribution with no network at all.
func runOffline(t *testing.T, image string) {
	b := start(t, image)

	out, code := b.run("install", "--offline")
	// The install itself succeeds; runDoctor is what exits non-zero, because
	// the tools it wanted are the ones --offline declined to fetch. Asserting
	// the exact code is a stronger claim than asserting a non-zero one.
	if code != 1 {
		t.Fatalf("install --offline exited %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "nothing outside that directory was touched") {
		t.Errorf("install did not state the isolation promise:\n%s", out)
	}
	b.assertNothingOutsideBothysTree()
}

func runOnline(t *testing.T, image string) {
	b := start(t, image)

	// Catches a libc or architecture problem in one line, before anything
	// expensive runs.
	if out, code := b.run("version"); code != 0 {
		t.Fatalf("bothy version exited %d on %s: %s", code, image, out)
	}

	// The README is explicit that bothy does not install the agent, so a CI
	// job that demanded `claude` would be testing npm. This is tuning the
	// config until the test passes, which is why runOnline pays for it at the
	// end by asserting the check still fires under the default config.
	if out, code := b.run("config", "set", "slots.agent", "none"); code != 0 {
		t.Fatalf("config set exited %d: %s", code, out)
	}
	if out, code := b.run("config", "set", "slots.editor", "none"); code != 0 {
		t.Fatalf("config set exited %d: %s", code, out)
	}

	if out, code := b.run("install"); code != 0 {
		t.Fatalf("install exited %d on %s:\n%s", code, image, out)
	}

	b.assertNothingOutsideBothysTree()
	assertReport(t, b.doctor(t), onlineExpectation)

	// The coarse assertion, stated the way a user would make it.
	if out, code := b.run("doctor"); code != 0 {
		t.Errorf("doctor exited %d:\n%s", code, out)
	}
	if out, code := b.run("layout"); code != 0 || !strings.Contains(out, "pane") {
		t.Errorf("layout exited %d without rendering panes:\n%s", code, out)
	}

	// With the agent back, `agent` must be the only thing that fails. That
	// proves the check still fires rather than having been configured away.
	if out, code := b.run("config", "set", "slots.agent", "claude-code"); code != 0 {
		t.Fatalf("config set exited %d: %s", code, out)
	}
	var failed []string
	for _, r := range b.doctor(t).Results {
		if r.Severity == doctor.Fail {
			failed = append(failed, r.ID)
		}
	}
	if len(failed) != 1 || failed[0] != "agent" {
		t.Errorf("failing checks under the default config = %v, want [agent] alone", failed)
	}

	if out, code := b.run("uninstall"); code != 0 {
		t.Fatalf("uninstall exited %d:\n%s", code, out)
	}
	b.assertUninstalled()
}

// box is one container's lifetime.
type box struct {
	t      *testing.T
	engine string
	name   string
	home   string // the host directory bind-mounted as $HOME
}

func engine(t *testing.T) string {
	for _, e := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	t.Skip("neither docker nor podman is installed")
	return ""
}

func start(t *testing.T, image string) *box {
	t.Helper()
	b := &box{
		t:      t,
		engine: engine(t),
		name:   "bothy-test-" + strings.NewReplacer(":", "-", ".", "-", "/", "-").Replace(image+"-"+t.Name()),
		home:   t.TempDir(),
	}

	// Copied host-side rather than with `cp`: home is a bind mount, so this
	// lands inside the container by construction.
	bin, err := filepath.Abs("../../bothy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("no binary at %s -- run `make build` first", bin)
	}
	src, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b.home, ".local", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.home, ".local", "bin", "bothy"), src, 0o755); err != nil {
		t.Fatal(err)
	}

	mount := b.home + ":/home/bothy"
	args := []string{"run", "-d", "--name", b.name}
	if b.engine == "podman" {
		// Rootless podman maps --user to a subuid, so a bind mount written
		// from inside would land under the wrong host uid. keep-id maps the
		// invoking user through instead. :Z relabels for SELinux, which is a
		// Fedora host concern and harmless elsewhere.
		args = append(args, "--userns=keep-id")
		mount += ":Z"
	} else {
		args = append(args, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	}
	args = append(args,
		"-v", mount,
		"-e", "HOME=/home/bothy",
		"-e", "PATH=/home/bothy/.local/bin:/usr/local/bin:/usr/bin:/bin",
		// An entry both distributions ship, so terminfo is a real check here
		// rather than a foregone failure.
		"-e", "TERM=xterm-256color",
		image, "sleep", "1800")

	if out, err := exec.Command(b.engine, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s run: %v\n%s", b.engine, err, out)
	}
	t.Cleanup(func() { _ = exec.Command(b.engine, "rm", "-f", b.name).Run() })

	distro := strings.SplitN(image, ":", 2)[0]
	if out, err := b.asRoot(prep[distro]); err != nil {
		t.Fatalf("preparing %s: %v\n%s", image, err, out)
	}
	return b
}

func (b *box) asRoot(script string) (string, error) {
	out, err := exec.Command(b.engine, "exec", "-u", "0", b.name, "sh", "-c", script).CombinedOutput()
	return string(out), err
}

// run invokes bothy inside the container and returns its output and exit code.
func (b *box) run(args ...string) (string, int) {
	b.t.Helper()
	cmd := exec.Command(b.engine, append([]string{"exec", b.name, "bothy"}, args...)...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		b.t.Fatalf("%s exec: %v", b.engine, err)
	}
	return string(out), code
}

func (b *box) doctor(t *testing.T) doctor.Report {
	t.Helper()
	out, _ := b.run("doctor", "--json")
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("doctor --json did not produce a report: %v\n%s", err, out)
	}
	return rep
}

// snapshot lists everything under the bind-mounted home, host-side.
func (b *box) snapshot() []string { return filesUnder(b.t, b.home) }

func (b *box) assertNothingOutsideBothysTree() {
	b.t.Helper()
	assertNothingUnexplained(b.t, b.snapshot())
}

func (b *box) assertUninstalled() {
	b.t.Helper()
	assertGone(b.t, b.snapshot())
}
