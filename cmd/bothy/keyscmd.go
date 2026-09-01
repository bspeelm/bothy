package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
)

// cmdKeys prints the bindings someone needs on their first day, for people who
// have used neither Zellij nor Yazi and would otherwise be stuck in a window
// they cannot leave.
func cmdKeys(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("keys takes no arguments")
	}
	p, _, err := load()
	if err != nil {
		return err
	}
	fmt.Print(keysText(p))
	return nil
}

// muxKeys are Zellij's own defaults. bothy sets no keybindings -- its zellij
// config names a theme, an editor and a frame style and nothing else -- so
// these are quoted rather than read, and TestBothySetsNoKeybindings fails if
// that ever stops being true.
var muxKeys = []struct{ on, does string }{
	{"Alt-h  Alt-j  Alt-k  Alt-l", "move between the panes"},
	{"Alt-n", "another pane"},
	{"Ctrl-o d", "detach, leaving everything running"},
	{"Ctrl-q", "quit, and the session is gone"},
	{"Ctrl-s e", "open the scrollback in your editor"},
}

// keysText renders the list. The file-pane half is read from the keymap bothy
// generated rather than restated here, because a binding written in two places
// is a binding that will disagree with itself.
func keysText(p platform.Info) string {
	var b strings.Builder
	b.WriteString("\nThe workspace — these are Zellij's, and bothy leaves them alone.\n\n")
	for _, k := range muxKeys {
		fmt.Fprintf(&b, "  %-28s %s\n", k.on, k.does)
	}
	fmt.Fprintf(&b, "  %-28s %s\n", "bothy attach", "come back to a detached one")
	fmt.Fprintf(&b, "  %-28s %s\n", "bothy ls", "the rooms you have open")

	if written := browserKeys(p); len(written) > 0 {
		b.WriteString("\nThe file pane — these are bothy's.\n\n")
		for _, k := range written {
			fmt.Fprintf(&b, "  %-28s %s\n", k.on, k.does)
		}
	}
	b.WriteString("\nEverything else is the tool's own. bothy adds no bindings beyond these.\n")
	return b.String()
}

type binding struct{ on, does string }

// browserKeys reads what bothy wrote into Yazi's keymap. Each binding is
// conditional on its plugin being installed, so what is listed is what is
// actually bound on this machine.
func browserKeys(p platform.Info) []binding {
	raw, err := os.ReadFile(filepath.Join(install.YaziDir(p), "keymap.toml"))
	if err != nil {
		return nil
	}
	var file struct {
		Mgr struct {
			Prepend []struct {
				On   any    `toml:"on"`
				Desc string `toml:"desc"`
			} `toml:"prepend_keymap"`
		} `toml:"mgr"`
	}
	if err := toml.Unmarshal(raw, &file); err != nil {
		return nil
	}
	var out []binding
	for _, k := range file.Mgr.Prepend {
		if on := keyChord(k.On); on != "" && k.Desc != "" {
			out = append(out, binding{on, k.Desc})
		}
	}
	return out
}

// keyChord flattens Yazi's `on`, which is a string for one key and a list for
// a sequence.
func keyChord(on any) string {
	switch v := on.(type) {
	case string:
		return v
	case []any:
		var keys []string
		for _, k := range v {
			if s, ok := k.(string); ok {
				keys = append(keys, s)
			}
		}
		return strings.Join(keys, " ")
	}
	return ""
}

// showKeysOnce prints the bindings after a first-run setup and waits, because
// the multiplexer is about to take the screen and anything printed without a
// pause is gone before it can be read. Skipped when nothing is there to read
// it, which is also how the download prompt behaves.
func showKeysOnce(p platform.Info) {
	if !isTerminal(os.Stdin) {
		return
	}
	fmt.Print(keysText(p))
	fmt.Print("\n`bothy keys` prints this again. Press enter to start.")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
