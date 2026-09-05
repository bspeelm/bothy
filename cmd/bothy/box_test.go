package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/state"
)

// Trimmed from real `podman ps -a --format json` output, box names replaced.
// Names is an array and not a string, which is the shape a decode
// written from memory gets wrong; the last entry has no toolbox label, because
// podman lists every container and only some of them are boxes.
const podmanListing = `[
  {"Names": ["rust"], "State": "exited", "Status": "Exited (143) 58 minutes ago",
   "Labels": {"com.github.containers.toolbox": "true", "io.buildah.version": "1.43.2"}},
  {"Names": ["dev"], "State": "running", "Status": "Up 3 hours",
   "Labels": {"com.github.containers.toolbox": "true"}},
  {"Names": ["some-service"], "State": "running", "Status": "Up 3 hours",
   "Labels": {"io.buildah.version": "1.43.2"}}
]`

func TestBoxesAreReadFromPodmansJSON(t *testing.T) {
	boxes, err := parseBoxes([]byte(podmanListing))
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) != 2 {
		t.Fatalf("parseBoxes() found %d boxes, want 2 — a container with no box label is not a box", len(boxes))
	}
	if boxes[0].Name != "dev" || boxes[0].State != "running" {
		t.Errorf("first box = %+v, want dev/running", boxes[0])
	}
	if boxes[1].Name != "rust" || boxes[1].State != "exited" {
		t.Errorf("second box = %+v, want rust/exited", boxes[1])
	}
}

// `box ls` reports where sessions are, read from the process table, and not
// where any record says they belong -- so a session in an unexpected box shows
// up in the box it is in.
func TestBoxLsNamesTheSessionsInEachBox(t *testing.T) {
	boxes := []toolbox{{"rust", "exited"}, {"dev", "running"}}
	where := map[string]string{
		"bothy-api":    "dev",
		"bothy-legacy": "dev",
		"bothy-notes":  "",
	}
	out := renderBoxes(boxes, where, "dev")

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("renderBoxes() wrote %d lines, want 3 — the host needs one too:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[1], "* dev") {
		t.Errorf("this project's box is not marked: %q", lines[1])
	}
	if !strings.Contains(lines[1], "bothy-api, bothy-legacy") {
		t.Errorf("sessions missing from their box: %q", lines[1])
	}
	if strings.Contains(lines[0], "bothy-") {
		t.Errorf("rust is empty but claims sessions: %q", lines[0])
	}
	if !strings.Contains(lines[2], "bothy-notes") {
		t.Errorf("a session outside every box is not reported: %q", lines[2])
	}
}

// Every reason the prompt stays quiet. One toolbox or none is not a choice,
// and a machine like that has to behave exactly as it did before the prompt
// existed -- including recording nothing.
func TestTheFirstRunPromptStaysQuiet(t *testing.T) {
	const dir = "/w"
	inBox := platform.Info{Container: platform.Toolbx, ContainerName: "rust"}
	pinned := config.Default()
	pinned.Workspace.Container = "rust"
	answered := state.Boxes{dir: ""}

	cases := []struct {
		why     string
		p       platform.Info
		cfg     config.Config
		boxes   state.Boxes
		choices int
		tty     bool
		want    bool
	}{
		{"a fresh project with boxes to choose from", platform.Info{}, config.Default(), state.Boxes{}, 5, true, true},
		{"bothy is already inside a box", inBox, config.Default(), state.Boxes{}, 5, true, false},
		{"workspace.container settles it", platform.Info{}, pinned, state.Boxes{}, 5, true, false},
		{"this project answered before", platform.Info{}, config.Default(), answered, 5, true, false},
		{"nothing can read the reply", platform.Info{}, config.Default(), state.Boxes{}, 5, false, false},
		{"one box is not a choice", platform.Info{}, config.Default(), state.Boxes{}, 1, true, false},
		{"no boxes at all", platform.Info{}, config.Default(), state.Boxes{}, 0, true, false},
	}
	for _, tc := range cases {
		if got := shouldAsk(tc.p, tc.cfg, tc.boxes, dir, tc.choices, tc.tty); got != tc.want {
			t.Errorf("shouldAsk(%s) = %v, want %v", tc.why, got, tc.want)
		}
	}
}

// Enter is today's behaviour, and an answer nobody can read is not recorded as
// one: a bad reply leaves the project unanswered so the next launch asks again.
func TestTheReplyIsReadAsANumberOrAName(t *testing.T) {
	names := []string{"", "rust", "dev"}
	cases := []struct {
		line, want string
		ok         bool
	}{
		{"\n", "dev", true},
		{"  \n", "dev", true},
		{"0\n", "", true},
		{"1\n", "rust", true},
		{"rust\n", "rust", true},
		{"3\n", "", false},
		{"-1\n", "", false},
		{"docs\n", "", false},
	}
	for _, tc := range cases {
		got, ok := pickBox(tc.line, names, "dev")
		if got != tc.want || ok != tc.ok {
			t.Errorf("pickBox(%q) = %q, %v; want %q, %v", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

// A box holding a live session is not free to stop: the session dies with it.
func TestBoxStopRefusesABoxSomethingIsUsing(t *testing.T) {
	boxes := []toolbox{{"rust", "running"}}
	stop, err := stopVerdict(boxes, "rust", []string{"bothy-notes"})
	if stop || err == nil {
		t.Fatalf("stopVerdict() = %v, %v; want a refusal", stop, err)
	}
	if !strings.Contains(err.Error(), "bothy-notes") {
		t.Errorf("the refusal does not name what is using it: %v", err)
	}
}

// The veto comes from the process table, never from the record. A project
// recorded for a box and then deleted has no session, and must not keep
// vetoing on behalf of a directory that is gone.
func TestBoxStopIgnoresAProjectThatIsGone(t *testing.T) {
	stop, err := stopVerdict([]toolbox{{"rust", "running"}}, "rust", nil)
	if err != nil || !stop {
		t.Fatalf("stopVerdict() = %v, %v; want a stop", stop, err)
	}
}

// Stopping a stopped box is not an error and not a podman call.
func TestBoxStopDoesNotShootAStoppedBox(t *testing.T) {
	stop, err := stopVerdict([]toolbox{{"rust", "exited"}}, "rust", nil)
	if err != nil || stop {
		t.Fatalf("stopVerdict() = %v, %v; want a quiet no-op", stop, err)
	}
	if _, err := stopVerdict([]toolbox{{"rust", "exited"}}, "docs", nil); err == nil {
		t.Error("stopVerdict() accepted a box that does not exist")
	}
}

// This inverts the house convention on purpose: confirmDownloads defaults to
// yes because refusing costs a download, and here proceeding costs a running
// session. So silence, an unreadable reply and no terminal all mean no.
func TestBoxUseWillNotEndASessionUnasked(t *testing.T) {
	cases := []struct {
		why      string
		yes, tty bool
		reply    string
		want     bool
	}{
		{"an explicit y", false, true, "y\n", true},
		{"yes spelt out", false, true, "YES\n", true},
		{"just Enter", false, true, "\n", false},
		{"anything else", false, true, "maybe\n", false},
		{"no terminal to ask on", false, false, "", false},
		{"--yes", true, false, "", true},
	}
	for _, tc := range cases {
		if got := mayEnd(tc.yes, tc.tty, tc.reply); got != tc.want {
			t.Errorf("mayEnd(%s) = %v, want %v", tc.why, got, tc.want)
		}
	}
}

// The drift fence. toolbox owns creation -- the image, the uid and gid
// mapping, the thirty-odd bind mounts, the init-container step -- and the day
// someone "improves" this into a podman invocation, this fails. The name is
// positional because `toolbox create` has no --container flag.
func TestBoxCreateDelegatesRatherThanReimplementing(t *testing.T) {
	got := createArgs("scratch")
	if len(got) != 2 || got[0] != "create" || got[1] != "scratch" {
		t.Fatalf("createArgs() = %q, want [create scratch]", got)
	}
	for _, banned := range []string{"-v", "--userns", "--user", "run", "fedora-toolbox", "--security-opt"} {
		if slices.Contains(got, banned) {
			t.Errorf("createArgs() contains %q — that is podman's job, and toolbox's to keep", banned)
		}
	}
}

// `bothy box use <box> --yes` is the order people type, and a flag.FlagSet
// stops parsing at the first operand, so it read as two operands and failed.
func TestTheYesFlagIsFoundWhereverItIsTyped(t *testing.T) {
	cases := []struct {
		args []string
		box  string
		yes  bool
	}{
		{[]string{"rust"}, "rust", false},
		{[]string{"rust", "--yes"}, "rust", true},
		{[]string{"--yes", "rust"}, "rust", true},
		{[]string{"-yes", "rust"}, "rust", true},
		{[]string{"host"}, "host", false},
	}
	for _, tc := range cases {
		rest, yes := takeYes(tc.args)
		if len(rest) != 1 || rest[0] != tc.box || yes != tc.yes {
			t.Errorf("takeYes(%q) = %q, %v; want [%s], %v", tc.args, rest, yes, tc.box, tc.yes)
		}
	}
}

// Run from a pane of the session it is moving, `box use` inherits an
// environment saying bothy already has a terminal open -- true of the one now
// being torn down. Left to the usual decision the new workspace would open in
// place, in a dying pane, which is why nothing appeared to happen.
func TestMovingFromInsideTheSessionAsksForAWindow(t *testing.T) {
	if got := reopenArgs(true); len(got) != 1 || got[0] != "--window" {
		t.Errorf("reopenArgs(inside) = %q, want [--window]", got)
	}
	if got := reopenArgs(false); got != nil {
		t.Errorf("reopenArgs(outside) = %q, want the usual decision", got)
	}
}
