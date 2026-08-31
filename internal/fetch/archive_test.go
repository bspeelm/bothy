package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

// Release archives disagree about layout, so these fixtures reproduce the
// shapes bothy actually meets rather than an idealised one.

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	return buf.Bytes()
}

// zellij and lazygit put the binary at the archive root.
func TestExtractTarGzAtRoot(t *testing.T) {
	body := tarGz(t, map[string]string{"zellij": "ELF", "LICENSE": "MIT"})
	got, err := Extract(body, "zellij-x86_64-unknown-linux-musl.tar.gz", []string{"zellij"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got["zellij"]) != "ELF" {
		t.Errorf("got %q", got["zellij"])
	}
	if _, ok := got["LICENSE"]; ok {
		t.Error("extracted a file that was not wanted")
	}
}

// delta, ripgrep, fd and zoxide nest the binary inside a versioned directory.
// Matching on basename is what makes one code path cover both layouts.
func TestExtractTarGzInVersionedDirectory(t *testing.T) {
	body := tarGz(t, map[string]string{
		"delta-0.19.2-x86_64-unknown-linux-musl/delta":   "ELF",
		"delta-0.19.2-x86_64-unknown-linux-musl/README":  "hi",
		"delta-0.19.2-x86_64-unknown-linux-musl/doc/man": "man",
	})
	got, err := Extract(body, "delta-0.19.2-x86_64-unknown-linux-musl.tar.gz", []string{"delta"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got["delta"]) != "ELF" {
		t.Errorf("got %q", got["delta"])
	}
}

// yazi ships a zip carrying two binaries: yazi and its package manager, ya.
func TestExtractZipWithTwoBinaries(t *testing.T) {
	body := zipOf(t, map[string]string{
		"yazi-x86_64-unknown-linux-musl/yazi":      "YAZI",
		"yazi-x86_64-unknown-linux-musl/ya":        "YA",
		"yazi-x86_64-unknown-linux-musl/README.md": "docs",
	})
	got, err := Extract(body, "yazi-x86_64-unknown-linux-musl.zip", []string{"yazi", "ya"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got["yazi"]) != "YAZI" || string(got["ya"]) != "YA" {
		t.Errorf("got yazi=%q ya=%q", got["yazi"], got["ya"])
	}
}

// jq publishes a bare binary with no archive at all.
func TestExtractBareBinary(t *testing.T) {
	got, err := Extract([]byte("JQBINARY"), "jq-linux-amd64", []string{"jq"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got["jq"]) != "JQBINARY" {
		t.Errorf("got %q", got["jq"])
	}
}

// tar.xz would need a dependency the project has not taken. Failing clearly
// beats failing mysteriously when helix is eventually added.
func TestExtractTarXzSaysWhyItCannot(t *testing.T) {
	_, err := Extract([]byte("x"), "helix-25.07.1-x86_64-linux.tar.xz", []string{"hx"})
	if err == nil {
		t.Fatal("expected an error for tar.xz")
	}
	if !strings.Contains(err.Error(), "tar.xz") {
		t.Errorf("error should name the format: %v", err)
	}
}

func TestVersionFromTag(t *testing.T) {
	// Each project decorates its tags differently, and the asset names in
	// slots/tools all interpolate the undecorated number.
	for tag, want := range map[string]string{
		"v0.45.1":  "0.45.1", // zellij, yazi, fd, fzf, zoxide, lazygit
		"0.19.2":   "0.19.2", // delta
		"15.2.0":   "15.2.0", // ripgrep
		"jq-1.8.2": "1.8.2",  // jq
		"v10.5.0":  "10.5.0",
	} {
		if got := VersionFromTag(tag); got != want {
			t.Errorf("VersionFromTag(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestSumIsStable(t *testing.T) {
	// Empty-input sha256, so a change to the hashing is caught immediately.
	if got := Sum(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Sum(nil) = %s", got)
	}
}

func TestReleaseURL(t *testing.T) {
	got := ReleaseURL("zellij-org/zellij", "v0.45.1", "zellij-x86_64-unknown-linux-musl.tar.gz")
	want := "https://github.com/zellij-org/zellij/releases/download/v0.45.1/zellij-x86_64-unknown-linux-musl.tar.gz"
	if got != want {
		t.Errorf("got %s", got)
	}
}

// An archive whose entries try to escape is not a release bothy should unpack,
// whatever it is called. The bytes were never written to a path the archive
// chose, so this was already harmless -- but quietly taking "passwd" out of
// "../../etc/passwd" treats a hostile archive as an ordinary one.
func TestExtractRefusesTraversingEntries(t *testing.T) {
	for _, name := range []string{
		"../../etc/passwd",
		"../bothy",
		"/etc/passwd",
		"a/../../b/bothy",
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			body := []byte("x")
			if err := tw.WriteHeader(&tar.Header{
				Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
			}); err != nil {
				t.Fatal(err)
			}
			tw.Write(body)
			tw.Close()
			gz.Close()

			if _, err := Extract(buf.Bytes(), "t.tar.gz", []string{"passwd", "bothy"}); err == nil {
				t.Errorf("entry %q was accepted", name)
			}
		})
	}
}

// And an ordinary nested path -- which is what every real release looks like --
// still works.
func TestExtractAcceptsOrdinaryNestedPaths(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\n")
	if err := tw.WriteHeader(&tar.Header{
		Name: "bothy-0.1.4/bin/bothy", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	tw.Write(body)
	tw.Close()
	gz.Close()

	got, err := Extract(buf.Bytes(), "t.tar.gz", []string{"bothy"})
	if err != nil {
		t.Fatalf("a normal nested entry was refused: %v", err)
	}
	if string(got["bothy"]) != string(body) {
		t.Errorf("extracted %q", got["bothy"])
	}
}
