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

// Graphics is the verdict on whether inline image previews will actually work.
//
// Zellij below 0.45.0 cannot pass the Kitty graphics protocol through:
// previews fall back to chafa block art, and Zellij's mangled reply to Yazi's
// capability query is parsed as keystrokes, firing a phantom "Find next" on
// every preview. 0.45.1 fixed image sizing and stopped advertising Sixel the
// terminal does not have. Hence a version gate rather than a permanent
// workaround — ADR-007.
//
// Reason travels with the verdict: "previews are off" without one is the kind
// of unexplained behaviour the doctor exists to remove.
type Graphics struct {
	Supported bool
	Reason    string
}

// MinZellijGraphics is the first Zellij release that renders images correctly.
// 0.45.0 added the protocol; 0.45.1 fixed image sizing on startup.
var MinZellijGraphics = Version{0, 45, 1}

// graphicsTerminals are the emulators known to implement the Kitty graphics
// protocol. Anything else falls back to block art, which is not worth the
// phantom-keypress risk.
var graphicsTerminals = map[string]bool{
	"ghostty": true,
	"kitty":   true,
	"wezterm": true,
}

// CheckGraphics decides whether Yazi should draw images.
//
// muxBin is the multiplexer binary to interrogate (usually "zellij"); terminal
// is the detected emulator. An empty muxBin means no multiplexer is involved.
func CheckGraphics(muxBin, terminal string) Graphics {
	if !graphicsTerminals[terminal] {
		what := terminal
		if what == "" {
			what = "this terminal"
		}
		return Graphics{
			Reason: fmt.Sprintf("%s is not known to support the Kitty graphics protocol; "+
				"previews would fall back to block art", what),
		}
	}

	if muxBin == "" {
		return Graphics{Supported: true, Reason: terminal + " supports the Kitty graphics protocol, no multiplexer in the way"}
	}

	v, err := ZellijVersion(muxBin)
	if err != nil {
		return Graphics{
			Reason: fmt.Sprintf("could not determine the %s version (%v), so assuming it cannot pass "+
				"the Kitty graphics protocol through", muxBin, err),
		}
	}
	if v.Less(MinZellijGraphics) {
		return Graphics{
			Reason: fmt.Sprintf("zellij %s cannot pass the Kitty graphics protocol through; "+
				"%s or newer can", v, MinZellijGraphics),
		}
	}
	return Graphics{
		Supported: true,
		Reason: fmt.Sprintf("zellij %s implements the Kitty graphics protocol and %s speaks it",
			v, terminal),
	}
}

// ZellijVersion runs `zellij --version` and parses the result.
func ZellijVersion(bin string) (Version, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return Version{}, err
	}
	return ParseVersion(string(out))
}

// Version is a lenient three-part version.
type Version struct{ Major, Minor, Patch int }

// versionPattern finds a dotted version anywhere in a string.
//
// Scanning for the pattern rather than splitting on whitespace, because tools
// do not agree on where the number goes: jq prints "jq-1.8.1" with the number
// glued to the name, lazygit prints "commit=, build date=, ...,
// version=0.47.2, os=linux". Requiring a token that *starts* with a digit
// finds neither, and a tool with no detectable version gets replaced.
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
