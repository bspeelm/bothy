package install

import (
	"os"
	"path/filepath"
	"testing"
)

// #33. runningBinary resolves symlinks and LocalBin did not, so the two named
// one directory in two ways and removeBinary returned early. Found by the
// macOS job, where a temporary home under /var is reached through a symlink to
// /private/var -- and it would find any home reached through one.
func TestUninstallRemovesTheBinaryThroughASymlink(t *testing.T) {
	real := t.TempDir()
	bin := filepath.Join(real, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}

	// A second name for the same home, the way /var is a second name for
	// /private/var.
	link := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if !sameDir(filepath.Join(real, ".local", "bin"), filepath.Join(link, ".local", "bin")) {
		t.Error("the same directory reached two ways did not compare equal, " +
			"so uninstall would report success and leave the binary behind")
	}
}
