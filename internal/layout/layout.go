// Package layout turns a profile into a Zellij layout.
//
// Users write profiles; bothy writes KDL, because Zellij's layout language has
// two traps (docs/origin-cheatsheet.md §3):
//
//  1. split_direction="vertical" produces *columns*, not rows — the opposite of
//     what the word suggests. A profile says "columns" and means columns.
//  2. A `{ plugin location="…" }` body written on one line needs a trailing
//     semicolon or Zellij fails to deserialise the node. This renderer always
//     emits plugin bodies across multiple lines, so the question never arises.
package layout

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Profile is one workspace arrangement, read from profiles/<name>.toml or
// ~/.config/bothy/profiles/<name>.toml. Deliberately two levels deep -- a
// stack of rows, each one pane or a set of columns -- because every layout in
// PLAN.md fits, and a nested tree buys expressiveness nobody asked for at the
// cost of a renderer that is harder to keep correct.
type Profile struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`

	// TabBar and StatusBar are fixed line counts, not percentages, so they are
	// flags rather than rows.
	TabBar    *bool `toml:"tab_bar"`
	StatusBar *bool `toml:"status_bar"`

	Rows []Row `toml:"rows"`
}

// Row is a horizontal band of the window.
type Row struct {
	// Size is a percentage ("50%") or a fixed line count ("3"). Empty means
	// "share what is left" — at most one row per profile should say nothing.
	Size  string `toml:"size"`
	Panes []Pane `toml:"panes"`
}

// Pane is one region. Either Slot or Command identifies what runs in it; a
// pane with neither is a plain shell, which the side pane wants.
type Pane struct {
	// Slot is resolved to a command by the caller ("browser" -> "yazi").
	Slot string `toml:"slot"`
	// Command overrides Slot with a literal command line.
	Command string `toml:"command"`
	Name    string `toml:"name"`
	Size    string `toml:"size"`
	Focus   bool   `toml:"focus"`
}

// Commands maps slot names to the command that runs in them.
type Commands map[string]string

// LoadProfile reads a profile from a TOML file.
func LoadProfile(path string) (Profile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("layout: %w", err)
	}
	return ParseProfile(src, path)
}

// ParseProfile reads a profile from bytes. name is what an error calls the
// source -- a path, or "profiles/cockpit.toml" for a shipped profile, which
// has none. Separate from LoadProfile because the shipped profiles are
// embedded, and a file loader meant writing them to a temp file to read back.
func ParseProfile(src []byte, name string) (Profile, error) {
	var p Profile
	if err := toml.Unmarshal(src, &p); err != nil {
		return Profile{}, fmt.Errorf("layout: %s: %w", name, err)
	}
	return p, p.Validate()
}

// Validate rejects profiles that would render into a layout Zellij accepts but
// nobody wants — an empty screen, or a focus fight between panes.
func (p Profile) Validate() error {
	if len(p.Rows) == 0 {
		return fmt.Errorf("layout: profile %q has no rows", p.Name)
	}
	focused := 0
	for ri, r := range p.Rows {
		if len(r.Panes) == 0 {
			return fmt.Errorf("layout: profile %q row %d has no panes", p.Name, ri+1)
		}
		for _, pane := range r.Panes {
			if pane.Focus {
				focused++
			}
		}
	}
	if focused > 1 {
		return fmt.Errorf("layout: profile %q focuses %d panes; exactly one may be focused", p.Name, focused)
	}
	return nil
}

// Slots lists the slots a profile needs filled, so install can refuse to write
// a layout whose panes would launch commands that were never installed.
func (p Profile) Slots() []string {
	seen := map[string]bool{}
	for _, r := range p.Rows {
		for _, pane := range r.Panes {
			if pane.Slot != "" {
				seen[pane.Slot] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// PaneCount is the number of content panes, excluding Zellij's own bars.
// The doctor compares this against the layout Zellij actually resolved, which
// is the only proof that the layout built the way the profile described.
func (p Profile) PaneCount() int {
	n := 0
	for _, r := range p.Rows {
		n += len(r.Panes)
	}
	return n
}

const managedHeader = `// managed by bothy — this file is generated at launch.
// Edit the profile instead (bothy config edit, or ~/.config/bothy/profiles/),
// then run 'bothy' again: Zellij applies layout changes at launch only.
`

// Render produces the Zellij KDL for a profile.
func Render(p Profile, cmds Commands) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(managedHeader)
	b.WriteString("layout {\n")

	// The bars are fixed line counts (1 and 2), not percentages. That asymmetry
	// matters: at lower resolutions they eat a larger fraction of the window,
	// so a row's share of the height is not its share of the screen.
	if enabled(p.TabBar, true) {
		writePlugin(&b, 1, "zellij:tab-bar")
	}

	for _, r := range p.Rows {
		if err := writeRow(&b, r, cmds); err != nil {
			return "", err
		}
	}

	if enabled(p.StatusBar, true) {
		writePlugin(&b, 2, "zellij:status-bar")
	}

	b.WriteString("}\n")
	return b.String(), nil
}

func enabled(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

// writePlugin emits a plugin pane. The body is always multi-line: written on
// one line, `{ plugin location="…" }` needs a trailing `;` or Zellij reports
// "Failed to deserialize KDL node".
func writePlugin(b *strings.Builder, size int, location string) {
	fmt.Fprintf(b, "    pane size=%d borderless=true {\n", size)
	fmt.Fprintf(b, "        plugin location=%q\n", location)
	b.WriteString("    }\n")
}

func writeRow(b *strings.Builder, r Row, cmds Commands) error {
	// A single-pane row is just that pane, carrying the row's size.
	if len(r.Panes) == 1 {
		pane := r.Panes[0]
		if pane.Size == "" {
			pane.Size = r.Size
		}
		return writePane(b, pane, cmds, 1)
	}

	// Several panes side by side. This is the trap: Zellij's "vertical" split
	// direction lays panes out as columns.
	b.WriteString("    pane")
	if r.Size != "" {
		fmt.Fprintf(b, " size=%s", quoteSize(r.Size))
	}
	b.WriteString(" split_direction=\"vertical\" {\n")
	for _, pane := range r.Panes {
		if err := writePane(b, pane, cmds, 2); err != nil {
			return err
		}
	}
	b.WriteString("    }\n")
	return nil
}

func writePane(b *strings.Builder, p Pane, cmds Commands, depth int) error {
	indent := strings.Repeat("    ", depth)

	cmd := p.Command
	if cmd == "" && p.Slot != "" {
		c, ok := cmds[p.Slot]
		if !ok || c == "" {
			return fmt.Errorf("layout: pane needs slot %q but no command is configured for it", p.Slot)
		}
		cmd = c
	}

	b.WriteString(indent + "pane")
	if p.Size != "" {
		fmt.Fprintf(b, " size=%s", quoteSize(p.Size))
	}
	if p.Focus {
		b.WriteString(" focus=true")
	}
	if p.Name != "" {
		fmt.Fprintf(b, " name=%q", p.Name)
	}

	if cmd == "" {
		// A pane with no command runs the user's shell. Zellij wants no body
		// at all here, not an empty one.
		b.WriteString("\n")
		return nil
	}

	b.WriteString(" {\n")
	if err := writeCommand(b, indent+"    ", cmd); err != nil {
		return fmt.Errorf("layout: pane %q: %w", p.Name, err)
	}
	b.WriteString(indent + "}\n")
	return nil
}

// writeCommand splits a command line into Zellij's `command` plus `args`.
// Zellij execs the command directly rather than through a shell, so
// "claude --continue" as a single string would look for a binary with a space
// in its name.
func writeCommand(b *strings.Builder, indent, cmd string) error {
	parts, err := splitCommand(cmd)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return fmt.Errorf("the command is empty")
	}
	fmt.Fprintf(b, "%scommand %q\n", indent, parts[0])
	if len(parts) > 1 {
		b.WriteString(indent + "args")
		for _, a := range parts[1:] {
			fmt.Fprintf(b, " %q", a)
		}
		b.WriteString("\n")
	}
	return nil
}

// splitCommand splits a command line into words the way a shell would,
// honouring quotes, so `claude --append-system-prompt "be terse"` yields
// three arguments. It is deliberately not a shell: no expansion, globbing
// or operators. PLAN.md §13 caps dependencies at go-toml.
func splitCommand(s string) ([]string, error) {
	var (
		parts []string
		cur   strings.Builder
		began bool // distinguishes "" as an argument from no argument
		quote rune // 0, '\'' or '"'
	)
	for i, r := range s {
		switch {
		case r == '\\' && quote != '\'' && i+1 < len(s):
			// A backslash escapes the next character, except inside single
			// quotes, where a shell treats it literally.
			continue
		case i > 0 && s[i-1] == '\\' && quote != '\'':
			cur.WriteRune(r)
			began = true
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
			began = true
		case r == ' ' || r == '\t':
			if began {
				parts = append(parts, cur.String())
				cur.Reset()
				began = false
			}
		default:
			cur.WriteRune(r)
			began = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unbalanced %c quote in %q", quote, s)
	}
	if began {
		parts = append(parts, cur.String())
	}
	return parts, nil
}

// quoteSize quotes percentages but leaves bare line counts unquoted, matching
// how Zellij's own example layouts are written.
func quoteSize(s string) string {
	if strings.HasSuffix(s, "%") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
