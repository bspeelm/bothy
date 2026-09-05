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

// listBoxes asks podman what boxes exist. podman and not `toolbox list`, which
// has no machine-readable output, and never `toolbox run`, which starts the
// container it is asked about. Runtime is confine's: inside a toolbox there is
// no podman, and the host's is reached through flatpak-spawn.
func listBoxes(p platform.Info) ([]toolbox, error) {
	runtime, err := confine.Runtime(p)
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, runtime[1:]...), "ps", "-a", "--format", "json")
	out, err := exec.Command(runtime[0], args...).Output()
	if err != nil {
		return nil, fmt.Errorf("could not list containers: %w", err)
	}
	return parseBoxes(out)
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
