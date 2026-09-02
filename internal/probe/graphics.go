// Package probe answers questions about the running environment that cannot be
// decided from configuration alone.
package probe

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Graphics is the verdict on whether inline image previews will work, with the
// reason alongside so "previews are off" is never unexplained. Zellij before
// 0.45.1 could not pass the Kitty protocol through, and its mangled reply to
// Yazi's capability query was parsed as keystrokes; the workaround is gated on
// a version probe rather than made permanent (ADR-007).
type Graphics struct {
	Supported bool
	Reason    string
}

// terminalGraphics is what an emulator can draw, and by which protocol. The
// distinction matters because Zellij passes the *Kitty* protocol through and
// nothing else, so a terminal drawing images by a protocol of its own works
// directly and not through Zellij. Treating "draws images" and "speaks Kitty"
// as one fact told iTerm2 users their terminal could do neither.
type terminalGraphics struct {
	// kitty reports the Kitty graphics protocol, which zellij can carry.
	kitty bool
	// own names a protocol of the terminal's own, which zellij cannot.
	own string
}

// graphicsTerminals is what each emulator can do. A terminal that is absent
// draws no images at all as far as bothy knows, which is not the same as
// having been tested.
var graphicsTerminals = map[string]terminalGraphics{
	"ghostty": {kitty: true},
	"kitty":   {kitty: true},
	"wezterm": {kitty: true},

	// $TERM_PROGRAM, lowercased by platform.detectTerminal. iTerm2 has drawn
	// inline images since long before the Kitty protocol existed and still
	// uses its own.
	"iterm.app": {own: "iTerm2's own inline-image protocol"},
}

// CheckGraphics decides whether Yazi should draw images.
//
// muxBin is the multiplexer binary to interrogate (usually "zellij"); terminal
// is the detected emulator. An empty muxBin means no multiplexer is involved.
func CheckGraphics(terminal string, mux MuxGraphics) Graphics {
	g, known := graphicsTerminals[terminal]
	if !known {
		what := terminal
		if what == "" {
			what = "this terminal"
		}
		return Graphics{
			Reason: fmt.Sprintf("%s is not known to draw inline images; "+
				"previews would fall back to block art", what),
		}
	}

	// A protocol of the terminal's own works when Yazi is asked directly and
	// stops at the multiplexer, which carries Kitty graphics and no other.
	if g.own != "" {
		if mux.None {
			return Graphics{Supported: true,
				Reason: terminal + " draws images with " + g.own + ", and nothing is in the way"}
		}
		return Graphics{
			Reason: fmt.Sprintf("%s draws images with %s, which the multiplexer does not "+
				"carry — it passes the Kitty graphics protocol through and no other",
				terminal, g.own),
		}
	}

	if mux.None {
		return Graphics{Supported: true,
			Reason: terminal + " supports the Kitty graphics protocol, no multiplexer in the way"}
	}
	if !mux.Carries {
		return Graphics{Reason: mux.Reason}
	}
	return Graphics{Supported: true, Reason: mux.Reason + " and " + terminal + " speaks it"}
}

// MuxGraphics is the multiplexer's answer about the Kitty protocol, asked by
// the caller so this package need not know which multiplexer it is.
type MuxGraphics struct {
	// None is true when no multiplexer is in the way.
	None    bool
	Carries bool
	Reason  string
}

// ToolVersion runs `<bin> --version` and parses the result.
func ToolVersion(bin string) (Version, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return Version{}, err
	}
	return ParseVersion(string(out))
}

// Version is a lenient three-part version.
type Version struct{ Major, Minor, Patch int }

// versionPattern finds a dotted version anywhere in a string. Tools disagree
// about where the number goes: jq prints "jq-1.8.1", lazygit prints
// "commit=, build date=, ..., version=0.47.2, os=linux".
var versionPattern = regexp.MustCompile(`([0-9]+)\.([0-9]+)(?:\.([0-9]+))?`)

// ParseVersion pulls the first dotted version out of a tool's version output.
// Handles "zellij 0.45.1", "Yazi 26.5.6 (...)", "25.07.1", "jq-1.8.1",
// "version=0.47.2, os=linux" and distro-packaged "Ghostty 1.3.1-4.fc44".
func ParseVersion(s string) (Version, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("no version number in %q", strings.TrimSpace(s))
	}
	atoi := func(x string) int {
		n, _ := strconv.Atoi(x)
		return n
	}
	return Version{Major: atoi(m[1]), Minor: atoi(m[2]), Patch: atoi(m[3])}, nil
}

// Less reports whether v sorts before other.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// AtLeast reports whether a tool's version output meets a minimum.
func AtLeast(versionOutput string, min Version) (bool, Version, error) {
	v, err := ParseVersion(versionOutput)
	if err != nil {
		return false, Version{}, err
	}
	return !v.Less(min), v, nil
}
