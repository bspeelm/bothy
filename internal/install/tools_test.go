package install

import (
	"path/filepath"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

// The manifest recorded a tool at <bin>/fd while the installer had written
// <bin>/fd.exe, so os.Stat missed it, Supplied said no, and a freshly
// installed Windows box reported every tool "behind this release's pins" and
// re-downloaded the lot on the next install. One function now decides the
// name, so the recorder and the lookup cannot drift apart again.
func TestSuppliedPathIsTheFileThatWasActuallyWritten(t *testing.T) {
	for _, tc := range []struct{ goos, want string }{
		{"linux", "fd"},
		{"darwin", "fd"},
		{"windows", "fd.exe"},
	} {
		p := platform.Info{OS: tc.goos, DataDir: t.TempDir()}
		if got := filepath.Base(SuppliedPath(p, "fd")); got != tc.want {
			t.Errorf("SuppliedPath on %s = %q, want %q", tc.goos, got, tc.want)
		}
	}
}
