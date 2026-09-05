package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
	"os/signal"
	"path/filepath"
	"syscall"
)

// cmdDev launches the workspace: bare `bothy`, and `bothy` with any flag.
func cmdDev(args []string) error {
	fs := flag.NewFlagSet("bothy", flag.ExitOnError)
	dir := fs.String("dir", "", "directory to open in (default: the current one)")
	profile := fs.String("profile", "", "layout profile (default: the configured one)")
	window := fs.Bool("window", false, "always open a new Ghostty window")
	inPlace := fs.Bool("in-place", false, "always run in the current terminal")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Checked after flag parsing so `bothy --profile x attach` still attaches.
	if fs.NArg() > 0 && fs.Arg(0) == "attach" {
		return cmdAttach(fs.Args()[1:])
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}

	// The layout starts its own agent; running from inside one would nest a second.
	if agent, nested := nestedAgent(); nested {
		return fmt.Errorf("already inside %s, and the layout would start another one\n"+
			"      exit this session first, then run bothy", agent)
	}

	cwd, _ := os.Getwd()
	plan, err := planLaunch(p, cfg, cwd, launchFlags{*dir, *profile, *window, *inPlace})
	if err != nil {
		return err
	}
	if fi, err := os.Stat(plan.Dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", plan.Dir)
	}

	// Before the install and before the hop: the answer decides where both of
	// them happen, and this is the last point at which the bothy asking is the
	// one whose terminal anybody is looking at.
	if err := askWhichBox(p, cfg, &plan); err != nil {
		return err
	}

	// Before the layout is rendered, so a cancelled setup leaves no debris.
	if err := ensureInstalled(p, cfg); err != nil {
		return err
	}

	// Before the window opens or the container is entered: both hand off to a
	// second bothy, and a refusal printed inside a window that then closes is
	// not a refusal anyone reads.
	if err := refuseIfInUse(p, cfg, plan.Dir); err != nil {
		return err
	}

	if plan.Spawn {
		if err := spawnTerminal(p, plan.Dir, plan.Profile); err != nil {
			// Not fatal: the workspace still runs here, without image previews.
			fmt.Fprintf(os.Stderr, "bothy: %v\n         running in this terminal instead\n", err)
		} else {
			return nil
		}
	}
	// After the spawn: the bothy that opens a window and exits is not the one
	// watching the session, the one living in the window is.
	defer ownSession(p, cfg, plan.Dir)()
	defer onHangup(endTheSession(p, cfg, plan.Dir))()

	if plan.Container != "" {
		return hopIntoContainer(plan.Container, plan.Dir, plan.Profile)
	}
	return launch(p, cfg, plan.Dir, plan.Profile, "")
}

// refuseIfInUse stops a launch into a session that already has a client. A
// backend it cannot resolve is not this function's problem to report: launch
// says so properly a moment later.
func refuseIfInUse(p platform.Info, cfg config.Config, dir string) error {
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return nil
	}
	env := install.SessionEnv(p, cfg)
	session := backend.SessionName(dir)
	n, counted := backend.Clients(bin, env, session, backend.Live(bin, env))

	switch clientVerdict(n, counted, mux.Owned(p.StateDir(), session)) {
	case proceed:
		return nil
	case refuse:
		return mux.InUse(session)
	}

	// Reclaiming is only worth anything if it worked, and the count says so
	// rather than the kill: something else may have attached in between.
	if mux.Reclaim(backend.Name(), session) == 0 {
		return mux.InUse(session)
	}
	if n, ok := backend.Clients(bin, env, session, backend.Live(bin, env)); ok && n > 0 {
		return mux.InUse(session)
	}
	fmt.Fprintf(os.Stderr, "bothy: reclaimed %s from a closed window\n", session)
	return nil
}

type verdict int

const (
	proceed verdict = iota
	refuse
	reclaim
)

// clientVerdict decides what a client count means. A probe that could not
// answer proceeds: a launch is not worth blocking on a diagnostic. A session
// somebody is watching is refused, which is the whole point of counting. What
// is left is a client with no terminal behind it.
func clientVerdict(n int, counted, owned bool) verdict {
	switch {
	case !counted, n == 0:
		return proceed
	case owned:
		return refuse
	}
	return reclaim
}

// onHangup runs cleanup when the terminal goes away, and returns the func that
// stops listening. A deferred call is no use here: the default disposition for
// a hangup is to die, and nothing deferred runs on the way.
//
// Only the hangup. An interrupt reaches this process too -- it sits in the
// window's foreground group -- and Ctrl-C in a pane must not end the workspace.
func onHangup(cleanup func()) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		if _, ok := <-ch; !ok {
			return
		}
		cleanup()
		signal.Stop(ch)
		// Re-raised so the exit looks like what it is, rather than a clean one.
		// Through os.Process, which every platform has; syscall.Kill is Unix.
		if self, err := os.FindProcess(os.Getpid()); err == nil {
			_ = self.Signal(syscall.SIGHUP)
		}
	}()
	return func() { signal.Stop(ch); close(ch) }
}

// endTheSession ends the multiplexer client this launch is showing.
//
// The client runs behind a container, where the terminal closing never reaches
// it: podman exec ignores the hangup, so it would hold the session open and
// the next launch would have to clean up after it. The pid namespace is shared
// with the container, so ending it is a signal from here rather than a round
// trip back through the container runtime.
func endTheSession(p platform.Info, cfg config.Config, dir string) func() {
	backend, _, err := muxPath(p, cfg)
	if err != nil || p.InContainer() {
		return func() {}
	}
	session := backend.SessionName(dir)
	return func() {
		_ = os.Remove(filepath.Join(p.StateDir(), "sessions", session))
		mux.Reclaim(backend.Name(), session)
	}
}

// ownSession records this process as the terminal showing the session, and
// returns the func that forgets it.
//
// Never from inside a container: that copy outlives the window too, so its
// record would never go stale and the project would be shut for good -- which
// is the failure this whole path exists to undo.
func ownSession(p platform.Info, cfg config.Config, dir string) func() {
	if p.InContainer() {
		return func() {}
	}
	backend, _, err := muxPath(p, cfg)
	if err != nil {
		return func() {}
	}
	return mux.Own(p.StateDir(), backend.SessionName(dir))
}

// launch renders the profile and hands off to the multiplexer with the
// session environment from install.SessionEnv, which is where isolation happens.
// agentOverride replaces the agent pane's command when non-empty, which is
// how `bothy confine` runs it inside a container without a second launch path.
func launch(p platform.Info, cfg config.Config, dir, profileName, agentOverride string) error {
	prof, err := install.LoadProfile(p, profileName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p.ConfigRoot()); err != nil {
		return fmt.Errorf("no workspace configured yet\n"+
			"      bothy keeps everything in one directory, derived from $HOME:\n"+
			"        %s\n"+
			"      that directory does not exist. 'bothy install' creates it.", p.ConfigRoot())
	}
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	env := install.SessionEnv(p, cfg)
	return backend.Open(mux.Request{
		Platform: p, Bin: bin, Session: backend.SessionName(dir), Dir: dir,
		Profile: prof, Commands: commandsWith(cfg, agentOverride), Env: env,
		Live: backend.Live(bin, env),
	})
}

// commandsWith is the pane commands, with the agent replaced when confined.
func commandsWith(cfg config.Config, agentOverride string) layout.Commands {
	cmds := install.Commands(cfg)
	if agentOverride != "" {
		cmds["agent"] = agentOverride
	}
	return cmds
}

func muxNames() []string {
	var out []string
	for _, b := range mux.All() {
		out = append(out, b.Name())
	}
	return out
}

// launchModeFor resolves workspace.launch against the flags: the setting is
// the standing answer, a flag overrides it for one run. Preferring to stay put
// should not mean typing --in-place every time.
func launchModeFor(cfg config.Config, window, inPlace bool) string {
	switch {
	case window:
		return "window"
	case inPlace:
		return "here"
	}
	return cfg.Workspace.Launch
}

// muxPath resolves the multiplexer this session will run. The slot, the
// fallback and the not-installed message were three copies apiece; the
// backend seam (#64) will take what remains behind here.
func muxPath(p platform.Info, cfg config.Config) (b mux.Backend, bin string, err error) {
	name := cfg.ProviderOrDefault("mux")
	b, ok := mux.For(name)
	if !ok {
		return nil, "", fmt.Errorf("no multiplexer backend for %q\n      bothy knows: %s",
			name, strings.Join(muxNames(), ", "))
	}
	if _, isNone := b.(mux.None); isNone {
		return b, "", nil
	}
	bin, err = lookPathIn(p, name)
	if err != nil {
		return nil, "", fmt.Errorf("%s is not installed — run 'bothy install'", name)
	}
	return b, bin, nil
}

// bothy's own bin first: a zellij bothy supplied is not on the ambient PATH.
func lookPathIn(p platform.Info, name string) (string, error) {
	if own, ok := install.InstalledBinary(p, name); ok {
		return own, nil
	}
	return exec.LookPath(name)
}

func hopIntoContainer(container, dir, profile string) error {
	// --in-place: the terminal question is settled outside; the inner copy
	// must not reopen it.
	inner := fmt.Sprintf("cd %s && bothy --in-place --dir %s --profile %s",
		shellQuote(dir), shellQuote(dir), shellQuote(profile))

	return containerHop(container, inner)
}

// containerHop runs one shell command inside a container, through whichever of
// toolbox and distrobox is installed.
func containerHop(container, command string) error {
	if bin, err := exec.LookPath("toolbox"); err == nil {
		return runInteractive(bin, "run", "--container", container, "bash", "-lc", command)
	}
	if bin, err := exec.LookPath("distrobox"); err == nil {
		return runInteractive(bin, "enter", container, "--", "bash", "-lc", command)
	}
	return fmt.Errorf("container %q is configured but neither toolbox nor distrobox is installed\n"+
		"      clear it with: bothy config set workspace.container \"\"", container)
}

func cmdAttach(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	backend, _, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	dir := workspaceDir(p, cfg, cwd)
	plan, err := planAttach(p, cfg, dir, backend.SessionName(dir), args)
	if err != nil {
		return err
	}
	// Same reason ownSession refuses inside a container: the copy in there
	// outlives the window, so its record would never go stale.
	if plan.Session != "" && !p.InContainer() {
		defer mux.Own(p.StateDir(), plan.Session)()
	}
	if plan.Container != "" {
		return containerHop(plan.Container, plan.Command)
	}
	return runInteractiveEnv(plan.Env, plan.Bin, plan.Args...)
}

type launchFlags struct {
	Dir, Profile    string
	Window, InPlace bool
}

// launchPlan is what `bothy` will do: spawn a terminal, hop, or run here.
type launchPlan struct {
	Dir, Profile string
	// Reason says why a spawn was chosen, for the message if it then fails.
	Spawn  bool
	Reason string
	// Container is empty to run here.
	Container string
}

// decide is a seam: the real one asks the terminal and the host (ADR-011).
var decide = decideLaunch

// planLaunch decides, touching no disk, so the tree can be tested rather than
// only run. cwd and the terminal question come in for that reason.
func planLaunch(p platform.Info, cfg config.Config, cwd string, f launchFlags) (launchPlan, error) {
	if f.Window && f.InPlace {
		return launchPlan{}, fmt.Errorf("--window and --in-place contradict each other")
	}
	plan := launchPlan{Dir: workspaceDir(p, cfg, cwd), Profile: f.Profile}
	if f.Dir != "" {
		plan.Dir = expandDir(f.Dir, p.Home)
	}
	if plan.Profile == "" {
		plan.Profile = cfg.Profile
	}

	// Before the container hop, so the window opens once and on the host.
	if d := decide(p, launchModeFor(cfg, f.Window, f.InPlace)); d.Spawn {
		plan.Spawn, plan.Reason = true, d.Reason
	}
	// Home is shared, so $PWD means the same thing on both sides.
	if !p.InContainer() {
		plan.Container = install.ContainerFor(p, cfg, plan.Dir)
	}
	return plan, nil
}

// attachPlan is what `bothy attach` will do: hop into the container the
// workspace lives in, or run the multiplexer here.
type attachPlan struct {
	// Container and Command are the container and the shell line to run in it,
	// both empty when attaching here.
	Container string
	Command   string
	// Bin, Args and Env are the multiplexer to run in this shell.
	Bin  string
	Args []string
	Env  []string
	// Session is what is being joined, which is what gets claimed. The
	// refusal names `bothy attach` as the way in, so a client it creates has
	// to be marked or the next launch would reclaim it.
	Session string
}

func planAttach(p platform.Info, cfg config.Config, dir, session string, args []string) (attachPlan, error) {
	mux := cfg.ProviderOrDefault("mux")
	// Bare `bothy attach` means this project's session. Naming one explicitly
	// still works, which is how you reach a session for somewhere else.
	if len(args) == 0 && session != "" {
		args = []string{session}
	}
	joined := ""
	if len(args) > 0 {
		joined = args[0]
	}
	if !p.InContainer() {
		if container := install.ContainerFor(p, cfg, dir); container != "" {
			return attachPlan{Container: container, Command: attachCommand(args), Session: joined}, nil
		}
	}
	bin, err := lookPathIn(p, mux)
	if err != nil {
		return attachPlan{}, fmt.Errorf("%s is not installed", mux)
	}
	return attachPlan{
		Bin:     bin,
		Session: joined,
		Args:    append([]string{"attach"}, args...),
		// The client reads config too -- keybindings in particular -- so
		// without this an attach reads your zellij config while the session
		// it joins was launched with bothy's.
		Env: install.SessionEnv(p, cfg),
	}, nil
}

// attachCommand builds the shell line for the container hop, quoted because
// it is interpolated into `bash -lc`.
//
// It runs bothy, not the multiplexer: bothy's bin directory is not on a login
// shell's PATH inside the container, and the copy in there resolves the
// binary out of that directory and applies the session environment -- which
// is what stops the client reading the user's own zellij config instead of
// bothy's. Same shape as hopIntoContainer, for the same reasons.
func attachCommand(args []string) string {
	parts := make([]string, 0, len(args)+2)
	parts = append(parts, "bothy", "attach")
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// nestedAgent reports whether this shell is already inside an agent session.
// Each agent advertises itself differently, so each says which variables it
// exports -- the list used to live here as well as in slots/, and the two
// disagreed about which agents existed.
func nestedAgent() (string, bool) {
	all, err := slots.All()
	if err != nil {
		return "", false
	}
	for _, pr := range all {
		if pr.Slot != "agent" {
			continue
		}
		for _, name := range pr.Detect {
			if os.Getenv(name) != "" {
				return pr.Name, true
			}
		}
	}
	return "", false
}

// runInteractive replaces this process's stdio with the child's, so the
// multiplexer owns the terminal rather than talking through a pipe.
func runInteractive(name string, args ...string) error {
	return runInteractiveEnv(nil, name, args...)
}

func runInteractiveEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return err
}

// shellQuote makes a value safe to embed in the `bash -lc` container hop.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
