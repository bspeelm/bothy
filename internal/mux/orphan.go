package mux

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Who is looking at a session, when the process tree cannot say.
//
// A terminal closing does not reach a multiplexer client running behind a
// container: podman exec ignores the hangup (containers/podman#19486), so the
// client keeps the session and every later launch is refused. The tree offers
// no way to tell that client from a live one -- both read
// zellij -> bothy -> conmon -> systemd -- so the terminal writes down that it
// is watching, and the record going stale is the evidence.

// procRoot is /proc, a var so a test can hand it a directory it built.
var procRoot = "/proc"

func ownerDir(stateDir string) string { return filepath.Join(stateDir, "sessions") }

// Own records this process as the terminal showing a session and returns the
// func that forgets it. The record carries the start time as well as the pid,
// because pids are reused and a reused one would read as a live owner.
//
// A bothy killed by the hangup never runs the returned func, which is the
// point: a record whose process is gone is the only evidence that a client
// still counted against the session has no window behind it.
func Own(stateDir, session string) func() {
	start, ok := startTime(os.Getpid())
	if !ok {
		return func() {}
	}
	path := filepath.Join(ownerDir(stateDir), session)
	if err := os.MkdirAll(ownerDir(stateDir), 0o755); err != nil {
		return func() {}
	}
	body := fmt.Sprintf("%d %s\n", os.Getpid(), start)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return func() {}
	}
	return func() { _ = os.Remove(path) }
}

// Owned reports whether a live process is showing this session. Not knowing
// answers yes: the caller refuses on a yes, and a refusal is the safe answer
// when the alternative is killing something on a guess.
func Owned(stateDir, session string) bool {
	body, err := os.ReadFile(filepath.Join(ownerDir(stateDir), session))
	if err != nil {
		return false
	}
	f := strings.Fields(string(body))
	if len(f) != 2 {
		return true
	}
	pid, err := strconv.Atoi(f[0])
	if err != nil {
		return true
	}
	// Whether the pid is gone and whether /proc can be read are different
	// answers. A missing process is the stale record this exists to find; an
	// unreadable /proc is ignorance, and ignorance refuses.
	if _, err := os.Stat(procRoot); err != nil {
		return true
	}
	start, ok := startTime(pid)
	return ok && start == f[1]
}

// startTime is field 22 of /proc/PID/stat. Parsed after the last ')' because
// the second field is the command name, which may itself contain spaces and
// brackets.
func startTime(pid int) (string, bool) {
	f, ok := statFields(pid)
	if !ok || len(f) < 20 {
		return "", false
	}
	return f[19], true
}

// statFields is /proc/PID/stat from the third field on, so f[0] is the state
// and f[19] the start time. Split after the last ')' because the second field
// is the command name, which may itself contain spaces and brackets.
func statFields(pid int) ([]string, bool) {
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, false
	}
	i := strings.LastIndex(string(b), ") ")
	if i < 0 {
		return nil, false
	}
	return strings.Fields(string(b)[i+2:]), true
}

// Reclaim ends the clients of a session that no terminal owns, and reports how
// many actually went. It waits for them, because the count it is clearing the
// way for is read from the multiplexer a moment later.
func Reclaim(tool, session string) int {
	var ended int
	for _, pid := range unwatchedClients(tool, session) {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		_ = proc.Signal(syscall.SIGTERM)
		if waitGone(pid) {
			ended++
			continue
		}
		// A client that will not leave holds the session shut, which is the
		// state this exists to end.
		_ = proc.Signal(syscall.SIGKILL)
		if waitGone(pid) {
			ended++
		}
	}
	return ended
}

func waitGone(pid int) bool {
	for i := 0; i < 50; i++ {
		if !alive(pid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// alive is false for a process that has exited but not yet been collected.
// /proc keeps the directory until the parent reaps it, so waiting for the
// directory to go would wait on somebody else's bookkeeping.
func alive(pid int) bool {
	f, ok := statFields(pid)
	return ok && len(f) > 0 && f[0] != "Z"
}

// unwatchedClients is every process that is a client of this session and is
// behind a container. Nothing on this side of the boundary qualifies: a client
// there dies with its terminal in milliseconds, so one still running is one
// somebody is using.
func unwatchedClients(tool, session string) []int {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil // no /proc: no answer, and the caller refuses instead
	}
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if isClientOf(argv, tool, session) && behindAContainer(pid) {
			out = append(out, pid)
		}
	}
	return out
}

// isClientOf reads one process's argv. The session has to be an argument of
// its own: the server names it too, inside the socket path it was given.
func isClientOf(argv []string, tool, session string) bool {
	if len(argv) < 2 || filepath.Base(argv[0]) != tool {
		return false
	}
	var attaches, names bool
	for _, a := range argv[1:] {
		switch a {
		case "attach":
			attaches = true
		case session:
			names = true
		}
	}
	return attaches && names
}

// behindAContainer reports whether a process runs in another mount namespace.
// The pid namespace is shared with a toolbox, so the pid is addressable from
// here; the mount namespace is what differs, and is readable.
func behindAContainer(pid int) bool {
	mine, err := os.Readlink(filepath.Join(procRoot, "self", "ns", "mnt"))
	if err != nil {
		return false
	}
	theirs, err := os.Readlink(filepath.Join(procRoot, strconv.Itoa(pid), "ns", "mnt"))
	return err == nil && theirs != mine
}
