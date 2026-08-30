// Package probe answers questions about the running environment that cannot be
// decided from configuration alone.
package probe

import (
	"fmt"
	"os/exec"
	"strings"
)

// Graphics is the verdict on whether inline image previews will actually work.
//
// This exists because the origin setup turned image previews off, and that was
// the right call at the time and is the wrong call now. Zellij 0.42 could not
// pass the Kitty graphics protocol through: previews fell back to chafa block
// art, and Zellij's mangled reply to Yazi's capability query was parsed as
// keystrokes, firing a phantom "Find next" on every preview. Zellij 0.45.0
// implemented Kitty graphics and 0.45.1 fixed image sizing and stopped
// advertising Sixel support the terminal does not have.
//
// So the workaround is gated rather than deleted or made permanent — see
// docs/decisions.md ADR-007. Reason is carried alongside the verdict because
// "previews are off" without a reason is exactly the kind of unexplained
// behaviour this project exists to remove.
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

// ParseVersion pulls the first dotted number out of a tool's version output.
// The shapes seen in practice are "zellij 0.45.1", "Yazi 26.5.6 (...)",
// "25.07.1" and distro-packaged "Ghostty 1.3.1-4.fc44" — hence taking each
// segment's leading digits and stopping at the first segment that has none or
// carries a suffix.
func ParseVersion(s string) (Version, error) {
	for _, field := range strings.Fields(s) {
		field = strings.TrimPrefix(strings.Trim(field, "(),"), "v")
		nums, ok := leadingNumbers(field)
		if !ok {
			continue
		}
		v := Version{Major: nums[0], Minor: nums[1]}
		if len(nums) > 2 {
			v.Patch = nums[2]
		}
		return v, nil
	}
	return Version{}, fmt.Errorf("no version number in %q", strings.TrimSpace(s))
}

// leadingNumbers turns "1.3.1-4.fc44" into [1 3 1]: each dot-separated segment
// contributes its leading digits, and a segment with a suffix ends the version.
func leadingNumbers(s string) ([]int, bool) {
	var out []int
	for _, part := range strings.Split(s, ".") {
		n, i := 0, 0
		for i < len(part) && part[i] >= '0' && part[i] <= '9' {
			n = n*10 + int(part[i]-'0')
			i++
		}
		if i == 0 {
			break // no digits at all: the version stopped before this segment
		}
		out = append(out, n)
		if i < len(part) || len(out) == 3 {
			break // trailing junk, or we have all three parts
		}
	}
	return out, len(out) >= 2
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
