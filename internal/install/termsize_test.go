package install

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
)

func envValue(env []string, key string) (string, bool) {
	for _, kv := range env {
		if name, value, ok := strings.Cut(kv, "="); ok && name == key {
			return value, true
		}
	}
	return "", false
}

// A pane's command can start before the multiplexer has sized its pty, and a
// pty nobody has sized reports 0x0. Yazi asks how big the terminal is, gets
// nothing from either the ioctl or these variables, and exits -- so the
// workspace opens with a dead file browser and one line of error in it.
func TestSessionCarriesTheTerminalSize(t *testing.T) {
	restore := terminalSize
	t.Cleanup(func() { terminalSize = restore })
	terminalSize = func() (int, int, bool) { return 176, 44, true }

	env := SessionEnv(platform.Info{Home: t.TempDir()}, config.Default())
	if got, ok := envValue(env, "COLUMNS"); !ok || got != "176" {
		t.Errorf("COLUMNS = %q (present=%v), want 176", got, ok)
	}
	if got, ok := envValue(env, "LINES"); !ok || got != "44" {
		t.Errorf("LINES = %q (present=%v), want 44", got, ok)
	}
}

// With no terminal to ask -- a pipe, a desktop launcher, CI -- there is no
// honest answer, and inventing one would hand every tool in the session a
// size that was never true.
func TestNoTerminalMeansNoSizeInTheEnvironment(t *testing.T) {
	restore := terminalSize
	t.Cleanup(func() { terminalSize = restore })
	terminalSize = func() (int, int, bool) { return 0, 0, false }

	env := SessionEnv(platform.Info{Home: t.TempDir()}, config.Default())
	for _, key := range []string{"COLUMNS", "LINES"} {
		if got, ok := envValue(env, key); ok {
			t.Errorf("%s = %q with no terminal to ask", key, got)
		}
	}
}

// The real one. Under `go test` the three streams are pipes, so the answer
// must be "no terminal" -- and must say so by returning false rather than by
// returning a size nobody can use. Both arms assert: a Windows console API
// that succeeds on a pipe would report a size here, which is how the old
// stty-shaped answer hid on that platform for five releases (ADR-059).
func TestTerminalSizeIsHonestAboutWhatItMeasured(t *testing.T) {
	cols, rows, ok := platform.TerminalSize()
	if ok && (cols <= 0 || rows <= 0) {
		t.Errorf("TerminalSize() = %d x %d, ok -- a size nothing can use", cols, rows)
	}
	if !ok && (cols != 0 || rows != 0) {
		t.Errorf("TerminalSize() = %d x %d with ok false; the caller reads these", cols, rows)
	}
}

// The environment bothy starts from carries whatever the shell had, and inside
// a bothy session that includes the *outer* session's size. Passing it on
// would hand every tool a number from another terminal, which is worse than
// handing them nothing: nothing is what makes a tool ask the ioctl.
func TestAnInheritedSizeIsNotPassedOn(t *testing.T) {
	t.Setenv("COLUMNS", "999")
	t.Setenv("LINES", "999")

	restore := terminalSize
	t.Cleanup(func() { terminalSize = restore })
	terminalSize = func() (int, int, bool) { return 0, 0, false }

	env := SessionEnv(platform.Info{Home: t.TempDir()}, config.Default())
	for _, key := range []string{"COLUMNS", "LINES"} {
		if got, ok := envValue(env, key); ok {
			t.Errorf("%s = %q, inherited from a terminal that is not this one", key, got)
		}
	}
}

// And a size bothy did measure replaces the inherited one rather than losing
// to it.
func TestAMeasuredSizeWinsOverTheInheritedOne(t *testing.T) {
	t.Setenv("COLUMNS", "999")

	restore := terminalSize
	t.Cleanup(func() { terminalSize = restore })
	terminalSize = func() (int, int, bool) { return 176, 44, true }

	env := SessionEnv(platform.Info{Home: t.TempDir()}, config.Default())
	if got, _ := envValue(env, "COLUMNS"); got != "176" {
		t.Errorf("COLUMNS = %q, want the measured 176", got)
	}
}

// The bug that emptied PATH on Windows. The block there spells it Path and
// compares names without case; bothy looked up PATH, missed, and wrote a
// second variable holding its own bin and the empty string it had just read.
// Windows collapsed the pair, keeping bothy's. Every tool it launched then
// had bothy's bin and nothing else: measured, a tower mirror could not find
// zellij and its pane exited 1, which is why the tower was blank there.
func TestTheSessionKeepsTheMachinesPathOnWindows(t *testing.T) {
	e := newEnv([]string{`Path=C:\WINDOWS\system32;C:\Program Files\Git\cmd`, "USERPROFILE=C:/u"}, true)
	e.set("PATH", `C:\bothy\bin`+";"+e.get("PATH"))

	var paths []string
	for _, kv := range e.slice() {
		if name, value, _ := strings.Cut(kv, "="); strings.EqualFold(name, "PATH") {
			paths = append(paths, value)
		}
	}
	if len(paths) != 1 {
		t.Fatalf("the session carries %d PATH variables %q; Windows keeps one of them and "+
			"the loser is the machine's own", len(paths), paths)
	}
	if !strings.Contains(paths[0], `C:\WINDOWS\system32`) {
		t.Errorf("PATH = %q, which has lost the machine's own directories", paths[0])
	}
	if !strings.HasPrefix(paths[0], `C:\bothy\bin`) {
		t.Errorf("PATH = %q, want bothy's bin first", paths[0])
	}
}

// And Unix is untouched: names are case-sensitive there, so two spellings are
// two variables and folding them would merge things the system keeps apart.
func TestUnixNamesAreNotFolded(t *testing.T) {
	e := newEnv([]string{"Path=/one", "PATH=/two"}, false)
	if got := e.get("PATH"); got != "/two" {
		t.Errorf("get(PATH) = %q, want /two", got)
	}
	if got := e.get("Path"); got != "/one" {
		t.Errorf("get(Path) = %q, want /one", got)
	}
}
