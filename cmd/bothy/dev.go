package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/platform"
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
	if *window && *inPlace {
		return fmt.Errorf("--window and --in-place contradict each other")
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}

	mode := launchModeFor(cfg, *window, *inPlace)

	// The layout starts its own agent; running from inside one would nest a second.
	if agent, nested := nestedAgent(); nested {
		return fmt.Errorf("already inside %s, and the layout would start another one\n"+
			"      exit this session first, then run bothy", agent)
	}

	target := *dir
	if target == "" {
		target = cfg.Workspace.ProjectDir
	}
	if target == "" {
		target, _ = os.Getwd()
	}
	target = expandDir(target, p.Home)
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", target)
	}

	name := *profile
	if name == "" {
		name = cfg.Profile
	}

	// Before the layout is rendered, so a cancelled setup leaves no debris.
	if err := ensureInstalled(p, cfg); err != nil {
		return err
	}

	// Open a terminal that can do the job, if this one cannot. Before the
	// container hop, so the window opens once, on the host.
	if decided := decideLaunch(p, mode); decided.Spawn {
		if err := spawnTerminal(p, target, name); err != nil {
			// Not fatal: the workspace still runs here, without image previews.
			fmt.Fprintf(os.Stderr, "bothy: %v\n         running in this terminal instead\n", err)
		} else {
			return nil
		}
	}

	// On the host with a container configured, hop in: the tools live inside
	// and home is shared, so $PWD means the same thing on both sides.
	if !p.InContainer() {
		if container := install.ContainerFor(p, cfg); container != "" {
			return hopIntoContainer(container, target, name)
		}
	}

	return launch(p, cfg, target, name)
}

// launch renders the profile and hands off to the multiplexer with the
// session environment from install.SessionEnv, which is where isolation happens.
func launch(p platform.Info, cfg config.Config, dir, profileName string) error {
	prof, err := install.LoadProfile(p, profileName)
	if err != nil {
		return err
	}
	kdl, err := layout.Render(prof, install.Commands(cfg))
	if err != nil {
		return err
	}

	if _, err := os.Stat(p.ConfigRoot()); err != nil {
		return fmt.Errorf("no workspace configured yet\n"+
			"      bothy keeps everything in one directory, derived from $HOME:\n"+
			"        %s\n"+
			"      that directory does not exist. 'bothy install' creates it.", p.ConfigRoot())
	}

	// Zellij reads layouts from disk, so the rendered profile goes into bothy's
	// own layout directory. Regenerated every launch: editing it does nothing.
	layoutDir := filepath.Join(install.ZellijDir(p), "layouts")
	if err := os.MkdirAll(layoutDir, 0o755); err != nil {
		return err
	}
	layoutFile := filepath.Join(layoutDir, profileName+".kdl")
	if err := os.WriteFile(layoutFile, []byte(kdl), 0o644); err != nil {
		return err
	}

	mux := cfg.Slots.Mux
	if mux == "" {
		mux = "zellij"
	}
	bin, err := lookPathIn(p, mux)
	if err != nil {
		return fmt.Errorf("%s is not installed — run 'bothy install'", mux)
	}

	if err := os.Chdir(dir); err != nil {
		return err
	}
	env := install.SessionEnv(p, cfg)
	session := sessionName(dir)
	live := liveSessions(bin, env)

	if !slices.Contains(live, session) {
		discardDeadSession(bin, env, session)
	}
	return runWithEnv(env, bin, launchArgs(session, layoutFile, live)...)
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

// sessionName is the multiplexer session for a project directory. One session
// per project is what lets `bothy attach` choose between them, so the name has
// to be derived from the directory rather than generated, and has to survive
// being a session name: zellij uses it as a directory under its cache.
func sessionName(dir string) string {
	var b strings.Builder
	for _, r := range filepath.Base(filepath.Clean(dir)) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			// Everything else collapses to one dash, so "my project" and
			// "my/project" cannot become the same name by different routes.
			if !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "bothy"
	}
	return "bothy-" + name
}

// launchArgs invokes the multiplexer for a session that may already be
// running. Attaching to a live session must not carry --layout: zellij applies
// one to an existing session by adding a tab, so a second `bothy` in the same
// project would grow the workspace rather than return to it.
func launchArgs(session, layoutFile string, live []string) []string {
	for _, s := range live {
		if s == session {
			return []string{"attach", session}
		}
	}
	return []string{"--layout", layoutFile, "attach", "--create", session}
}

// discardDeadSession removes a session of ours that has stopped.
//
// Attaching to an EXITED zellij session resurrects it: the saved layout comes
// back with every command suspended behind "Waiting to run", ignoring a
// changed profile. EXITED sessions are invisible to `list-sessions --short`,
// so the live check cannot see it coming. Errors are ignored -- the common
// case is no such session, and either way the next command creates one.
func discardDeadSession(bin string, env []string, session string) {
	cmd := exec.Command(bin, "delete-session", session)
	cmd.Env = env
	_ = cmd.Run()
}

// liveSessions asks the multiplexer which sessions are running, through
// bothy's own environment -- with the ambient one it reads a different cache
// directory and reports none. No sessions and no multiplexer are one answer
// here, because creating is the right move for both.
func liveSessions(bin string, env []string) []string {
	cmd := exec.Command(bin, "list-sessions", "--short", "--no-formatting")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var live []string
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			live = append(live, s)
		}
	}
	return live
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
	dir, _ := os.Getwd()
	plan, err := planAttach(p, cfg, sessionName(dir), args)
	if err != nil {
		return err
	}
	if plan.Container != "" {
		return containerHop(plan.Container, plan.Command)
	}
	return runWithEnv(plan.Env, plan.Bin, plan.Args...)
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
}

func planAttach(p platform.Info, cfg config.Config, session string, args []string) (attachPlan, error) {
	mux := cfg.Slots.Mux
	if mux == "" {
		mux = "zellij"
	}
	// Bare `bothy attach` means this project's session. Naming one explicitly
	// still works, which is how you reach a session for somewhere else.
	if len(args) == 0 && session != "" {
		args = []string{session}
	}
	if !p.InContainer() {
		if container := install.ContainerFor(p, cfg); container != "" {
			return attachPlan{Container: container, Command: attachCommand(mux, args)}, nil
		}
	}
	bin, err := lookPathIn(p, mux)
	if err != nil {
		return attachPlan{}, fmt.Errorf("%s is not installed", mux)
	}
	return attachPlan{
		Bin:  bin,
		Args: append([]string{"attach"}, args...),
		// The client reads config too -- keybindings in particular -- so
		// without this an attach reads your zellij config while the session
		// it joins was launched with bothy's.
		Env: install.SessionEnv(p, cfg),
	}, nil
}

// attachCommand builds the shell line for the container hop, quoted because
// it is interpolated into `bash -lc`.
func attachCommand(mux string, args []string) string {
	parts := make([]string, 0, len(args)+2)
	parts = append(parts, shellQuote(mux), "attach")
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// nestedAgent reports whether this shell is already inside an agent session.
// Each agent advertises itself differently, so this is a list.
func nestedAgent() (string, bool) {
	for env, name := range map[string]string{
		"CLAUDECODE":      "a Claude Code session",
		"CLAUDE_CODE_SSE": "a Claude Code session",
		"AIDER_CHAT":      "an aider session",
		"GEMINI_CLI":      "a Gemini CLI session",
	} {
		if os.Getenv(env) != "" {
			return name, true
		}
	}
	return "", false
}

// runInteractive replaces this process's stdio with the child's, so the
// multiplexer owns the terminal rather than talking through a pipe.
func runInteractive(name string, args ...string) error {
	return runWithEnv(nil, name, args...)
}

// runWithEnv is runInteractive with an explicit environment. A nil env
// inherits this process's, which is what the container hop wants.
func runWithEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	// A non-zero exit from the multiplexer is its own status, not a bothy error.
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
