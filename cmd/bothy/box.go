package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"github.com/bspeelm/bothy/internal/confine"
	"github.com/bspeelm/bothy/internal/platform"
)

// The half of `bothy box` that touches nothing: decoding podman's listing and
// rendering it. Kept apart so the output can be tested on a machine with no
// podman, which is every machine CI runs on.

// toolbox is one container a container manager made.
type toolbox struct {
	Name  string
	State string
}

// boxLabels are the labels container managers stamp their containers with. A
// list because distrobox's containers are podman containers too and differ
// only by the label -- but that label is unverified here, so it is left out
// and adding it is one entry rather than a rewrite. The legacy
// com.github.debarshiray.toolbox is in the same position.
var boxLabels = []string{"com.github.containers.toolbox"}

// parseBoxes reads `podman ps -a --format json` and keeps the containers a
// container manager made. The filtering is here and not in podman's --filter
// because several label filters combine in a way that is not worth guessing
// at, and because it makes the exclusion testable.
//
// Names is an array; the container's name is its first entry.
func parseBoxes(out []byte) ([]toolbox, error) {
	var raw []struct {
		Names  []string
		State  string
		Labels map[string]string
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("could not read podman's listing: %w", err)
	}
	boxes := []toolbox{}
	for _, r := range raw {
		if len(r.Names) == 0 || !madeByABoxManager(r.Labels) {
			continue
		}
		boxes = append(boxes, toolbox{r.Names[0], r.State})
	}
	sort.Slice(boxes, func(i, j int) bool { return boxes[i].Name < boxes[j].Name })
	return boxes, nil
}

func madeByABoxManager(labels map[string]string) bool {
	for _, l := range boxLabels {
		if _, ok := labels[l]; ok {
			return true
		}
	}
	return false
}

// podman builds a podman invocation. Runtime is confine's: inside a toolbox
// there is no podman, and the host's is reached through flatpak-spawn.
func podman(p platform.Info, args ...string) (*exec.Cmd, error) {
	runtime, err := confine.Runtime(p)
	if err != nil {
		return nil, err
	}
	return exec.Command(runtime[0], append(append([]string{}, runtime[1:]...), args...)...), nil
}

// listBoxes asks podman what boxes exist. podman and not `toolbox list`, which
// has no machine-readable output, and never `toolbox run`, which starts the
// container it is asked about.
func listBoxes(p platform.Info) ([]toolbox, error) {
	cmd, err := podman(p, "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list containers: %w", err)
	}
	return parseBoxes(out)
}

// stopBox stops a container. podman, because toolbox has no stop at all --
// which is the awkwardness `bothy box stop` exists to hide.
func stopBox(p platform.Info, name string) error {
	cmd, err := podman(p, "stop", name)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}

// untouchable says why a box may not be acted on at all: it does not exist, or
// a live session is in it. Stop and rm share this and differ only in what they
// make of a box that is merely running.
//
// The record is deliberately not consulted. A project recorded for a box with
// nothing running in it is not a reason to refuse, or a directory deleted
// months ago would veto on behalf of nothing. Only a live session does.
func untouchable(boxes []toolbox, name string, using []string) (toolbox, error) {
	i := slices.IndexFunc(boxes, func(b toolbox) bool { return b.Name == name })
	switch {
	case i < 0:
		return toolbox{}, fmt.Errorf("no box called %s\n      'bothy box ls' lists them", name)
	case len(using) > 0:
		return toolbox{}, fmt.Errorf("%s is in use by %s\n      end it with 'bothy kill %s' first",
			name, strings.Join(using, ", "), using[0])
	}
	return boxes[i], nil
}

// stopVerdict reports whether there is a box to stop. A box already down is
// not an error and not a podman call.
func stopVerdict(boxes []toolbox, name string, using []string) (bool, error) {
	box, err := untouchable(boxes, name, using)
	if err != nil {
		return false, err
	}
	return box.State == "running", nil
}

// removeArgs is the command that unmakes one. toolbox again, and without
// --force: forcing would delete a box with something working in it, and
// nothing about removing a box is urgent enough to want that.
func removeArgs(name string) []string { return []string{"rm", name} }

// removeVerdict is stricter than stop, because stopping is reversible and this
// is not: a running box is refused outright rather than stopped on the way
// past.
func removeVerdict(boxes []toolbox, name string, using []string) error {
	box, err := untouchable(boxes, name, using)
	if err != nil {
		return err
	}
	if box.State == "running" {
		return fmt.Errorf("%s is running\n      stop it with 'bothy box stop %s' first", name, name)
	}
	return nil
}

// createArgs is the command that makes a box, and toolbox is the only thing
// that may run it: toolbox owns the image, the uid and gid mapping, the
// thirty-odd bind mounts and the init-container step, and a second
// implementation of that on raw podman is the drift this design refuses.
//
// The name is positional -- `toolbox create` has no --container flag.
func createArgs(name string) []string { return []string{"create", name} }

// confirmed reads a reply where silence means no. This inverts the house
// convention -- confirmDownloads defaults to yes, because refusing it costs a
// download -- since what these guard costs a running session or a box's
// contents. No terminal and no --yes is a no, not a shrug.
func confirmed(yes, tty bool, reply string) bool {
	if yes {
		return true
	}
	if !tty {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(reply)) {
	case "y", "yes":
		return true
	}
	return false
}

// renderBoxes lists every box with the sessions in it. where maps a session to
// the box it is really in, read from the process table rather than from any
// record, so a session somewhere unexpected is shown where it is. here is this
// project's box, marked so the listing answers "and which one am I".
func renderBoxes(boxes []toolbox, where map[string]string, here string) string {
	var b strings.Builder
	for _, box := range boxes {
		line(&b, markIf(box.Name == here), box.Name, box.State, sessionsIn(where, box.Name))
	}
	if onHost := sessionsIn(where, ""); len(onHost) > 0 {
		line(&b, markIf(here == ""), "(the host)", "", onHost)
	}
	return b.String()
}

func line(b *strings.Builder, mark, name, state string, sessions []string) {
	row := fmt.Sprintf("%s %-24s %-9s %s", mark, name, state, strings.Join(sessions, ", "))
	fmt.Fprintln(b, strings.TrimRight(row, " "))
}

func markIf(yes bool) string {
	if yes {
		return "*"
	}
	return " "
}

func sessionsIn(where map[string]string, box string) []string {
	var out []string
	for session, in := range where {
		if in == box {
			out = append(out, session)
		}
	}
	slices.Sort(out)
	return out
}

// knownBox refuses a name that is not a box, so a typo does not record a
// project into a container that does not exist. The host is always a valid
// answer and needs no listing.
func knownBox(p platform.Info, name string) error {
	if name == "" {
		return nil
	}
	boxes, err := listBoxes(p)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(boxes, func(b toolbox) bool { return b.Name == name }) {
		return nil
	}
	return fmt.Errorf("no box called %s\n"+
		"      'bothy box ls' lists them, 'bothy box create' makes one", name)
}

// toolboxBinary is the thing that makes boxes. distrobox is not accepted here
// even though bothy enters distrobox containers happily: its create flags are
// unverified, and guessing at another tool's syntax is how docs start lying.
func toolboxBinary() (string, error) {
	bin, err := exec.LookPath("toolbox")
	if err != nil {
		return "", fmt.Errorf("toolbox is not installed, and it is what makes boxes\n" +
			"      bothy manages boxes; it does not create them itself")
	}
	return bin, nil
}

// saidYesByDefault reads a reply where silence means yes -- the house
// convention, for an offer that costs nothing to accept.
func saidYesByDefault(reply string) bool {
	switch strings.ToLower(strings.TrimSpace(reply)) {
	case "", "y", "yes":
		return true
	}
	return false
}
