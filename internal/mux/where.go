package mux

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Where a session actually is, as opposed to where a record says it belongs.
//
// A toolbox shares the pid namespace with its host, so the server process is
// addressable from either side and its own /run/.containerenv -- reached
// through /proc/<pid>/root -- names the container it is in. Nothing else
// answers this: the session name carries no container, and asking the
// container would mean entering it, which starts it.

// ServerBox is the container the server holding session runs in. Not found,
// not running, and running outside any container are one answer: "".
func ServerBox(tool, session string) string {
	pid, ok := serverPid(tool, session)
	if !ok {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "root", "run", ".containerenv"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if v, ok := strings.CutPrefix(line, "name="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// serverPid finds the server holding a session, which names it in the socket
// path it was given -- the last argument. The clients name it too, so
// --server is what separates the one process that is the session from the
// several that are looking at it.
func serverPid(tool, session string) (int, bool) {
	found, ok := 0, false
	eachProc(func(pid int, argv []string) {
		if ok || len(argv) < 3 || filepath.Base(argv[0]) != tool {
			return
		}
		if slices.Contains(argv, "--server") && filepath.Base(argv[len(argv)-1]) == session {
			found, ok = pid, true
		}
	})
	return found, ok
}
