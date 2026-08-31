package install

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bothy-dev/bothy/internal/platform"
)

// Running is a process still executing a binary bothy supplied.
type Running struct {
	PID     int
	Path    string
	Cmdline string
}

// StillRunning finds processes executing binaries from bothy's bin.
//
// This exists because uninstall deleted a multiplexer out from under a live
// session. The process kept running — Linux holds the inode open — but its
// binary was gone, so the session could never be attached to again and its
// memory could not be reclaimed until reboot. Removing files a running process
// depends on is a thing an uninstaller should notice, not something the user
// discovers later.
//
// It matches on /proc/PID/cmdline rather than the /proc/PID/exe link, which is
// the obvious choice and the wrong one. Reading the exe link needs ptrace
// permission, and that is refused across the user-namespace boundary of a
// rootless container — so from inside a toolbox it silently found nothing,
// while cmdline read the very same process fine. In an environment built on
// containers with a shared home, "silently found nothing" is the failure mode
// that matters.
//
// Linux-only. Elsewhere it reports nothing, which is the right failure: a
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
