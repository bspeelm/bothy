// Package bootstrap tests the one piece of shell in this project.
//
// install.sh had never been run end to end, because running it needs a
// published release and there has never been one. That is a bad reason for the
// first line of the README to be untested: it is the first thing anyone does,
// and a mistake in it is a mistake nobody can work around.
//
// So the test serves a release-shaped archive over a local HTTP server and runs
// the real script against it. Everything except GitHub is exercised.
package bootstrap

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseArchive builds the tarball .goreleaser.yaml describes: the binary at
// the root of the archive, no wrapping directory.
func releaseArchive(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: "bothy", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestBootstrapInstallsTheBinary(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	archive := releaseArchive(t, "#!/bin/sh\necho bothy-under-test\n")

	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Path
		w.Write(archive)
	}))
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}

	// The URL must match what goreleaser publishes, or the script 404s for
	// everyone on the day of the first release.
	if !strings.Contains(asked, "/latest/download/bothy_") {
		t.Errorf("requested %q, which is not the release layout", asked)
	}
	if !strings.HasSuffix(asked, ".tar.gz") {
		t.Errorf("requested %q, want a .tar.gz", asked)
	}

	dest := filepath.Join(home, ".local", "bin", "bothy")
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("nothing installed: %v", err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Error("the installed binary is not executable")
	}
}

// ~/.local/bin missing from PATH is the most common reason a fresh install
// looks like it did nothing.
func TestBootstrapWarnsWhenBindirIsNotOnPath(t *testing.T) {
	archive := releaseArchive(t, "x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL, "PATH=/usr/bin:/bin")
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "not on your PATH") {
		t.Errorf("no PATH warning:\n%s", out)
	}

	// And it must not warn when the directory *is* on PATH.
	cmd = exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL,
		"PATH="+filepath.Join(home, ".local", "bin")+":/usr/bin:/bin")
	out, _ = cmd.CombinedOutput()
	if strings.Contains(string(out), "not on your PATH") {
		t.Errorf("warned about a directory that is on PATH:\n%s", out)
	}
}

// A download that fails must not leave a half-installed binary behind.
func TestBootstrapLeavesNothingOnAFailedDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("a 404 download reported success:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "bothy")); err == nil {
		t.Error("a failed download still installed something")
	}
}

// The script tells you what to run next, and that instruction changed when
// `bothy` learned to set itself up on first run.
func TestBootstrapPointsAtTheRightNextStep(t *testing.T) {
	src, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "next: bothy install") {
		t.Error("still tells people to run 'bothy install'; first run sets up on its own")
	}
}
