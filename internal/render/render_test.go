package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inRoot is where ADR-009 stops being a promise and becomes a check. An entire
// CI job exists to protect that promise, and the function enforcing it had no
// test of its own.
func TestWriteRefusesToLeaveBothysTree(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	w := NewWriter(root, "")

	for _, dest := range []string{
		filepath.Join(outside, "escaped.toml"),
		filepath.Join(root, "..", "escaped.toml"),
		filepath.Join(root, "a", "..", "..", "escaped.toml"),
		// Not checked for absence afterwards, for the obvious reason.
		"/etc/passwd",
	} {
		t.Run(dest, func(t *testing.T) {
			existed := func() bool { _, err := os.Stat(dest); return err == nil }()
			if _, err := w.Write(dest, []byte("x")); err == nil {
				t.Fatalf("Write(%q) was allowed outside the tree", dest)
			}
			if !existed {
				if _, err := os.Stat(dest); err == nil {
					t.Errorf("%s exists; the refusal did not stop the write", dest)
				}
			}
		})
	}
}

// A path merely *starting* with the root's name is not inside it: /tmp/xyz-evil
// shares a prefix with /tmp/xyz and is a different directory.
func TestWriteRefusesASiblingWithASharedPrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bothy")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(base, "bothy-evil", "c.toml")
	if _, err := NewWriter(root, "").Write(sibling, []byte("x")); err == nil {
		t.Error("a sibling sharing the root's prefix was treated as inside it")
	}
}

func TestWriteWithNoRootRefusesEverything(t *testing.T) {
	w := &Writer{}
	if _, err := w.Write(filepath.Join(t.TempDir(), "c.toml"), []byte("x")); err == nil {
		t.Error("a Writer with no root wrote a file; it has no tree to be inside of")
	}
}

func TestWriteReportsWhetherAnythingChanged(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root, "")
	dest := filepath.Join(root, "config", "c.toml")

	changed, err := w.Write(dest, []byte("a = 1\n"))
	if err != nil || !changed {
		t.Fatalf("first Write: changed=%v err=%v, want a change", changed, err)
	}
	// Rewriting identical bytes must report no change, or `install` claims to
	// have done something every time it is run.
	changed, err = w.Write(dest, []byte("a = 1\n"))
	if err != nil || changed {
		t.Fatalf("rewriting the same bytes: changed=%v err=%v", changed, err)
	}
	if changed, err = w.Write(dest, []byte("a = 2\n")); err != nil || !changed {
		t.Fatalf("rewriting different bytes: changed=%v err=%v", changed, err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "a = 2\n" {
		t.Errorf("file = %q, %v", got, err)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root, "")
	w.DryRun = true
	dest := filepath.Join(root, "c.toml")
	if _, err := w.Write(dest, []byte("a = 1\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("--dry-run created a file")
	}
}

func TestWriteExecIsExecutable(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "bin", "xdg-open")
	if _, err := NewWriter(root, "").WriteExec(dest, []byte("#!/bin/sh\n")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("mode = %v; a shim that is not executable is not a shim", fi.Mode())
	}
}

// The header is what tells a reader the file is not theirs to edit, and what
// IsGenerated later looks for. It has to be in the file's own comment syntax
// or the tool reading it will not start.
func TestHeaderUsesTheFileTypesCommentSyntax(t *testing.T) {
	for ext, want := range map[string]string{
		".toml": "#", ".kdl": "//", ".lua": "--", ".vim": `"`, ".sh": "#",
		".unknown": "#", // the fallback, which should be visible rather than silent
	} {
		h := Header("c"+ext, "yazi")
		if !strings.HasPrefix(h, want+" ") {
			t.Errorf("Header(%q) starts %q, want the %q comment style", ext, h[:4], want)
		}
	}
}

func TestIsGeneratedRecognisesOnlyBothysOwnFiles(t *testing.T) {
	dir := t.TempDir()
	mine := filepath.Join(dir, "mine.toml")
	if err := os.WriteFile(mine, []byte(Header(mine, "yazi")+"a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsGenerated(mine) {
		t.Error("bothy did not recognise a file it had just written the header into")
	}

	theirs := filepath.Join(dir, "theirs.toml")
	if err := os.WriteFile(theirs, []byte("a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsGenerated(theirs) {
		t.Error("a hand-written file was claimed as bothy's; install would overwrite it")
	}
	if IsGenerated(filepath.Join(dir, "absent.toml")) {
		t.Error("a file that does not exist was reported as generated")
	}

	// The marker only counts near the top. A file that merely mentions the
	// phrase further down is somebody's own notes, not bothy's output.
	deep := filepath.Join(dir, "deep.toml")
	body := strings.Repeat("# padding\n", HeaderLines+2) + "# " + Marker + "\n"
	if err := os.WriteFile(deep, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsGenerated(deep) {
		t.Error("the marker was honoured below the header window")
	}
}

// Overrides are appended verbatim after the generated body, so that whatever
// the user wrote wins in every format bothy emits.
func TestRenderAppendsTheUsersOverride(t *testing.T) {
	root, overrides := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(overrides, "yazi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overrides, "yazi", "yazi.toml"),
		[]byte("mine = true"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := NewWriter(root, overrides).Render(
		filepath.Join(root, "yazi.toml"), "yazi", "t", "generated = {{ .V }}\n",
		struct{ V string }{"1"})
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "generated = 1") {
		t.Error("the template body is missing")
	}
	if strings.Index(body, "mine = true") < strings.Index(body, "generated = 1") {
		t.Error("the override was not appended after the body, so it would not win")
	}
	if !strings.HasSuffix(body, "\n") {
		t.Error("an override without a trailing newline left the file without one")
	}
}

func TestRenderReportsATemplateThatDoesNotParse(t *testing.T) {
	root := t.TempDir()
	if _, err := NewWriter(root, "").Render(
		filepath.Join(root, "c.toml"), "yazi", "broken", "{{ .Unclosed", nil); err == nil {
		t.Error("a template that does not parse rendered anyway")
	}
}
