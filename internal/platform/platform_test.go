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
	id, like, ver := osRelease()
	if id == "" {
		t.Skip("no /etc/os-release on this machine")
	}
	if id[0] == '"' || (ver != "" && ver[0] == '"') {
		t.Errorf("quotes not stripped: ID=%q VERSION_ID=%q", id, ver)
	}
	if strings.ContainsAny(like, `" `) {
		t.Errorf("ID_LIKE should be one bare word, got %q", like)
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
//
// Keyed on /run/.toolboxenv rather than /run/.containerenv: the latter is
// podman's file and is present in any podman container, where there is no name
// worth having. An earlier version of this test skipped on .containerenv and
// then asserted SharedHome -- a precondition every named podman container
// satisfies, so it asserted the misdetection was correct.
func TestContainerNameParsed(t *testing.T) {
	if _, err := os.Stat("/run/.toolboxenv"); err != nil {
		t.Skip("not in a toolbox")
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

// What each combination of marker files means. This is the whole of container
// detection, and until detectContainerIn took a root none of it could be
// tested without being inside the thing under test.
func TestDetectContainerFromMarkers(t *testing.T) {
	// Set here so the ambient environment of whatever runs the tests -- which
	// may itself be a distrobox -- cannot reach the cases below.
	t.Setenv("DISTROBOX_ENTER_PATH", "")
	t.Setenv("CONTAINER_ID", "")

	const podmanEnv = "engine=\"podman-5.8.4\"\nname=\"bothy-test\"\nrootless=1\n"

	for _, tc := range []struct {
		name     string
		files    map[string]string
		wantKind ContainerKind
		wantName string
	}{
		{"bare host", nil, NotContainer, ""},
		{"docker", map[string]string{".dockerenv": ""}, Generic, ""},
		{
			// The case that was wrong: podman names every container it runs,
			// so a name here is evidence of podman and of nothing else.
			"named podman container is not a toolbox",
			map[string]string{"run/.containerenv": podmanEnv},
			Generic, "",
		},
		{
			"toolbox",
			map[string]string{"run/.containerenv": podmanEnv, "run/.toolboxenv": ""},
			Toolbx, "bothy-test",
		},
		{
			"distrobox",
			map[string]string{"run/.distrobox-enter-path": "", "run/.containerenv": podmanEnv},
			Distrobox, "bothy-test",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, body := range tc.files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			kind, name := detectContainerIn(root)
			if kind != tc.wantKind || name != tc.wantName {
				t.Errorf("detectContainerIn() = (%q, %q), want (%q, %q)",
					kind, name, tc.wantKind, tc.wantName)
			}
		})
	}
}
