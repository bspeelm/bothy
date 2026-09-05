package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/state"
)

// Asking, once, which box a project belongs in. bothy guessed before, and the
// guess was where its own tools were installed -- a different question.

// enoughToChoose is how many boxes make the question worth asking.
const enoughToChoose = 2

// shouldAsk reports whether to ask which box this project belongs in. Silent
// when bothy is already inside one (that is the answer), when
// workspace.container settles it, when nothing can read a reply, when this
// project has answered before, and when there are too few boxes to choose
// between. Nothing is recorded in any of those cases, so a machine with one
// toolbox or none behaves exactly as it did before this existed.
func shouldAsk(p platform.Info, cfg config.Config, boxes state.Boxes, dir string, choices int, tty bool) bool {
	if p.InContainer() || cfg.Workspace.Container != "" || !tty || choices < enoughToChoose {
		return false
	}
	_, answered := boxes[dir]
	return !answered
}

// askWhichBox asks once per project and records the answer.
func askWhichBox(p platform.Info, cfg config.Config, plan *launchPlan) error {
	ask := func(choices int) bool {
		return shouldAsk(p, cfg, install.ProjectBoxes(p), plan.Dir, choices, isTerminal(os.Stdin))
	}
	// Twice, because listing boxes costs a podman call and after the first
	// launch the answer is already on disk: the rules needing no listing
	// decide first, and the real count confirms.
	if !ask(enoughToChoose) {
		return nil
	}
	boxes, err := listBoxes(p)
	if err != nil || !ask(len(boxes)) {
		return nil // no podman is no choice, and never a reason to fail a launch
	}

	names := []string{""}
	fmt.Printf("%s has not been opened before. which toolbox should it use?\n", plan.Dir)
	fmt.Println("  0) the host")
	for i, b := range boxes {
		names = append(names, b.Name)
		fmt.Printf("  %d) %-24s %s\n", i+1, b.Name, b.State)
	}
	fmt.Printf("choose [%s]: ", labelFor(plan.Container))

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return nil // no answer available; carry on with today's
	}
	choice, ok := pickBox(line, names, plan.Container)
	if !ok {
		fmt.Printf("not sure what %q means — opening in %s, and asking again next time\n",
			strings.TrimSpace(line), labelFor(plan.Container))
		return nil
	}
	if err := install.RecordBox(p, plan.Dir, choice); err != nil {
		return err
	}
	plan.Container = choice
	fmt.Printf("%s now opens in %s\n", plan.Dir, labelFor(choice))
	return nil
}

// pickBox reads the reply: a number, a name, or nothing for the default. The
// default is what bothy would have done anyway, so Enter changes nothing.
func pickBox(line string, names []string, def string) (string, bool) {
	answer := strings.TrimSpace(line)
	if answer == "" {
		return def, true
	}
	if n, err := strconv.Atoi(answer); err == nil {
		if n < 0 || n >= len(names) {
			return "", false
		}
		return names[n], true
	}
	for _, n := range names {
		if n == answer {
			return answer, true
		}
	}
	return "", false
}

func labelFor(box string) string {
	if box == "" {
		return "the host"
	}
	return box
}
