//go:build !unix

package platform

// TerminalSize has no answer where there is no TIOCGWINSZ. Windows has a
// console API instead, which nothing has needed yet (ADR-054) -- COLUMNS is
// then unset rather than guessed, and the layout treats that as "do not
// assume it fits".
func TerminalSize() (cols, rows int, ok bool) { return 0, 0, false }
