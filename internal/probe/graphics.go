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

// terminalGraphics names the protocol Yazi uses on an emulator, which is not
// the same question as what the emulator is capable of. Yazi chooses by
// terminal rather than by capability, and its choice changes when a
// multiplexer is in the way -- so both cases are recorded. Treating "draws
// images" and "speaks Kitty" as one fact told iTerm2 users their terminal
// could do neither.
type terminalGraphics struct {
	// direct is the protocol Yazi uses with nothing in the way.
	direct string
	// carried is the protocol Yazi uses inside a multiplexer, and which the
	// multiplexer therefore has to pass through. Empty when a multiplexer
	// carries nothing Yazi would pick here.
	carried string
}

// kgp is the protocol zellij was gated on: see MinGraphics in internal/mux.
const kgp = "the Kitty graphics protocol"

// graphicsTerminals is what each emulator can do. A terminal that is absent
// draws no images at all as far as bothy knows, which is not the same as
// having been tested.
var graphicsTerminals = map[string]terminalGraphics{
	"ghostty": {direct: kgp, carried: kgp},
	"kitty":   {direct: kgp, carried: kgp},

	// WezTerm speaks Kitty, but Yazi does not use it there: its driver table
	// maps WezTerm to the iTerm2 protocol first and sixel second, and inside a
	// multiplexer it takes sixel. zellij carries sixel as well as Kitty, so
	// previews work -- by a different route than ghostty's, and the reason
	// shown to the user has to say which.
	"wezterm": {direct: "iTerm2's inline-image protocol", carried: "sixel"},

	// $TERM_PROGRAM, lowercased by platform.detectTerminal. iTerm2 has drawn
	// inline images since long before the Kitty protocol existed and still
	// uses its own, which no multiplexer here carries.
	"iterm.app": {direct: "iTerm2's own inline-image protocol"},
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

	if mux.None {
		return Graphics{Supported: true,
			Reason: terminal + " draws images with " + g.direct + ", and nothing is in the way"}
	}

	// Drawn by a protocol of the terminal's own: it works when Yazi is asked
	// directly and stops at the multiplexer.
	if g.carried == "" {
		return Graphics{
			Reason: fmt.Sprintf("%s draws images with %s, which the multiplexer does not "+
				"carry — it passes the Kitty graphics protocol and sixel through, and no other",
				terminal, g.direct),
		}
	}

	// The gate is the multiplexer's answer about Kitty, which is the protocol
	// its version mattered for. A sixel terminal is held to the same answer:
	// zellij has carried sixel for far longer, so this can only ever be
	// cautious, and bothy supplies a zellij that passes the gate anyway.
	if !mux.Carries {
		return Graphics{Reason: mux.Reason}
	}
	if g.carried == kgp {
		return Graphics{Supported: true, Reason: mux.Reason + " and " + terminal + " speaks it"}
	}
	return Graphics{Supported: true,
		Reason: fmt.Sprintf("%s, and Yazi draws on %s with %s, which it also carries",
			mux.Reason, terminal, g.carried)}
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
