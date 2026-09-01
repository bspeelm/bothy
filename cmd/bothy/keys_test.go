package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
)

// The multiplexer bindings in muxKeys are quoted rather than read, which is
// only honest while bothy sets none of its own. If its zellij config ever
// grows a keybinds block, these become a second copy that can disagree with
// the first, and `bothy keys` has to read that block instead.
func TestBothySetsNoMultiplexerKeybindings(t *testing.T) {
	raw, err := bothy.Templates.ReadFile("templates/mux/zellij/config.kdl.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		code, _, _ := strings.Cut(line, "//")
		if strings.Contains(code, "keybinds") || strings.Contains(code, "bind ") {
			t.Fatalf("bothy now sets a zellij keybinding:\n     %s\n"+
				"     `bothy keys` quotes zellij's defaults; it must read these instead", strings.TrimSpace(line))
		}
	}
}

// The file-pane half is read from the keymap bothy generated, because a
// binding written in two places is a binding that will disagree with itself.
func TestKeysReadsTheBindingsBothyWrote(t *testing.T) {
	home := t.TempDir()
	p := platform.Info{Home: home, DataDir: filepath.Join(home, ".local", "share")}
	if err := os.MkdirAll(install.YaziDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	keymap := `
[[mgr.prepend_keymap]]
on   = "l"
run  = "plugin smart-enter"
desc = "Enter the child directory, or open the file"

[[mgr.prepend_keymap]]
on   = [ "c", "m" ]
run  = "plugin chmod"
desc = "Chmod on selected files"
`
	if err := os.WriteFile(filepath.Join(install.YaziDir(p), "keymap.toml"), []byte(keymap), 0o644); err != nil {
		t.Fatal(err)
	}

	got := browserKeys(p)
	if len(got) != 2 {
		t.Fatalf("read %d bindings, want 2: %+v", len(got), got)
	}
	if got[0].on != "l" || got[0].does != "Enter the child directory, or open the file" {
		t.Errorf("first binding = %+v", got[0])
	}
	// A sequence arrives as a list and has to read as one chord.
	if got[1].on != "c m" {
		t.Errorf("key sequence = %q, want %q", got[1].on, "c m")
	}
}

// A plugin that is not installed has no binding written, so it must not be
// listed either: telling someone about a key that does nothing is worse than
// saying nothing.
func TestKeysListsNothingWhenNoKeymapWasWritten(t *testing.T) {
	home := t.TempDir()
	p := platform.Info{Home: home, DataDir: filepath.Join(home, ".local", "share")}
	if got := browserKeys(p); len(got) != 0 {
		t.Errorf("browserKeys with no keymap = %+v, want none", got)
	}
	if strings.Contains(keysText(p), "The file pane") {
		t.Error("keysText printed an empty file-pane section")
	}
}
