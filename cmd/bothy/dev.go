package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/platform"
)

// cmdDev launches the workspace, and is what the `dev` shell function calls.
func cmdDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	dir := fs.String("dir", "", "directory to open in (default: the current one)")
	profile := fs.String("profile", "", "layout profile (default: the configured one)")
	window := fs.Bool("window", false, "always open a new Ghostty window")
	inPlace := fs.Bool("in-place", false, "always run in the current terminal")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// `dev attach` reattaches rather than starting a second session. Checked
	// after parsing, not on args[0], so flags may precede the word.
	if fs.NArg() > 0 && fs.Arg(0) == "attach" {
		return cmdAttach(fs.Args()[1:])
	}
	if *window && *inPlace {
		return fmt.Errorf("--window and --in-place contradict each other")
	}
	force := ""
	if *window {
		force = "window"
	} else if *inPlace {
		force = "in-place"
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}

	// The layout starts its own agent. Running `dev` from inside an agent
	// session would nest a second one in the first.
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
	if mode := decideLaunch(p, force); mode.Spawn {
		if err := spawnTerminal(p, target, name); err != nil {
			// A terminal that will not open is a reason to carry on here, not
			// to refuse: the workspace still works, it just cannot draw images.
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

// launch renders the profile and hands off to the multiplexer.
//
// This is where isolation happens: the tools read bothy's configs only because
// environment variables scoped to this one process tree point them there. Your
// shell keeps its own PATH and EDITOR, and your ~/.config is untouched.
func launch(p platform.Info, cfg config.Config, dir, profileName string) error {
	prof, err := install.LoadProfile(p, profileName)
	if err != nil {
		return err
	}
	kdl, err := layout.Render(prof, install.Commands(cfg))
	if err != nil {
		return err
	}

	// The guard belongs beside the write rather than beside cmdDev's setup
	// call: a check next to one caller protects only that caller.
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
	return runWithEnv(install.SessionEnv(p, cfg), bin, "--layout", layoutFile)
}

// lookPathIn prefers bothy's own bin, then falls back to the system PATH.
func lookPathIn(p platform.Info, name string) (string, error) {
	if own, ok := install.InstalledBinary(p, name); ok {
		return own, nil
	}
	return exec.LookPath(name)
}

func hopIntoContainer(container, dir, profile string) error {
	// --in-place: whatever decided to hop already settled the terminal
	// question, and the copy inside the container must not reopen it.
	inner := fmt.Sprintf("cd %s && bothy dev --in-place --dir %s --profile %s",
		shellQuote(dir), shellQuote(dir), shellQuote(profile))

	return containerHop(container, inner)
}

// containerHop runs one shell command inside a container, through whichever of
// toolbox and distrobox is installed. Shared, so `dev` and `attach` cannot
// diverge on which they support.
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
	plan, err := planAttach(p, cfg, args)
	if err != nil {
		return err
	}
	if plan.Container != "" {
		return containerHop(plan.Container, plan.Command)
	}
	return runWithEnv(plan.Env, plan.Bin, plan.Args...)
}

// attachPlan is what `bothy attach` will do: hop into the container the
// workspace lives in, or run the multiplexer here. A value, so the resolution
// can be asserted about in tests.
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

func planAttach(p platform.Info, cfg config.Config, args []string) (attachPlan, error) {
	mux := cfg.Slots.Mux
	if mux == "" {
		mux = "zellij"
	}
	if !p.InContainer() {
		if container := install.ContainerFor(p, cfg); container != "" {
			return attachPlan{Container: container, Command: attachCommand(mux, args)}, nil
		}
	}
	// lookPathIn, not exec.LookPath: bothy's own bin comes first, so a
	// multiplexer bothy supplied is found even when the ambient PATH has none.
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

// attachCommand builds the shell line for the container hop. Quoted, because
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
// Each agent advertises itself differently, so this is a list rather than a
// rule, and a refusal to nest rather than a promise never to.
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

	// A non-zero exit from the multiplexer is its business, not an error in
	// bothy — reporting "bothy: exit status 1" over it would be noise.
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
