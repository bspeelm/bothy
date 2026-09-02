package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bspeelm/bothy/internal/slots"
	"github.com/bspeelm/bothy/internal/state"
	"github.com/bspeelm/bothy/internal/tools"
)

// #55. Whether `bothy install` upgraded a tool used to depend on where it was
// typed: bothy's own bin is on PATH inside a bothy session and not outside
// one, so Resolve saw a supplied tool as "installed" in one shell and "not
// installed" in the other. What the system has does not change with where you
// stand.
func TestSystemLookupIgnoresBothysOwnBin(t *testing.T) {
	own, other := t.TempDir(), t.TempDir()
	for _, dir := range []string{own, other} {
		if err := os.WriteFile(filepath.Join(dir, "zellij"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// bothy's bin first, exactly as a bothy session puts it.
	t.Setenv("PATH", own+string(os.PathListSeparator)+other)

	got, err := tools.SystemLookPath(own)("zellij")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(other, "zellij"); got != want {
		t.Errorf("found %q, want the system's copy at %q", got, want)
	}

	// And with only bothy's copy present, the system has none.
	t.Setenv("PATH", own)
	if path, err := tools.SystemLookPath(own)("zellij"); err == nil {
		t.Errorf("bothy's own copy at %q counted as the system's", path)
	}
}

// A tool bothy supplied is judged against the pin, not the minimum: at the
// pinned version there is nothing to do, and when the pin moves there is.
func TestSuppliedComparesAgainstThePin(t *testing.T) {
	p := sandboxPlatform(t)
	bin := filepath.Join(p.BinDir(), "zellij")
	if err := os.MkdirAll(p.BinDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := &state.Manifest{}
	m.RecordBinary(state.Binary{Name: "zellij", Path: bin, Version: "0.45.1", Source: "bothy"})

	if !Supplied(p, m, "zellij", "0.45.1") {
		t.Error("a tool at the pinned version was not recognised as current")
	}
	if Supplied(p, m, "zellij", "0.46.0") {
		t.Error("a tool behind the pin was reported current, so a moved pin would never install")
	}
}

// The system's own copy is not bothy's to keep current -- fill gaps, never
// replace -- so it is never counted as supplied however new it is.
func TestSuppliedIgnoresTheSystemsCopy(t *testing.T) {
	p := sandboxPlatform(t)
	m := &state.Manifest{}
	m.RecordBinary(state.Binary{Name: "jq", Path: "/usr/bin/jq", Version: "1.8.2", Source: "/usr/bin/jq"})

	if Supplied(p, m, "jq", "1.8.2") {
		t.Error("the system's own jq was treated as bothy's to upgrade")
	}
}

// A tool bothy has at the pinned version costs nothing, so the first-run
// prompt must not offer to download it.
func TestPendingFetchesLeavesOutWhatIsAlreadyThere(t *testing.T) {
	p := sandboxPlatform(t)
	decisions := []tools.Decision{
		{Tool: tools.Tool{Header: slots.Header{Name: "jq"}, Binary: "jq"}, Action: tools.UseSystem},
		{Tool: tools.Tool{Header: slots.Header{Name: "zellij"}, Binary: "zellij"}, Action: tools.Fetch},
	}
	// No manifest and no state: everything that would be fetched still is.
	if got := PendingFetches(p, decisions); len(got) != 1 || got[0].Tool.Name != "zellij" {
		t.Errorf("PendingFetches = %+v, want just zellij", got)
	}
}
