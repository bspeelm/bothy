// Package mux arranges the workspace. ADR-019 makes the multiplexer a
// renderer rather than a template: zellij takes a layout file at launch, tmux
// runs commands against a live session.
//
// The interface below was shaped by a throwaway tmux backend (#64).
package mux

import (
	"errors"
	"os"
	"os/exec"

	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/platform"
)

// Backend is one multiplexer.
type Backend interface {
	// Name is the provider name, as slots.mux spells it.
	Name() string

	// Dir is where the backend's generated config lives. Owned here because
	// SessionEnv points the multiplexer at it.
	Dir(p platform.Info) string

	// SessionName turns a project directory into a name the backend can create
	// and address. tmux accepts "." and ":" and then cannot target the
	// session; zellij refuses them.
	SessionName(dir string) string

	// Preview is what the backend would build, as text. `bothy layout` prints
	// it and a doctor check runs it for the error alone. Separate from Open
	// because tmux builds by running commands: there is no text to hand back.
	Preview(p layout.Profile, cmds layout.Commands) (string, error)

	// Open starts the workspace or returns to a running one, replacing this
	// process. Rendering and launching are one call: tmux splits a live
	// session, so there is no layout text to hand back first.
	Open(Request) error

	// Live is the sessions running. No sessions and no multiplexer give the
	// same answer: create.
	Live(bin string, env []string) []string

	// SessionEnv is what the backend needs in the session. Every backend's keys
	// are unset before the chosen one's are set: an inherited value survives
	// into the session otherwise.
	SessionEnv(p platform.Info) map[string]string

	// Clients counts who is looking at a session, false when it could not be
	// found out. Asked before a window is opened or a container entered, so
	// the refusal lands where the person typing can read it.
	Clients(bin string, env []string, session string, live []string) (int, bool)

	// Stopped is the sessions kept for resurrection, and Discard removes one.
	// Nothing removes them on its own, so they pile up until someone asks.
	Stopped(bin string, env []string) []string
	Discard(bin string, env []string, session string) error

	// Kill ends a running session and leaves nothing behind, which is what
	// quitting from inside does. Discard refuses a live session; this is the
	// one that means to end it.
	Kill(bin string, env []string, session string) error

	// CurrentSession is the session this shell is inside, "" when it is not.
	CurrentSession() string

	// Graphics reports whether this multiplexer carries the Kitty graphics
	// protocol, with the reason when it does not. It sits between the terminal
	// and the file browser, so it decides whether previews survive.
	Graphics(bin string) (bool, string)

	// CheckConfig asks the multiplexer whether it accepts its own config.
	// Returns ErrUnsupported for a backend with no such command.
	CheckConfig(bin string, env []string) (string, error)

	// Panes counts the panes carrying a command, for comparison against the
	// profile. A query, not a file read: `list-panes` for tmux, `action
	// dump-layout` for zellij, whose session_info cache is private.
	Panes(bin, session string, env []string) (int, bool)
}

// Request is everything Open needs that is not the backend's own business.
type Request struct {
	Platform platform.Info
	Bin      string
	Session  string
	Dir      string
	Profile  layout.Profile
	Commands layout.Commands
	Env      []string
	Live     []string
}

// runReplacing hands stdio to the multiplexer. A non-zero exit is the
// multiplexer's status, not a bothy error.
func runReplacing(env []string, name string, args ...string) error {
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

// ErrUnsupported is what a backend returns for a question it cannot answer.
// The caller reports it as a skip, not a failure.
var ErrUnsupported = errors.New("mux: the backend does not answer that")

// backends is every multiplexer bothy knows. A registry, not build tags
// (ADR-031): a backend compiles everywhere and is not selected.
var backends = []Backend{Zellij{}, None{}}

// For returns the backend filling the mux slot, false when bothy has no
// implementation for the name.
func For(name string) (Backend, bool) {
	for _, b := range backends {
		if b.Name() == name {
			return b, true
		}
	}
	return nil, false
}

// All is every backend.
func All() []Backend { return backends }
