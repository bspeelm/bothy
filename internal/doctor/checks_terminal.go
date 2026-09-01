package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/advice"
	"github.com/bspeelm/bothy/internal/probe"
)

// Checks about the terminal, the multiplexer, and getting a file opened --
// the parts of the workspace that depend on what is outside bothy's tree.

// checkTerminalCapability reports where bothy will run. A terminal that cannot
// draw images is not an error — it is a reason previews will be off, and
// saying so beats letting someone wonder why their config "did not work".
func checkTerminalCapability(env Env) Result {
	term := env.Platform.Terminal
	if term == "" {
		term = "an unrecognised terminal"
	}
	g := probe.CheckGraphics("", env.Platform.Terminal)
	if g.Supported {
		return pass("running in " + term + ", which can draw images")
	}
	// Not a failure: bothy opens a Ghostty window when the current terminal
	// cannot draw, so this is a statement of what will happen rather than a
	// problem to fix. It becomes one only if there is no Ghostty to open.
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

func checkZellijConfig(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if env.Config.Slots.Mux != "zellij" {
		return skip("mux slot is not zellij")
	}
	bin, err := env.lookPath("zellij")
	if err != nil {
		return fail("zellij is not installed", "", "run 'bothy install'")
	}
	out, err := env.tool(bin, "setup", "--check").CombinedOutput()
	if err != nil {
		return fail("zellij rejects its configuration",
			strings.TrimSpace(string(out)),
			"run 'bothy install' to regenerate bothy's zellij config")
	}
	return pass("zellij accepts its config")
}

// checkTerminfo catches the container trap: the toolbox image has no
// xterm-ghostty entry, so entering it greets you with a terminfo error and
// leaves the terminal degraded.
func checkTerminfo(env Env) Result {
	term := env.Platform.Term
	if term == "" {
		return warn("$TERM is not set", "", "")
	}
	// Without infocmp there is no way to answer the question. Saying so is
	// better than reporting the entry missing, which is a different problem
	// with a different fix.
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
//
// The host copy is only there under Toolbx and Distrobox, which mount the host
// root at /run/host. Everywhere else -- a plain container, or any machine whose
// distribution has no package for this terminal, which on Ubuntu is every
// machine -- the portable answer is to carry the compiled entry over from
// somewhere that has it: "install the terminfo entry for xterm-ghostty" names
// no package that exists.
func terminfoFix(env Env, term string) string {
	sub := term[:1]
	if env.Platform.SharedHome {
		return "copy it in from the host: mkdir -p ~/.terminfo/" + sub +
			" && cp /run/host/usr/share/terminfo/" + sub + "/" + term + " ~/.terminfo/" + sub + "/"
	}
	return "carry it over from a machine that has it: 'infocmp -x " + term +
		" > " + term + ".src' there, then 'tic -x " + term + ".src' here"
}

func checkOpener(env Env) Result {
	if _, err := env.lookPath("xdg-open"); err != nil {
		if env.Platform.SharedHome {
			return fail("xdg-open is not on PATH",
				"pressing Enter on a file in yazi will report 'No such file or directory'",
				"run 'bothy install' to place the host-forwarding shim in bothy's bin")
		}
		if env.Platform.InContainer() {
			// No host session to forward to, so there is no shim to suggest
			// and nothing bothy can do about it.
			return warn("xdg-open is not on PATH",
				"this container has no desktop to open files with, and no host to forward to",
				"")
		}
		return warn("xdg-open is not on PATH", "", "install xdg-utils")
	}
	return pass("xdg-open resolves")
}

// checkXdgOpenShimGuard is the check that stops a fix from becoming a worse
// bug. Home is shared between host and container, so the shim in ~/.local/bin
// is on the host's PATH too — without its containerenv guard, the host execs
// itself forever.
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

func checkWatermarkImage(env Env) Result {
	if !env.Config.Workspace.Watermark {
		return skip("watermark is off")
	}
	if env.Config.Slots.Terminal != "ghostty" {
		return skip("watermark needs ghostty")
	}
	path := filepath.Join(env.Platform.ConfigRoot(), "watermark.png")
	fi, err := os.Stat(path)
	if err != nil {
		return fail("the watermark image is missing",
			path+" does not exist; ghostty will silently draw nothing",
			"run 'bothy install' to write it")
	}
	if fi.Size() == 0 {
		return fail("the watermark image is empty", path+" is zero bytes",
			"run 'bothy install' to rewrite it")
	}
	return pass("watermark image is in place")
}
