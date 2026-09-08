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
	"strconv"
	"strings"

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

	// PanesOf describes a session's panes, asked from outside it, false when the
	// backend cannot say. The tower needs to know which pane holds the agent,
	// and the position does not answer that.
	PanesOf(bin, session string, env []string) ([]PaneRef, bool)

	// Screen is a pane's rendered contents, addressed by pane so a session can
	// be watched without attaching: a second client would resize it to the
	// smaller of the two windows.
	Screen(bin, session, pane string, env []string) (string, error)

	// Expand toggles a pane between filling its tab and its place in the
	// layout. A watched pane is worth expanding because what can be read out of
	// it is exactly what it displays: a pane sharing its window two ways holds
	// a quarter of what the same pane holds alone.
	//
	// It changes what a session looks like and nothing about what runs in it.
	// The program inside receives SIGWINCH and redraws (ADR-048).
	Expand(bin, session, pane string, env []string) error

	// Send delivers a line a person typed to a pane, addressed so it reaches the
	// agent rather than whichever pane the session has focused. What bothy may
	// send is only what someone typed; it originates nothing (ADR-048).
	Send(bin, session, pane, line string, env []string) error
}

// PaneRef is a pane of a running session: enough to find the agent's and no
// more.
type PaneRef struct {
	ID      int    `json:"id"`
	Plugin  bool   `json:"is_plugin"`
	Command string `json:"pane_command"`
	Title   string `json:"title"`
	Dir     string `json:"pane_cwd"`
	Cols    int    `json:"pane_columns"`
	Exited  bool   `json:"exited"`
	// Fullscreen is read before it is changed. Expand toggles, so acting
	// without looking would collapse a pane that was already expanded.
	Fullscreen bool `json:"is_fullscreen"`
}

// Addr is how an action addresses this pane. Terminal and plugin ids each start
// at zero, so the kind is part of the address.
func (p PaneRef) Addr() string {
	if p.Plugin {
		return "plugin_" + strconv.Itoa(p.ID)
	}
	return "terminal_" + strconv.Itoa(p.ID)
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

// withPWD makes PWD name the directory the workspace is opening in.
//
// The variable is inherited from wherever bothy was typed, and chdir does not
// touch it. A tool that trusts it over the syscall -- yazi does -- then opens
// the wrong directory entirely, which is invisible until --dir or a remote
// mount makes the two differ. Measured: yazi in a workspace whose every
// process had the right cwd, showing the directory bothy was launched from.
func withPWD(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "PWD=") {
			out = append(out, kv)
		}
	}
	return append(out, "PWD="+dir)
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
