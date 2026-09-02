package mux

import (
	"fmt"
	"strings"

	"github.com/bspeelm/bothy/internal/layout"
)

const managedHeader = `// managed by bothy — this file is generated at launch.
// Edit the profile instead (bothy config edit, or ~/.config/bothy/profiles/),
// then run 'bothy' again: Zellij applies layout changes at launch only.
`

// Render produces the Zellij KDL for a profile.
func render(p layout.Profile, cmds layout.Commands) (string, error) {
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
func writeRow(b *strings.Builder, r layout.Row, cmds layout.Commands) error {
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
func writePane(b *strings.Builder, p layout.Pane, cmds layout.Commands, depth int) error {
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
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\\' && quote != '\'' && i+1 < len(runes):
			// Both characters are consumed here rather than skipping and
			// looking back: a lookback cannot see a second backslash, because
			// this branch has already taken it. Inside double quotes a shell
			// escapes only these four, so "a\nb" keeps its backslash.
			next := runes[i+1]
			if quote == '"' && !strings.ContainsRune(`$`+"`"+`"\\`, next) {
				cur.WriteRune(r)
			} else {
				i++
				cur.WriteRune(next)
			}
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
