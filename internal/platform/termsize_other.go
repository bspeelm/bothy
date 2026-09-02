//go:build !unix

package platform

// TerminalSize has no answer where there is no TIOCGWINSZ. Windows has a
// console API instead, and bothy does not run there yet (ADR-018) -- this
// exists so the tree keeps compiling for it.
func TerminalSize() (cols, rows int, ok bool) { return 0, 0, false }
