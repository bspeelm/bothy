//go:build !unix

package platform

// Mounted has no answer where there is no st_dev. On Windows that means
// `bothy connect` reports nothing mounted (ADR-054) -- it is a Linux and
// macOS feature, and this keeps the tree compiling.
func Mounted(path string) bool { return false }
