package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy box` -- which container this project runs in, what boxes exist, and
// the two verbs toolbox does not offer usably: moving a project and stopping a
// box nothing is in.

func cmdBox(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	dir := workspaceDir(p, cfg, cwd)
	if len(args) == 0 {
		return boxHere(p, cfg, dir)
	}
	switch args[0] {
	case "ls":
		return boxList(p, cfg, dir)
	case "use":
		return boxUse(p, cfg, dir, args[1:])
	case "stop":
		return boxStop(p, cfg, args[1:])
	case "create":
		return boxCreate(p, cfg, dir, args[1:])
	case "rm":
		return boxRemove(p, cfg, args[1:])
	default:
		return fmt.Errorf("usage: bothy box [ls|use|stop|create|rm]")
	}
}

// boxHere names the container and the rule that chose it. A launcher that
// picks a container silently is the thing being fixed, so the reason is part
// of the answer. It asks podman nothing, which is why it works everywhere.
func boxHere(p platform.Info, cfg config.Config, dir string) error {
	switch b := install.Resolve(p, cfg, dir); b.Name {
	case "":
		fmt.Printf("%s runs on the host\n", dir)
	default:
		fmt.Printf("%s runs in %s — %s\n", dir, b.Name, b.Reason)
	}
	return nil
}

// boxList shows every box and the sessions in it, marking this project's.
func boxList(p platform.Info, cfg config.Config, dir string) error {
	boxes, err := listBoxes(p)
	if err != nil {
		return err
	}
	if len(boxes) == 0 {
		fmt.Println("no toolboxes on this machine")
		return nil
	}
	fmt.Print(renderBoxes(boxes, whereSessionsAre(p, cfg), install.Resolve(p, cfg, dir).Name))
	return nil
}

// whereSessionsAre maps each live session to the box it is really in, read
// from the process table. Where this and the record disagree, this is right.
func whereSessionsAre(p platform.Info, cfg config.Config) map[string]string {
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return nil
	}
	where := map[string]string{}
	for _, s := range backend.Live(bin, install.SessionEnv(p, cfg)) {
		where[s] = mux.ServerBox(cfg.Slots.Mux, s)
	}
	return where
}

// boxUse moves this project to another box. The session has to end first:
// the multiplexer server runs inside the container, so there is no carrying a
// running session across. Hence the confirmation, and hence --yes.
func boxUse(p platform.Info, cfg config.Config, dir string, args []string) error {
	args, yes := takeYes(args)
	if len(args) != 1 {
		return fmt.Errorf("usage: bothy box use [--yes] <box>  (use 'host' for no box)")
	}
	name := args[0]
	if name == "host" {
		name = ""
	}
	if err := knownBox(p, name); err != nil {
		return err
	}
	if cfg.Workspace.Container != "" {
		fmt.Printf("note: workspace.container is set to %s and applies everywhere, so it wins\n"+
			"      clear it with: bothy config set workspace.container \"\"\n", cfg.Workspace.Container)
	}

	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	env := install.SessionEnv(p, cfg)
	session := backend.SessionName(dir)
	live := slices.Contains(backend.Live(bin, env), session)
	stake := fmt.Sprintf("%s is running and has to end before this project can move to %s.",
		session, labelFor(name))
	if live && !confirmDestructive(yes, stake, "end it?") {
		fmt.Println("left alone")
		return nil
	}

	// Recorded before anything is ended. Run from a pane of the session being
	// moved, this process does not outlive the kill below by default, and a
	// move that is lost halfway leaves the session gone and the project where
	// it started.
	if err := install.RecordBox(p, dir, name); err != nil {
		return err
	}
	fmt.Printf("%s now opens in %s\n", dir, labelFor(name))
	// Reporting, not probing: finding out for certain would mean entering the
	// box, and entering starts it.
	if in := install.InstalledIn(p); in != name {
		fmt.Printf("bothy resolved its tools in %s, so some may not exist in %s.\n"+
			"      run 'bothy doctor' there to find out\n", labelFor(in), labelFor(name))
	}

	inside := live && session == backend.CurrentSession()
	if live {
		if inside {
			// zellij hangs up this pane as it tears the session down, and the
			// default answer to a hangup is to die -- measured: the statement
			// after the kill never ran, so neither the record nor the reopen
			// happened. Nothing below is printed anywhere anyone can read it.
			signal.Ignore(syscall.SIGHUP)
		}
		fmt.Printf("ending %s and opening it again\n", session)
		if err := backend.Kill(bin, env, session); err != nil {
			return err
		}
	}
	// The move has happened. A workspace that cannot reopen right now is worth
	// saying, but it is not a failure of the thing that was asked for.
	if err := cmdDev(reopenArgs(inside)); err != nil {
		fmt.Printf("not reopened: %v\n      run 'bothy' when you are ready\n", err)
	}
	return nil
}

// reopenArgs says how to open the workspace again after a move. From inside
// the session being moved, the environment still says bothy already has a
// terminal open -- true of the one now closing -- so decideLaunch would run
// the new workspace in place, in a pane that is being torn down. A window has
// to be asked for.
func reopenArgs(insideTheSession bool) []string {
	if insideTheSession {
		return []string{"--window"}
	}
	return nil
}

// takeYes pulls --yes out wherever it appears. A flag.FlagSet stops parsing at
// the first operand, so `box use <box> --yes` -- the order people type -- would
// read as two operands and fail.
func takeYes(args []string) (rest []string, yes bool) {
	for _, a := range args {
		if a == "--yes" || a == "-yes" {
			yes = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, yes
}

// confirmDestructive asks a question whose default is no, saying what is at
// stake first. Separate from confirmDownloads, which defaults to yes: the cost
// of refusing there is a download, and here it is a running session or a box.
func confirmDestructive(yes bool, stake, question string) bool {
	tty := isTerminal(os.Stdin)
	reply := ""
	if !yes && tty {
		fmt.Println(stake)
		fmt.Print(question + " [y/N] ")
		reply, _ = bufio.NewReader(os.Stdin).ReadString('\n') // an unreadable reply is not a yes
	}
	return confirmed(yes, tty, reply)
}

// boxStop stops a box nothing is using. podman's job, because toolbox has no
// stop -- and if you are inside a box you have no podman either, which is the
// whole reason this reads as one command.
func boxStop(p platform.Info, cfg config.Config, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: bothy box stop <box>")
	}
	boxes, err := listBoxes(p)
	if err != nil {
		return err
	}
	name := args[0]
	var using []string
	for session, in := range whereSessionsAre(p, cfg) {
		if in == name {
			using = append(using, session)
		}
	}
	slices.Sort(using)

	stop, err := stopVerdict(boxes, name, using)
	if err != nil {
		return err
	}
	if !stop {
		fmt.Printf("%s is not running\n", name)
		return nil
	}
	if err := stopBox(p, name); err != nil {
		return err
	}
	fmt.Printf("stopped %s — its packages are still there, 'toolbox run' starts it again\n", name)
	return nil
}

// boxCreate delegates to toolbox and then does the two things toolbox cannot:
// remember that this project belongs in the new box, and offer to install
// bothy's tools inside it, which is where the missing-tool trap begins.
func boxCreate(p platform.Info, cfg config.Config, dir string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: bothy box create <name>")
	}
	name := args[0]
	bin, err := toolboxBinary()
	if err != nil {
		return err
	}
	if err := runInteractive(bin, createArgs(name)...); err != nil {
		return err
	}
	if err := install.RecordBox(p, dir, name); err != nil {
		return err
	}
	fmt.Printf("%s now opens in %s\n", dir, name)

	if isTerminal(os.Stdin) {
		fmt.Print("install bothy's tools in it now? [Y/n] ")
		reply, err := bufio.NewReader(os.Stdin).ReadString('\n') // an unreadable reply declines
		if err == nil && saidYesByDefault(reply) {
			return containerHop(name, "bothy install")
		}
	}
	fmt.Printf("run 'bothy install' inside %s before opening a workspace there\n", name)
	return nil
}

// boxRemove deletes a box and forgets it. toolbox does the deletion, as it did
// the creation; what bothy adds is the record, because a project still
// pointing at a box that no longer exists would be sent somewhere that is not
// there.
func boxRemove(p platform.Info, cfg config.Config, args []string) error {
	args, yes := takeYes(args)
	if len(args) != 1 {
		return fmt.Errorf("usage: bothy box rm [--yes] <box>")
	}
	name := args[0]
	boxes, err := listBoxes(p)
	if err != nil {
		return err
	}
	if err := removeVerdict(boxes, name, sessionsIn(whereSessionsAre(p, cfg), name)); err != nil {
		return err
	}
	stake := fmt.Sprintf("removing %s deletes the container and everything installed in it.", name)
	if !confirmDestructive(yes, stake, "remove it?") {
		fmt.Println("left alone")
		return nil
	}
	bin, err := toolboxBinary()
	if err != nil {
		return err
	}
	if err := runInteractive(bin, removeArgs(name)...); err != nil {
		return err
	}
	fmt.Printf("removed %s\n", name)

	freed, err := install.ForgetBox(p, name)
	if err != nil {
		return err
	}
	for _, dir := range freed {
		fmt.Printf("%s now opens in %s\n", dir, labelFor(install.Resolve(p, cfg, dir).Name))
	}
	return nil
}
