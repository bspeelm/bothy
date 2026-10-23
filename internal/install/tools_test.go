package install

import (
	"os"
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

// Yazi detects MIME types by shelling out to file(1) and has no fallback. On
// Windows there is none on PATH, so every entry reads "Cannot find `file` to
// detect the file's MIME type" and nothing previews or opens by type. Git for
// Windows ships one but exports only git, so bothy names the path.
func TestYaziIsPointedAtFileOneOnWindowsOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramFiles", root)
	t.Setenv("PATH", root) // no file.exe on it

	if got := fileOne(platform.Info{OS: "linux"}); got != "" {
		t.Errorf("linux = %q; file(1) is on PATH there and naming one would override it", got)
	}
	if got := fileOne(platform.Info{OS: "windows"}); got != "" {
		t.Errorf("windows with no Git = %q, want empty so the doctor can say so", got)
	}

	git := filepath.Join(root, "Git", "usr", "bin")
	if err := os.MkdirAll(git, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git, "file.exe"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := fileOne(platform.Info{OS: "windows"}); got != filepath.Join(git, "file.exe") {
		t.Errorf("windows with Git installed = %q, want the bundled file.exe", got)
	}
}
