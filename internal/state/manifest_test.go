package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A machine bothy has never installed on is not an error condition, and
// treating it as one would make `doctor` and `uninstall` fail on exactly the
// systems where they have the least to say.
func TestLoadTreatsAMissingManifestAsEmpty(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "never-installed"))
	if err != nil {
		t.Fatalf("Load() on a fresh machine = %v", err)
	}
	if len(m.Binaries) != 0 || m.InstalledIn != "" {
		t.Errorf("a missing manifest came back non-empty: %+v", m)
	}
	if m.Version != ManifestVersion {
		t.Errorf("Version = %d, want the current %d", m.Version, ManifestVersion)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{InstalledIn: "bothy-test"}
	m.RecordBinary(Binary{Name: "zellij", Path: "/b/zellij", Version: "0.45.1", SHA256: "aa", Source: "bothy"})
	m.RecordBinary(Binary{Name: "jq", Path: "/usr/bin/jq", Version: "1.8.1", Source: "/usr/bin/jq"})
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.InstalledIn != "bothy-test" {
		t.Errorf("InstalledIn = %q; bothy would look for its tools in the wrong place", got.InstalledIn)
	}
	if len(got.Binaries) != 2 {
		t.Fatalf("got %d binaries, want 2", len(got.Binaries))
	}
	// Sorted on save, so the file is a reviewable diff rather than a reshuffle.
	if got.Binaries[0].Name != "jq" || got.Binaries[1].Name != "zellij" {
		t.Errorf("binaries are not sorted by name: %q, %q", got.Binaries[0].Name, got.Binaries[1].Name)
	}
	if got.Binaries[1].SHA256 != "aa" || got.Binaries[1].Source != "bothy" {
		t.Errorf("zellij lost fields in the round trip: %+v", got.Binaries[1])
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not stamped")
	}
}

// RecordBinary replaces rather than appends, or reinstalling would leave the
// manifest with two entries for one tool and uninstall guessing between them.
func TestRecordBinaryReplacesByName(t *testing.T) {
	m := &Manifest{}
	m.RecordBinary(Binary{Name: "zellij", Version: "0.42.2"})
	m.RecordBinary(Binary{Name: "zellij", Version: "0.45.1"})
	if len(m.Binaries) != 1 {
		t.Fatalf("got %d entries for one tool", len(m.Binaries))
	}
	if m.Binaries[0].Version != "0.45.1" {
		t.Errorf("Version = %q, want the newer record to win", m.Binaries[0].Version)
	}
}

func TestSaveCreatesTheStateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "nested")
	if err := (&Manifest{}).Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ManifestPath(dir)); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
	// The atomic write must not leave its temporary file behind, or the state
	// directory accumulates one per install.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("%s was left behind by the atomic write", e.Name())
		}
	}
}

// A manifest written by a newer bothy has to be refused rather than guessed
// at: the alternative is uninstall deleting from a schema it cannot read.
func TestLoadRefusesAManifestFromTheFuture(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(ManifestPath(dir),
		[]byte(`{"version":999,"binaries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("a manifest from a newer bothy was read anyway")
	}
	if !strings.Contains(err.Error(), "999") {
		t.Errorf("the error does not say which version it found: %v", err)
	}
}

func TestLoadReportsACorruptManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(ManifestPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("a corrupt manifest loaded without complaint")
	}
	// Naming the file matters: it is the one the reader has to go and fix.
	if !strings.Contains(err.Error(), "manifest.json") {
		t.Errorf("the error does not name the file: %v", err)
	}
}
