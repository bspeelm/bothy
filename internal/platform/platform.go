// Package platform detects everything about the machine that changes what
// bothy installs or writes.
//
// Detection is done once and passed around as an Info value rather than
// re-probed, so a doctor report and the install that produced it always agree
// about what machine they were looking at.
package platform

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ContainerKind distinguishes the container runtimes that change bothy's
// behaviour. Toolbx and Distrobox both share the host's home directory, which
// is what makes the xdg-open recursion trap and the shared-.bashrc trap real.
type ContainerKind string

const (
	NotContainer ContainerKind = ""
	Toolbx       ContainerKind = "toolbx"
	Distrobox    ContainerKind = "distrobox"
	Generic      ContainerKind = "container"
)

// Info is the detected machine.
type Info struct {
	OS   string // "linux", "darwin"
	Arch string // "x86_64", "aarch64" — release-asset spelling, not Go's

	DistroID      string // os-release ID, e.g. "fedora", "ubuntu", "arch"
	DistroVersion string // os-release VERSION_ID
	Immutable     bool   // rpm-ostree/bootc host: never install system packages

	Container     ContainerKind
	ContainerName string // the toolbox/distrobox name, for the `dev` host hop
	SharedHome    bool   // home is the host's home (Toolbx/Distrobox always)
	WSL           bool

	Term     string // $TERM
	Terminal string // best guess at the emulator: "ghostty", "wezterm", …

	Home      string
	LocalBin  string // ~/.local/bin
	ConfigDir string // $XDG_CONFIG_HOME or ~/.config
	StateDir  string // $XDG_STATE_HOME or ~/.local/state
}

// Detect probes the machine. It never fails: an undetectable field is left
// zero, and the doctor is what turns a missing field into a visible problem.
func Detect() Info {
	home, _ := os.UserHomeDir()

	i := Info{
		OS:       runtime.GOOS,
		Arch:     assetArch(),
		Term:     os.Getenv("TERM"),
		Terminal: detectTerminal(),
		Home:     home,
	}
	i.LocalBin = filepath.Join(home, ".local", "bin")
	i.ConfigDir = xdg("XDG_CONFIG_HOME", home, ".config")
	i.StateDir = xdg("XDG_STATE_HOME", home, ".local", "state")

	i.DistroID, i.DistroVersion = osRelease()
	i.Container, i.ContainerName = detectContainer()
	i.SharedHome = i.Container == Toolbx || i.Container == Distrobox
	i.WSL = detectWSL()
	i.Immutable = detectImmutable(i.Container)

	return i
}

// InContainer reports whether bothy is running inside a container that shares
// the host's home. This is the condition for the flatpak-spawn opener and the
// guarded xdg-open shim.
func (i Info) InContainer() bool { return i.Container != NotContainer }

// assetArch returns the architecture the way release assets spell it, which is
// uname's spelling rather than Go's ("x86_64", not "amd64"). Getting this wrong
// means every download 404s, so it is worth not deriving it twice.
func assetArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func xdg(env, home string, fallback ...string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return filepath.Join(append([]string{home}, fallback...)...)
}

// osRelease reads ID and VERSION_ID from /etc/os-release.
func osRelease() (id, version string) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		switch strings.TrimSpace(k) {
		case "ID":
			id = v
		case "VERSION_ID":
			version = v
		}
	}
	return id, version
}

// detectContainer identifies Toolbx/Distrobox and recovers the container's
// name. The name matters: `dev` run from the host has to hop back into *this*
// container, and hardcoding a name is exactly the machine-specific detail that
// stopped the origin setup from being portable.
func detectContainer() (ContainerKind, string) {
	if _, err := os.Stat("/run/.distrobox-enter-path"); err == nil {
		return Distrobox, containerName()
	}
	if os.Getenv("DISTROBOX_ENTER_PATH") != "" || os.Getenv("CONTAINER_ID") != "" {
		if name := os.Getenv("CONTAINER_ID"); name != "" {
			return Distrobox, name
		}
		return Distrobox, containerName()
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		if _, err := os.Stat("/run/.toolboxenv"); err == nil {
			return Toolbx, containerName()
		}
		// A Toolbx container always has a name in .containerenv; a bare podman
		// container usually does not.
		if n := containerName(); n != "" {
			return Toolbx, n
		}
		return Generic, ""
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return Generic, ""
	}
	return NotContainer, ""
}

// containerName parses name="…" out of /run/.containerenv.
func containerName() string {
	f, err := os.Open("/run/.containerenv")
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok && strings.TrimSpace(k) == "name" {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

func detectWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// detectImmutable reports whether the *host* is an image-based system where
// system packages need rpm-ostree and a reboot. Inside a container the question
// is about the host, so we ask the host.
func detectImmutable(kind ContainerKind) bool {
	if kind == NotContainer {
		if _, err := os.Stat("/run/ostree-booted"); err == nil {
			return true
		}
		return false
	}
	// /run/host is the host root, mounted by Toolbx and Distrobox.
	if _, err := os.Stat(filepath.Join("/run/host", "run", "ostree-booted")); err == nil {
		return true
	}
	if _, err := exec.LookPath("flatpak-spawn"); err == nil {
		out, err := exec.Command("flatpak-spawn", "--host", "test", "-e", "/run/ostree-booted").CombinedOutput()
		if err == nil && len(out) == 0 {
			return true
		}
	}
	return false
}

// detectTerminal guesses the emulator. Ghostty is the one bothy cares about,
// because it is why inline image previews are possible at all.
func detectTerminal() string {
	if v := os.Getenv("TERM_PROGRAM"); v != "" {
		return strings.ToLower(v)
	}
	term := os.Getenv("TERM")
	switch {
	case strings.Contains(term, "ghostty"):
		return "ghostty"
	case strings.Contains(term, "kitty"):
		return "kitty"
	case strings.Contains(term, "wezterm"):
		return "wezterm"
	case strings.Contains(term, "alacritty"):
		return "alacritty"
	}
	if os.Getenv("WEZTERM_EXECUTABLE") != "" {
		return "wezterm"
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return "kitty"
	}
	return ""
}
