package mux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
)

// Zellij is the multiplexer CI tests.
type Zellij struct{}

func (Zellij) Name() string { return "zellij" }

// Preview is the KDL zellij is handed at launch.
func (Zellij) Preview(p layout.Profile, cmds layout.Commands) (string, error) {
	return render(p, cmds)
}

func (Zellij) Dir(p platform.Info) string {
	return filepath.Join(p.ConfigRoot(), "zellij")
}

func (z Zellij) SessionEnv(p platform.Info) map[string]string {
	return map[string]string{"ZELLIJ_CONFIG_DIR": z.Dir(p)}
}

// SessionName collapses everything outside [A-Za-z0-9_] to one dash, so "my
// project" and "my/project" cannot produce the same name. Zellij uses the
// result as a cache directory.
func (Zellij) SessionName(dir string) string {
	var b strings.Builder
	for _, r := range filepath.Base(filepath.Clean(dir)) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			if !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "bothy"
	}
	return "bothy-" + name
}

// Open writes the rendered layout where zellij reads it, then attaches. The
// file is regenerated every launch; editing it does nothing.
func (z Zellij) Open(r Request) error {
	if n, ok := attachedClients(r.Bin, r.Env, r.Session, r.Live); ok && n > 0 {
		return InUse(r.Session)
	}

	kdl, err := render(r.Profile, r.Commands)
	if err != nil {
		return err
	}
	dir := filepath.Join(z.Dir(r.Platform), "layouts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file := filepath.Join(dir, r.Profile.Name+".kdl")
	if err := os.WriteFile(file, []byte(kdl), 0o644); err != nil {
		return err
	}
	if err := os.Chdir(r.Dir); err != nil {
		return err
	}
	if !slices.Contains(r.Live, r.Session) {
		z.discardDead(r.Bin, r.Env, r.Session)
	}
	return runReplacing(r.Env, r.Bin, z.launchArgs(r.Session, file, r.Live)...)
}

// launchArgs must not carry --layout into a live session: zellij applies one
// to an existing session by adding a tab, growing the workspace instead of
// returning to it.
func (Zellij) launchArgs(session, layoutFile string, live []string) []string {
	if slices.Contains(live, session) {
		return []string{"attach", session}
	}
	return []string{"--layout", layoutFile, "attach", "--create", session}
}

// discardDead removes a stopped session. Attaching to an EXITED one
// resurrects it with commands suspended behind "Waiting to run" and a changed
// profile ignored. Errors are ignored: usually there is no such session. No
// --force, so one somehow still running is refused rather than killed.
func (Zellij) discardDead(bin string, env []string, session string) {
	cmd := exec.Command(bin, "delete-session", session)
	cmd.Env = env
	_ = cmd.Run()
}

func (Zellij) CurrentSession() string { return os.Getenv("ZELLIJ_SESSION_NAME") }

func (Zellij) Clients(bin string, env []string, session string, live []string) (int, bool) {
	return attachedClients(bin, env, session, live)
}

// InUse refuses a launch into a session someone is already looking at. Joining
// makes a second client, and zellij sizes a session to its smallest one -- the
// workspace shrinks to this window and a fullscreen TUI overprints itself.
// zellij offers no way to displace a client, so the only cure is not joining.
func InUse(session string) error {
	return fmt.Errorf("%s is already open in another terminal\n"+
		"      zellij sizes a session to its smallest client, so joining from\n"+
		"      here would shrink it. Close it there, or run 'bothy attach %s'\n"+
		"      to join anyway.", session, session)
}

// attachedClients counts who is looking at a session -- a header then a row per
// client id -- reporting false when it could not find out, since a launch is
// not worth blocking on a probe. A var so a test can answer without a zellij;
// only live sessions are asked, as list-clients on a dead one never returns.
var attachedClients = func(bin string, env []string, session string, live []string) (int, bool) {
	if !slices.Contains(live, session) {
		return 0, true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "action", "list-clients")
	// list-clients takes no --session; the name comes from the environment.
	cmd.Env = append(append([]string{}, env...), "ZELLIJ_SESSION_NAME="+session)
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	return countClients(string(out)), true
}

func countClients(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if _, err := strconv.Atoi(f[0]); err == nil {
			n++
		}
	}
	return n
}

// Live reports the sessions that are running. `--short` is not the flag for
// that: it prints stopped ones too, minus the marker that tells them apart.
func (Zellij) Live(bin string, env []string) []string {
	cmd := exec.Command(bin, "list-sessions", "--no-formatting")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return nil // exit 1, "No active zellij sessions found.", is the empty case
	}
	return liveSessions(string(out))
}

// liveSessions reads `<name> [Created ...]` lines, marked EXITED where the
// session has stopped. The bracket rejects the prose zellij also writes here.
func liveSessions(out string) []string {
	var live []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || !strings.HasPrefix(f[1], "[") {
			continue
		}
		if strings.Contains(line, "EXITED") {
			continue
		}
		live = append(live, f[0])
	}
	return live
}

// MinGraphics is the first zellij that renders images correctly: 0.45.0 added
// the protocol, 0.45.1 fixed image sizing on startup.
var MinGraphics = probe.Version{Major: 0, Minor: 45, Patch: 1}

// Graphics gates on the version: below MinGraphics zellij mangles its reply to
// yazi's capability query, and the reply is parsed as keystrokes.
func (Zellij) Graphics(bin string) (bool, string) {
	v, err := probe.ToolVersion(bin)
	if err != nil {
		return false, fmt.Sprintf("could not determine the zellij version (%v), "+
			"so assuming it cannot pass the Kitty graphics protocol through", err)
	}
	if v.Less(MinGraphics) {
		return false, fmt.Sprintf("zellij %s cannot pass the Kitty graphics protocol "+
			"through; %s or newer can", v, MinGraphics)
	}
	return true, "zellij " + v.String() + " implements the Kitty graphics protocol"
}

// CheckConfig runs `zellij setup --check`, which parses the config and reports
// what it rejected.
func (Zellij) CheckConfig(bin string, env []string) (string, error) {
	cmd := exec.Command(bin, "setup", "--check")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Panes asks zellij what it built. `action dump-layout` is documented; the
// session_info cache holding the same KDL is private.
func (Zellij) Panes(bin, session string, env []string) (int, bool) {
	cmd := exec.Command(bin, "action", "dump-layout")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	return Zellij{}.countPanes(string(out))
}

// countPanes counts the panes carrying a command, encoding three facts: only
// the first tab counts because new_tab_template repeats it, floating panes are
// zellij's own tip window, and a plugin pane declares its plugin on the next
// line.
func (Zellij) countPanes(kdl string) (int, bool) {
	lines := strings.Split(kdl, "\n")

	inTab, depth, floatingDepth := false, 0, -1
	count := 0
	for i, raw := range lines {
		line := strings.TrimSpace(raw)

		if !inTab {
			if strings.HasPrefix(line, "tab ") || line == "tab {" {
				inTab, depth = true, 0
			} else {
				continue
			}
		}

		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")

		if strings.HasPrefix(line, "floating_panes") && floatingDepth < 0 {
			floatingDepth = depth
		}

		if floatingDepth < 0 && strings.HasPrefix(line, "pane") {
			isPlugin := i+1 < len(lines) && strings.Contains(lines[i+1], "plugin location=")
			hasCommand := strings.Contains(line, "command=")
			hasName := strings.Contains(line, "name=")
			if !isPlugin && (hasCommand || hasName) {
				count++
			}
		}

		depth += opens - closes
		if floatingDepth >= 0 && depth <= floatingDepth {
			floatingDepth = -1
		}
		if depth <= 0 {
			break // end of the first tab
		}
	}

	if count == 0 {
		return 0, false
	}
	return count, true
}
