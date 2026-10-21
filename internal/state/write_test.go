package state

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// zellij may be reading a layout while a second launch rewrites it, and a tool
// its config while `bothy install` does. Written in place, the reader's file is
// truncated under it; renamed into place, it keeps reading the whole old one.
func TestAReaderMidFileKeepsTheWholeOldVersion(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "cockpit.kdl")
	old := bytes.Repeat([]byte("o"), 4096)
	if err := os.WriteFile(dest, old, 0o644); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	if err := WriteFile(dest, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Errorf("a reader open before the write saw %d bytes %.8q, not the old file", len(got), got)
	}
	if now, _ := os.ReadFile(dest); string(now) != "new" {
		t.Errorf("the file now holds %q, want the new content", now)
	}
}

// The temporary file starts 0600; a config a tool reads as another user, or a
// script that must run, needs the mode the caller asked for.
func TestTheWrittenFileHasTheCallersMode(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "f")
	if err := WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", fi.Mode().Perm())
	}
}
