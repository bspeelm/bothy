//go:build !unix

package platform

// Mounted has no answer where there is no st_dev. bothy does not run on
// Windows (ADR-018) -- this exists so the tree keeps compiling for it.
func Mounted(path string) bool { return false }
