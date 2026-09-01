package install

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bspeelm/bothy/internal/platform"
)

// Running is a process still executing a binary bothy supplied.
type Running struct {
	PID     int
	Path    string
	Cmdline string
}

// StillRunning finds processes executing binaries from bothy's bin, so that
// uninstall can warn before deleting one out from under a live session: Linux
// holds the inode open, so the session keeps running but can never be
// reattached.
//
// Matches on /proc/PID/cmdline rather than the /proc/PID/exe link. Reading the
// exe link needs ptrace permission, which is refused across the user-namespace
// boundary of a rootless container — so inside a toolbox it finds nothing at
// all, while cmdline reads the same process fine.
//
// Linux-only; elsewhere it reports nothing, which is the right failure. A
// missing warning beats a wrong one.
func StillRunning(p platform.Info) []Running {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	prefix := p.BinDir() + string(filepath.Separator)

	var out []Running
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if len(args) == 0 || !strings.HasPrefix(args[0], prefix) {
			continue
		}
		out = append(out, Running{
			PID:     pid,
			Path:    args[0],
			Cmdline: strings.Join(args, " "),
		})
	}
	return out
}
