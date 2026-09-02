// Package mux is the seam between bothy's one workspace idea and the program
// that arranges it. ADR-019 calls the multiplexer tier three: a renderer, not
// a template -- zellij takes a declarative layout file at launch, tmux builds
// the same arrangement with commands against a live session, and no amount of
// data makes those one thing.
//
// The shape was measured, not designed: a throwaway tmux backend tested five
// guesses about this boundary and corrected four (ADR-033, #64).
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

	// Dir is where this backend's generated config lives. Owned here because
	// the variable pointing the multiplexer at it is the backend's too.
	Dir(p platform.Info) string

	// SessionName turns a project directory into a name this backend can both
	// create and address -- per-backend because tmux accepts "." and ":" and
	// then cannot target the session, where zellij simply refuses them.
	SessionName(dir string) string

	// Open starts the workspace, or returns to it when it is already running,
	// replacing this process. Rendering and launching are one method because
	// they cannot be split: an interface returning layout text for the caller
	// to write out cannot express tmux, which splits a live session.
	Open(Request) error

	// Live is the sessions running. No sessions and no multiplexer are one
	// answer here: creating is the right move for both.
	Live(bin string, env []string) []string

	// SessionEnv is what this backend needs in the session. Every backend's
	// keys are unset before the chosen one's are set, because an inherited
	// value would otherwise survive into it.
	SessionEnv(p platform.Info) map[string]string

	// CurrentSession is the session this shell is inside, "" when it is not.
	CurrentSession() string

	// Panes counts the panes carrying a command, so the doctor can compare what
	// was built against what the profile described. A query rather than a file
	// read -- `list-panes` for tmux, `action dump-layout` for zellij, whose
	// session_info cache is a private path with a version directory in it.
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

// runReplacing hands stdio to the multiplexer; a non-zero exit is the
// multiplexer's own answer, not a bothy error.
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

// backends is every multiplexer bothy knows -- a registry rather than build
// tags (ADR-031): a backend compiles everywhere and is simply not selected.
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
