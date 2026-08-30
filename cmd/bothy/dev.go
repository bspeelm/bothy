package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/install"
	"github.com/bothy-dev/bothy/internal/layout"
	"github.com/bothy-dev/bothy/internal/platform"
)

// cmdDev launches the workspace. This is the command the `dev` shell function
// calls, and the one moment the whole project exists to make good.
func cmdDev(args []string) error {
	// `dev attach` reattaches rather than starting a second session.
	if len(args) > 0 && args[0] == "attach" {
		return cmdAttach(args[1:])
	}

	fs := newFlagSet("dev")
	dir := fs.String("dir", "", "directory to open in (default: the current one)")
	profile := fs.String("profile", "", "layout profile (default: the configured one)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}

	// The layout starts its own agent. Running `dev` from inside an agent
	// session gives you a second one nested in the first, which is confusing
	// rather than useful — and the origin cheat sheet had to warn about it in
	// prose because nothing enforced it.
	if agent, nested := nestedAgent(); nested {
		return fmt.Errorf("already inside %s, and the layout would start another one\n"+
			"      exit this session first, then run dev", agent)
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

	// On the host with a container configured, hop in: the multiplexer and the
	// tools live inside, home is shared, so $PWD means the same thing on both
	// sides. This is the case a plain shell alias cannot handle.
	if !p.InContainer() {
		if container := cfg.ContainerFor(p); container != "" {
			return hopIntoContainer(container, target, name)
		}
	}

	return launch(p, cfg, target, name)
}

// launch renders the profile and hands off to the multiplexer.
func launch(p platform.Info, cfg config.Config, dir, profileName string) error {
	prof, err := install.LoadProfile(p, profileName)
	if err != nil {
		return err
	}
	kdl, err := layout.Render(prof, install.Commands(cfg))
	if err != nil {
		return err
	}

	// Zellij reads layouts from disk, so the rendered profile is written to its
	// layout directory under a bothy- prefix. It is regenerated every launch,
	// which is also why editing it by hand does nothing.
	layoutDir := filepath.Join(p.ConfigDir, "zellij", "layouts")
	if err := os.MkdirAll(layoutDir, 0o755); err != nil {
		return err
	}
	layoutFile := filepath.Join(layoutDir, "bothy-"+profileName+".kdl")
	if err := os.WriteFile(layoutFile, []byte(kdl), 0o644); err != nil {
		return err
	}

	mux := cfg.Slots.Mux
	if mux == "" {
		mux = "zellij"
	}
	bin, err := exec.LookPath(mux)
	if err != nil {
		return fmt.Errorf("%s is not installed — run 'bothy install'", mux)
	}

	if err := os.Chdir(dir); err != nil {
		return err
	}
	return runInteractive(bin, "--layout", layoutFile)
}

// hopIntoContainer runs `bothy dev` again, inside the container.
func hopIntoContainer(container, dir, profile string) error {
	inner := fmt.Sprintf("cd %s && bothy dev --dir %s --profile %s",
		shellQuote(dir), shellQuote(dir), shellQuote(profile))

	if bin, err := exec.LookPath("toolbox"); err == nil {
		return runInteractive(bin, "run", "--container", container, "bash", "-lc", inner)
	}
	if bin, err := exec.LookPath("distrobox"); err == nil {
		return runInteractive(bin, "enter", container, "--", "bash", "-lc", inner)
	}
	return fmt.Errorf("container %q is configured but neither toolbox nor distrobox is installed\n"+
		"      clear it with: bothy config set workspace.container \"\"", container)
}

func cmdAttach(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	mux := cfg.Slots.Mux
	if mux == "" {
		mux = "zellij"
	}
	if !p.InContainer() {
		if container := cfg.ContainerFor(p); container != "" {
			if bin, err := exec.LookPath("toolbox"); err == nil {
				return runInteractive(bin, "run", "--container", container,
					"bash", "-lc", mux+" attach "+strings.Join(args, " "))
			}
		}
	}
	bin, err := exec.LookPath(mux)
	if err != nil {
		return fmt.Errorf("%s is not installed", mux)
	}
	return runInteractive(bin, append([]string{"attach"}, args...)...)
}

// nestedAgent reports whether this shell is already inside an agent session.
// Each agent advertises itself differently, so this is a list rather than a
// rule; an agent that sets nothing simply is not detected, which is why the
// check is a refusal to nest rather than a promise never to.
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
	cmd := exec.Command(name, args...)
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

// shellQuote makes a value safe to embed in the `bash -lc` string used for the
// container hop.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
