//go:build !unix && !windows

package platform

// TerminalSize has no answer where there is neither TIOCGWINSZ nor a console
// API -- COLUMNS is then unset rather than guessed, and the layout treats that
// as "do not assume it fits".
func TerminalSize() (cols, rows int, ok bool) { return 0, 0, false }
