package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
)

// Small shared helpers. Dispatch lives in main.go and stays there:
// docs_test.go reads that file by name to check the README against it.

func tilde(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// expandDir is config.Expand plus an absolute path, which a --dir argument
// needs and a configured path does not.
func expandDir(dir, home string) string {
	if expanded := config.Expand(dir, home); expanded != dir {
		return expanded
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// fileExists reports whether a path is a regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// hopTarget is the container the workspace will run in, or "" when it runs
// here. Only meaningful from outside a container: inside one, this is it.
func hopTarget(p platform.Info, cfg config.Config) string {
	if p.InContainer() {
		return ""
	}
	return install.ContainerFor(p, cfg)
}
