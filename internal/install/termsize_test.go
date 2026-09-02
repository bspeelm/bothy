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

// The real one, run by `go test`, has no terminal on any of the three.
func TestTerminalSizeIsHonestWhenThereIsNoTerminal(t *testing.T) {
	if _, _, ok := platform.TerminalSize(); ok {
		t.Skip("this test runs on a terminal; nothing to assert")
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
