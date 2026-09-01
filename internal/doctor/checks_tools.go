package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/advice"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/state"
	"github.com/bspeelm/bothy/internal/theme"
	"github.com/bspeelm/bothy/internal/tools"
)

// Checks about the tools themselves: where each came from, whether bothy can
// still reach it, and whether the agent and the palette are usable.

// checkToolProvenance reports where each tool came from and whether it is
// still good enough. This replaces revision 1's PATH-shadowing check, which
// mattered only because bothy installed into ~/.local/bin; a tool bothy
// supplies now lives in its own bin and is on PATH for its session alone.
//
// The failure it exists to catch: a system tool below the minimum with nothing
// supplied to cover it, which is a workspace that will misbehave in a way the
// individual tool never complains about.
func checkToolProvenance(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	names, err := tools.Required(env.Config.Slots.Mux, env.Config.Slots.Browser, env.Config.Extras)
	if err != nil {
		return warn("could not read the tool definitions", err.Error(), "")
	}

	var missing, supplied, system []string
	for _, name := range names {
		t, err := tools.Get(name)
		if err != nil {
			continue
		}
		own := filepath.Join(env.Platform.BinDir(), t.Binary)
		if fi, err := os.Stat(own); err == nil && fi.Mode()&0o111 != 0 {
			supplied = append(supplied, name)
			continue
		}
		d := tools.Resolve(t, tools.SystemLookPath, tools.SystemVersion)
		if d.Action == tools.Fetch {
			missing = append(missing, name+" ("+d.Reason+")")
			continue
		}
		system = append(system, name)
	}

	if len(missing) > 0 {
		return fail("some tools are missing or too old",
			strings.Join(missing, "; "),
			"run 'bothy install' to supply them into bothy's own bin")
	}
	return pass(fmt.Sprintf("%d tool(s) from your system, %d supplied by bothy",
		len(system), len(supplied)))
}

// checkToolsReachable catches tools recorded at one side of a container
// boundary and looked for at the other.
//
// Home is shared between a host and its toolboxes; PATH is not. An install run
// inside a container records yazi at /usr/bin/yazi, which simply does not
// exist on the host — so launching from the host opened a pane that died with
// "command not found: yazi", with nothing to suggest why. The launcher now
// hops back to where the tools are; this reports the case where it cannot.
func checkToolsReachable(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	m, err := state.Load(env.Platform.StateDir())
	if err != nil || len(m.Binaries) == 0 {
		return skip("nothing installed to check")
	}

	var unreachable []string
	for _, b := range m.Binaries {
		if b.Source != "system" || b.Path == "" {
			continue // bothy's own, inside its tree, reachable from anywhere
		}
		if _, err := os.Stat(b.Path); err != nil {
			unreachable = append(unreachable, b.Name+" ("+b.Path+")")
		}
	}
	if len(unreachable) == 0 {
		return pass("every recorded tool is reachable from here")
	}

	where := m.InstalledIn
	if where == "" {
		where = "the host"
	}
	return fail(fmt.Sprintf("%d tool(s) recorded at install time are not here", len(unreachable)),
		strings.Join(unreachable, "; ")+" — bothy was installed in "+where+
			", and home is shared between there and here but PATH is not",
		"run 'bothy install' from here, or let bothy hop back: bothy config set workspace.container "+m.InstalledIn)
}

func checkAgent(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	slot := env.Config.Slots.Agent
	if slot == "" || slot == "none" {
		return skip("no agent slot configured")
	}
	bin := map[string]string{"claude-code": "claude", "gemini-cli": "gemini"}[slot]
	if bin == "" {
		bin = slot
	}
	if _, err := env.lookPath(bin); err != nil {
		fix := "install it, or point the slot elsewhere: bothy config set slots.agent none"
		if a, err := advice.Get(slot); err == nil {
			fix = a.Command(env.Platform) +
				"\n         or turn the pane off: bothy config set slots.agent none"
		}
		return fail(bin+" is not on PATH", "the agent pane would open empty", fix)
	}
	return pass(bin + " is on PATH")
}

// checkEditor is the counterpart to checkAgent, and existed for neither until
// now: a configured editor that is not installed produced a pane running a
// missing command, an EDITOR pointing at nothing, and a doctor reporting
// everything fine.
//
// bothy supplies no editor. That is deliberate -- an editor is the most
// personal tool in the workspace and the one people already have -- but it
// means the editor slot is entirely a claim about the machine, and a claim
// about the machine is exactly what the doctor is for.
func checkEditor(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	slot := env.Config.Slots.Editor
	if slot == "" || slot == "none" {
		return skip("no editor slot configured")
	}
	bin := install.EditorBinary(slot)
	if _, err := env.lookPath(bin); err != nil {
		fix := "install it, or point the slot elsewhere: bothy config set slots.editor vim"
		if a, err := advice.Get(slot); err == nil {
			fix = a.Command(env.Platform) +
				"\n         or choose another: bothy config set slots.editor vim"
		}
		return fail(bin+" is not on PATH",
			"the editor pane would open empty, and EDITOR points at a command that does not exist",
			fix)
	}
	return pass(bin + " is on PATH")
}

func checkThemePalette(env Env) Result {
	path := env.Config.PalettePath(env.Platform)
	if path == "" {
		return skip("using the built-in open Dracula palette")
	}
	if _, err := os.Stat(path); err != nil {
		return fail("the configured palette file is missing",
			path+" does not exist",
			"point bothy at it again: bothy config set theme.palette <path>")
	}
	pal, err := theme.Load(path)
	if err != nil {
		return fail("the palette file could not be read", err.Error(),
			"fix the file, or fall back: bothy config set theme.palette \"\"")
	}
	return pass("palette " + pal.Name + " reads cleanly")
}
