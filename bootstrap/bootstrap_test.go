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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseServer serves a release the way GitHub does: the archive under
// /latest/download/, and the checksums.txt goreleaser publishes beside it.
//
// sums controls what that file says -- "" omits it entirely, which is what an
// old release looks like.
func releaseServer(archive []byte, sums string) (*httptest.Server, *string) {
	asked := new(string)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			if sums == "" {
				http.NotFound(w, r)
				return
			}
			fmt.Fprint(w, sums)
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			*asked = r.URL.Path
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	})), asked
}

// checksumsFor renders the checksums.txt goreleaser would publish.
func checksumsFor(archive []byte, name string) string {
	sum := sha256.Sum256(archive)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

// archiveName is what the running platform's archive is called, which is what
// the script will ask for.
func archiveName(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("sh", "-c",
		`case "$(uname -s)" in Linux) os=linux;; Darwin) os=darwin;; esac
		 case "$(uname -m)" in x86_64|amd64) a=amd64;; aarch64|arm64) a=arm64;; esac
		 echo "bothy_${os}_${a}.tar.gz"`).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

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

	srv, askedp := releaseServer(archive, checksumsFor(archive, archiveName(t)))
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
	asked := *askedp
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
	srv, _ := releaseServer(archive, checksumsFor(archive, archiveName(t)))
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
	srv, _ := releaseServer(nil, "")
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

// Every tool bothy installs is checksum-verified before it is written. For a
// long time the one unverified download in the whole system was bothy's own
// binary, which is a poor advertisement for the argument.
func TestBootstrapVerifiesTheChecksum(t *testing.T) {
	archive := releaseArchive(t, "#!/bin/sh\necho ok\n")
	srv, _ := releaseServer(archive, checksumsFor(archive, archiveName(t)))
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "checksum verified") {
		t.Errorf("no verification reported:\n%s", out)
	}
}

// The case the check exists for. A truncated or tampered download must not be
// installed, and must not be reported as success.
func TestBootstrapRefusesAMismatchedChecksum(t *testing.T) {
	archive := releaseArchive(t, "#!/bin/sh\necho ok\n")
	other := releaseArchive(t, "#!/bin/sh\necho something else entirely\n")
	// checksums.txt describes a different archive than the one served.
	srv, _ := releaseServer(archive, checksumsFor(other, archiveName(t)))
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a mismatched checksum reported success:\n%s", out)
	}
	if !strings.Contains(string(out), "checksum mismatch") {
		t.Errorf("the failure does not say what went wrong:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "bothy")); err == nil {
		t.Error("a mismatched archive was installed anyway")
	}
}

// An archive missing from checksums.txt is a broken release, not a reason to
// install it unchecked.
func TestBootstrapRefusesAnUnlistedArchive(t *testing.T) {
	archive := releaseArchive(t, "x")
	srv, _ := releaseServer(archive, checksumsFor(archive, "bothy_some_other_platform.tar.gz"))
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an unlisted archive reported success:\n%s", out)
	}
	if !strings.Contains(string(out), "not listed") {
		t.Errorf("the failure does not say what went wrong:\n%s", out)
	}
}

// Releases before 0.1.4 have no checksums.txt. Those must still install, and
// must say plainly that nothing was verified rather than implying it was.
func TestBootstrapSaysWhenItCannotVerify(t *testing.T) {
	archive := releaseArchive(t, "#!/bin/sh\necho ok\n")
	srv, _ := releaseServer(archive, "") // no checksums.txt published
	defer srv.Close()

	home := t.TempDir()
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = append(os.Environ(), "HOME="+home, "BOTHY_BASE_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("an old release without checksums.txt would not install: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "skipping verification") {
		t.Errorf("silently skipped verification:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "bothy")); err != nil {
		t.Error("nothing was installed")
	}
}
