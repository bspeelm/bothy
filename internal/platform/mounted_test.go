package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// Mounted replaced a read of /proc/self/mounts, which on macOS failed and was
// reported as "nothing is mounted here" -- so a leaked mount was never cleared
// and the next connect mounted on top of it.
func TestAnOrdinaryDirectoryIsNotAMount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "remotes", "abbey")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if Mounted(dir) {
		t.Error("an empty directory reports as mounted, so connect would unmount it")
	}
	if Mounted(filepath.Join(dir, "missing")) {
		t.Error("a path that does not exist reports as mounted")
	}
}
