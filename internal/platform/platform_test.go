package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetArchUsesUnameSpelling(t *testing.T) {
	// Release assets are named x86_64/aarch64, not amd64/arm64. Getting this
	// wrong 404s every download, so pin the mapping.
	if got := assetArch(); got != "x86_64" && got != "aarch64" {
		t.Errorf("assetArch() = %q, want a uname-style arch", got)
	}
}

func TestOsReleaseParsesQuotedValues(t *testing.T) {
	id, ver := osRelease()
	if id == "" {
		t.Skip("no /etc/os-release on this machine")
	}
	if id[0] == '"' || (ver != "" && ver[0] == '"') {
		t.Errorf("quotes not stripped: ID=%q VERSION_ID=%q", id, ver)
	}
}

func TestDetectFillsPaths(t *testing.T) {
	i := Detect()
	if i.Home == "" {
		t.Fatal("Home is empty")
	}
	if i.LocalBin != filepath.Join(i.Home, ".local", "bin") {
		t.Errorf("LocalBin = %q", i.LocalBin)
	}
	if i.ConfigDir == "" || i.DataDir == "" {
		t.Errorf("ConfigDir=%q DataDir=%q", i.ConfigDir, i.DataDir)
	}
	// Everything bothy generates must sit under one directory — that is the
	// property `uninstall` relies on (ADR-009).
	for name, dir := range map[string]string{
		"ConfigRoot": i.ConfigRoot(), "BinDir": i.BinDir(), "StateDir": i.StateDir(),
	} {
		if !strings.HasPrefix(dir, i.BothyDir()) {
			t.Errorf("%s() = %q, which is outside BothyDir() %q", name, dir, i.BothyDir())
		}
	}
	// The user's own settings live outside that tree, so uninstall keeps them.
	if strings.HasPrefix(i.UserConfigDir(), i.BothyDir()) {
		t.Error("UserConfigDir() is inside BothyDir(); uninstall would delete the user's settings")
	}
}

func TestXDGHonoursEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/cfg")
	if got := xdg("XDG_CONFIG_HOME", "/home/x", ".config"); got != "/custom/cfg" {
		t.Errorf("xdg() = %q, want /custom/cfg", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := xdg("XDG_CONFIG_HOME", "/home/x", ".config"); got != "/home/x/.config" {
		t.Errorf("xdg() = %q, want the fallback", got)
	}
}

// The container name is what lets `dev` hop back into the right container from
// the host. Losing it is how a setup ends up hardcoding someone's box name.
func TestContainerNameParsed(t *testing.T) {
	if _, err := os.Stat("/run/.containerenv"); err != nil {
		t.Skip("not in a container")
	}
	i := Detect()
	if !i.InContainer() {
		t.Fatal("InContainer() = false inside a container")
	}
	if i.ContainerName == "" {
		t.Error("ContainerName is empty; `dev` would not know which container to enter")
	}
	if !i.SharedHome {
		t.Error("SharedHome = false; Toolbx/Distrobox always share the host home")
	}
}
