package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/advice"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/probe"
)

// Checks about the terminal, the multiplexer, and getting a file opened.

// checkTerminalCapability reports where bothy will run. A terminal that cannot
// draw images is not an error; it is the reason previews will be off.
func checkTerminalCapability(env Env) Result {
	term := env.Platform.Terminal
	if term == "" {
		term = "an unrecognised terminal"
	}
	g := probe.CheckGraphics(env.Platform.Terminal, probe.MuxGraphics{None: true})
	if g.Supported {
		return pass("running in " + term + ", which can draw images")
	}
	// Not a failure: bothy opens a Ghostty window instead — unless there is
	// no ghostty to open.
	if _, err := exec.LookPath("ghostty"); err != nil {
		fix := "install a terminal that can draw images, or accept block-art previews"
		if a, err := advice.Get("ghostty"); err == nil {
			fix = a.Command(env.Platform)
			if w := a.Warnings(env.Platform); w != "" {
				fix += "\n         " + w
			}
		}
		return warn("this terminal cannot draw inline images, and ghostty is not installed",
			g.Reason, fix)
	}
	return pass("this terminal cannot draw images, so bothy will open a Ghostty window — " + g.Reason)
}

func checkMuxConfig(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if env.Mux == nil {
		return skip("no multiplexer backend for the configured slot")
	}
	name := env.Mux.Name()
	bin, err := env.lookPath(name)
	if err != nil {
		return fail(name+" is not installed", "", "run 'bothy install'")
	}
	detail, err := env.Mux.CheckConfig(bin, env.ToolEnv)
	if errors.Is(err, mux.ErrUnsupported) {
		return skip(name + " has no configuration check")
	}
	if err != nil {
		return fail(name+" rejects its configuration", detail,
			"run 'bothy install' to regenerate bothy's "+name+" config")
	}
	return pass(name + " accepts its config")
}

// checkTerminfo catches the container trap: the toolbox image has no
// xterm-ghostty entry, so entering it leaves the terminal degraded.
func checkTerminfo(env Env) Result {
	term := env.Platform.Term
	if term == "" {
		return warn("$TERM is not set", "", "")
	}
	// Without infocmp the question cannot be answered; reporting the entry
	// missing would be a different problem with a different fix.
	if _, err := exec.LookPath("infocmp"); err != nil {
		return skip("infocmp is not installed, so terminfo cannot be checked")
	}
	if err := exec.Command("infocmp", term).Run(); err != nil {
		return fail("no terminfo entry for $TERM ("+term+")",
			"the terminal will fall back to a degraded mode", terminfoFix(env, term))
	}
	return pass("terminfo entry for " + term + " is present")
}

// terminfoFix names a way to get the entry that works where it is offered.
// The host copy is only there under Toolbx and Distrobox, which mount the host
// root at /run/host. Elsewhere the portable answer is to carry the compiled
// entry over from a machine that has it: no package by that name exists.
func terminfoFix(env Env, term string) string {
	sub := term[:1]
	if env.Platform.SharedHome {
		return "copy it in from the host: mkdir -p ~/.terminfo/" + sub +
			" && cp /run/host/usr/share/terminfo/" + sub + "/" + term + " ~/.terminfo/" + sub + "/"
	}
	return "carry it over from a machine that has it: 'infocmp -x " + term +
		" > " + term + ".src' there, then 'tic -x " + term + ".src' here"
}

// checkWatermarkImage exists because Ghostty says nothing about a
// background-image that is not there. It simply draws nothing, which looks
// exactly like "the opacity is too low" and sends you tuning a setting that
// was never the problem.
func checkWatermarkImage(env Env) Result {
	path := config.Expand(env.Config.Workspace.BackgroundImage, env.Platform.Home)
	if path == "" {
		return skip("no watermark configured")
	}
	if env.Config.Slots.Terminal != "ghostty" {
		return skip("the watermark needs ghostty")
	}
	fi, err := os.Stat(path)
	switch {
	case err != nil:
		return fail("the watermark image is missing", path+" does not exist",
			"point workspace.watermark at an image, or clear it")
	case fi.Size() == 0:
		return fail("the watermark image is empty", path+" is zero bytes",
			"replace it, or clear workspace.watermark")
	}
	return pass("watermark image is in place")
}

func checkOpener(env Env) Result {
	// The binary the generated config actually names, so the check and the
	// config cannot disagree about which one matters on this machine.
	want := install.OpenerBinary(env.Platform)
	if _, err := env.lookPath(want); err != nil {
		if env.Platform.OS == "darwin" {
			// `open` is part of macOS. Its absence means something stranger
			// than a missing package, so there is nothing to advise
			// installing.
			return warn(want+" is not on PATH",
				"pressing Enter on a file in yazi will not open it", "")
		}
		if env.Platform.SharedHome {
			return fail(want+" is not on PATH",
				"pressing Enter on a file in yazi will report 'No such file or directory'",
				"run 'bothy install' to place the host-forwarding shim in bothy's bin")
		}
		if env.Platform.InContainer() {
			// No host session to forward to, so there is no shim to suggest.
			return warn(want+" is not on PATH",
				"this container has no desktop to open files with, and no host to forward to",
				"")
		}
		return warn(want+" is not on PATH", "", "install xdg-utils")
	}
	return pass(want + " resolves")
}

// checkXdgOpenShimGuard: home is shared between host and container, so the
// shim must keep its containerenv guard or the host execs itself forever.
func checkXdgOpenShimGuard(env Env) Result {
	shim := filepath.Join(env.Platform.BinDir(), "xdg-open")
	b, err := os.ReadFile(shim)
	if err != nil {
		return skip("no xdg-open shim installed")
	}
	body := string(b)
	if !strings.Contains(body, "flatpak-spawn") {
		return skip("~/.local/bin/xdg-open is not bothy's shim")
	}
	if !strings.Contains(body, "/run/.containerenv") && !strings.Contains(body, "/run/.dockerenv") {
		return fail("the xdg-open shim has no container guard",
			"home is shared with the host, so on the host this shim would exec itself forever",
			"run 'bothy install' to rewrite it")
	}
	return pass("xdg-open shim is guarded against host recursion")
}

// checkOneClientPerSession reports a session more than one terminal is looking
// at. zellij sizes a session to its smallest client, so a second one caps the
// workspace at the smaller window -- and the symptom is corrupted output,
// which reads as the agent's bug rather than a geometry one. The launcher
// refuses to create this (#205); `bothy attach` still can, on purpose.
func checkOneClientPerSession(env Env) Result {
	if env.Mux == nil {
		return skip("no multiplexer backend for the configured slot")
	}
	session := env.Mux.CurrentSession()
	if session == "" {
		return skip("not inside a multiplexer session")
	}
	n, ok := env.Mux.Clients(env.MuxBin, env.ToolEnv, session, env.Mux.Live(env.MuxBin, env.ToolEnv))
	if !ok {
		return skip("the multiplexer could not say who is attached")
	}
	if n > 1 {
		return warn(fmt.Sprintf("%s has %d terminals attached", session, n),
			"the session is sized to the smallest of them, so the workspace is capped at that window",
			"close the others, or resize them to match this one")
	}
	return pass("one terminal attached to " + session)
}
