// Package platform detects everything about the machine that changes what
// bothy installs or writes. Detection happens once and is passed around as an
// Info value, so a doctor report and the install that produced it agree about
// what machine they were looking at.
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
	DistroLike    string // os-release ID_LIKE, e.g. "debian" on Mint and Pop!_OS
	DistroVersion string // os-release VERSION_ID
	Immutable     bool   // rpm-ostree/bootc host: never install system packages

	Container     ContainerKind
	ContainerName string // the toolbox/distrobox name, for the `dev` host hop
	SharedHome    bool   // home is the host's home (Toolbx/Distrobox always)
	WSL           bool

	Term     string // $TERM
	Terminal string // best guess at the emulator: "ghostty", "wezterm", …

	Home     string
	LocalBin string // ~/.local/bin — where the bothy binary itself lives
	// Root is bothy's own tree, named outright rather than derived. Set from
	// $BOTHY_DIR, which SessionEnv exports, so a bothy run inside its own
	// workspace finds the tree it was launched from.
	Root      string
	ConfigDir string // $XDG_CONFIG_HOME or ~/.config
	DataDir   string // $XDG_DATA_HOME or ~/.local/share
}

// bothy's own tree. Everything bothy generates lives under BothyDir and
// nothing outside it, which is what makes `uninstall` a directory removal
// rather than a manifest replay. See PLAN.md §2.

// strandedSessionRoot recovers the tree for a shell inside a workspace that an
// older bothy started.
//
// Those sessions set XDG_DATA_HOME into the tree without naming the tree, so
// every command typed in them resolved one level deeper, and upgrading does
// not fix a session already running. The recovery is exact rather than a
// guess -- the tree is that directory's parent -- and BOTHY_SESSION is what
// makes reading it safe, since only bothy sets it. Removable once no
// pre-v0.3.1 session is plausibly still running.
func strandedSessionRoot() string {
	if os.Getenv("BOTHY_SESSION") == "" {
		return ""
	}
	// The shape those versions wrote, and only that shape: <tree>/share, where
	// the tree is always named bothy. An ordinary ~/.local/share also ends in
	// "share", and stripping that would send a healthy session to ~/.local.
	share := os.Getenv("XDG_DATA_HOME")
	tree := filepath.Dir(share)
	if filepath.Base(share) != "share" || filepath.Base(tree) != "bothy" {
		return ""
	}
	return tree
}

// BothyDir is the root of everything bothy owns. Root wins when set, because
// DataDir derives from XDG_DATA_HOME and SessionEnv points that variable
// *into* this tree -- so deriving bothy's own location from it would have every
// command typed in the shell pane look one level deeper.
func (i Info) BothyDir() string {
	if i.Root != "" {
		return i.Root
	}
	return filepath.Join(i.DataDir, "bothy")
}

// ConfigRoot holds the generated configs the tools are launched against.
// Note this is *not* the user's ~/.config — bothy never writes there.
func (i Info) ConfigRoot() string { return filepath.Join(i.BothyDir(), "config") }

// BinDir holds tools bothy had to supply because the system's were missing or
// too old. It goes on PATH for bothy's session only, so a supplied tool never
// shadows the user's everyday one.
func (i Info) BinDir() string { return filepath.Join(i.BothyDir(), "bin") }

// StateDir holds the manifest of what bothy installed.
func (i Info) StateDir() string { return filepath.Join(i.BothyDir(), "state") }

// CacheDir is where the tools bothy runs are pointed with XDG_CACHE_HOME, so
// that what they cache lands inside bothy's tree like everything else.
func (i Info) CacheDir() string { return filepath.Join(i.BothyDir(), "cache") }

// UserConfigDir is ~/.config/bothy: the user's own settings, palette and
// overrides. bothy reads it and writes only config.toml there.
func (i Info) UserConfigDir() string { return filepath.Join(i.ConfigDir, "bothy") }

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
	i.DataDir = xdg("XDG_DATA_HOME", home, ".local", "share")

	i.Root = os.Getenv("BOTHY_DIR")
	if i.Root == "" {
		i.Root = strandedSessionRoot()
	}

	i.DistroID, i.DistroLike, i.DistroVersion = osRelease()
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

// osRelease reads the three fields bothy dispatches on.
//
// ID_LIKE matters as much as ID: it is how Mint, Pop!_OS and other derivatives
// say "treat me as debian".
func osRelease() (id, like, version string) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", "", ""
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
		case "ID_LIKE":
			// ID_LIKE is a space-separated, most-similar-first list.
			like, _, _ = strings.Cut(v, " ")
		case "VERSION_ID":
			version = v
		}
	}
	return id, like, version
}

// detectContainer identifies Toolbx/Distrobox and recovers the container's
// name, which `bothy` on the host needs to hop back into it.
func detectContainer() (ContainerKind, string) { return detectContainerIn("/") }

// detectContainerIn is detectContainer against an arbitrary root, so the
// marker files can be tested.
func detectContainerIn(root string) (ContainerKind, string) {
	marker := func(name string) bool {
		_, err := os.Stat(filepath.Join(root, name))
		return err == nil
	}
	name := func() string { return containerNameIn(root) }

	if marker("run/.distrobox-enter-path") {
		return Distrobox, name()
	}
	if os.Getenv("DISTROBOX_ENTER_PATH") != "" || os.Getenv("CONTAINER_ID") != "" {
		if id := os.Getenv("CONTAINER_ID"); id != "" {
			return Distrobox, id
		}
		return Distrobox, name()
	}
	if marker("run/.containerenv") {
		// Toolbx marks itself, and that mark is the only evidence worth
		// reading. /run/.containerenv is written by podman for every container
		// it runs, with a generated name if none was given, so a name there is
		// not evidence of Toolbx.
		if marker("run/.toolboxenv") {
			return Toolbx, name()
		}
		return Generic, ""
	}
	if marker(".dockerenv") {
		return Generic, ""
	}
	return NotContainer, ""
}

// containerName parses name="…" out of /run/.containerenv.
func containerName() string { return containerNameIn("/") }

func containerNameIn(root string) string {
	f, err := os.Open(filepath.Join(root, "run/.containerenv"))
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
