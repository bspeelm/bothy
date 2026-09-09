package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy tower` -- the effects half: ask the multiplexer what is running, and
// open one window that watches all of it.

func cmdTower(args []string) error {
	fs := flag.NewFlagSet("tower", flag.ExitOnError)
	one := fs.String("mirror", "", "watch one session's agent pane and nothing else")
	every := fs.Duration("every", 2*time.Second, "how often to refresh")
	flat := fs.Bool("no-expand", false, "leave the watched panes at the size they are")
	restore := fs.Bool("restore", false, "collapse expanded agent panes, and open nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, cfg, err := load()
	if err != nil {
		return err
	}
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	env := install.SessionEnv(p, cfg)

	if *one != "" {
		return runMirror(backend, bin, env, cfg, *one, *every)
	}
	return openTower(p, cfg, backend, bin, env, !*flat, *restore)
}

// runMirror prints one session's agent pane until the window closes. The pane
// is looked up every pass, not once: an agent restarted lands in a new pane.
func runMirror(backend mux.Backend, bin string, env []string, cfg config.Config,
	session string, every time.Duration) error {

	agent := install.AgentBinary(cfg.Slots.Agent)
	typed := make(chan string, 1)
	closed := make(chan error, 1)
	go func() { closed <- readLines(os.Stdin, typed) }()

	tick := time.NewTicker(every)
	defer tick.Stop()
	rows := paneRows()
	replyPrompt(os.Stdout, rows)
	last := ""
	for {
		select {
		case err := <-closed:
			// Watching continues; only replying has stopped. A nil channel
			// blocks forever, which says this once rather than every pass.
			fmt.Printf("\n%s: replies are closed (%v); still watching\n", session, err)
			closed = nil
		case line := <-typed:
			relay(backend, bin, env, session, agent, line)
			last = "" // the reply is about to change the screen; do not skip it
			rows = paneRows()
			replyPrompt(os.Stdout, rows)
		case <-tick.C:
			// Asked every pass, not once: fullscreening a mirror makes its pane
			// taller, and a height read at startup leaves the rest of it dead
			// with the reply line stranded where the bottom used to be.
			if r := paneRows(); r != rows {
				rows, last = r, ""
				replyPrompt(os.Stdout, rows)
			}
			screen, err := mirrorOnce(backend, bin, env, session, agent)
			if err != nil {
				// Reported in the pane, not returned: one session ending must
				// not close a pane the other mirrors share a window with.
				fmt.Printf("\n%s: %v\n", session, err)
				continue
			}
			// Only when it changed, which saves the work rather than protecting
			// the reply line -- paint does that by leaving the bottom rows
			// alone. Measured: an idle pane's dump is byte-identical between
			// refreshes, so watching a quiet agent costs one read and no draw.
			if screen != last {
				paint(os.Stdout, screen, rows)
				last = screen
			}
		}
	}
}

// readLines carries what someone typed to the loop that paints, from a
// goroutine because the scanner blocks until Enter.
// readLines delivers what someone typed, one message per Enter, joining the
// lines of a message held open with a trailing backslash.
//
// A Reader rather than a Scanner: a Scanner caps a line at 64KB and then stops
// scanning at all, so one oversized paste ended replies for the rest of the
// mirror's life without saying so. Measured: a 70KB line through a Scanner
// delivered zero lines, the short line after it included.
func readLines(r io.Reader, out chan<- string) error {
	br := bufio.NewReader(r)
	var held []string
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		trimmed, more := continues(line)
		switch {
		case err != nil && line == "" && len(held) == 0:
			// Input closed with nothing typed. Sending here would press Enter
			// at the agent on the way out.
		case more && err == nil:
			held = append(held, trimmed)
		default:
			// A bare Enter sends an empty message on purpose: it is how a
			// prompt's default is accepted.
			out <- strings.Join(append(held, trimmed), "\n")
			held = nil
		}
		if err != nil {
			return err
		}
	}
}

// relay hands a typed line to the agent this pane mirrors. It arrives from the
// keyboard and is passed on unexamined: bothy relays, it does not speak.
func relay(backend mux.Backend, bin string, env []string, session, agent, line string) {
	// Both refusals are reported. A reply that vanishes silently reads as an
	// agent ignoring you, and the pane is the only place the person who typed
	// it is looking.
	panes, ok := backend.PanesOf(bin, session, env)
	if !ok {
		fmt.Printf("\n%s: cannot read the session's panes; nothing was sent\n", session)
		return
	}
	pane, found := agentPane(panes, agent)
	if !found {
		fmt.Printf("\n%s: no live agent pane; nothing was sent\n", session)
		return
	}
	if err := backend.Send(bin, session, pane.Addr(), line, env); err != nil {
		fmt.Printf("\n%s: %v\n", session, err)
	}
}

// mirrorOnce is one refresh: find the agent's pane, read it.
func mirrorOnce(backend mux.Backend, bin string, env []string, session, agent string) (string, error) {
	panes, ok := backend.PanesOf(bin, session, env)
	if !ok {
		return "", fmt.Errorf("gone, or the multiplexer stopped answering")
	}
	pane, found := agentPane(panes, agent)
	if !found {
		return "", fmt.Errorf("no %s pane in this session", agent)
	}
	return backend.Screen(bin, session, pane.Addr(), env)
}

// openTower builds the window and launches it.
func openTower(p platform.Info, cfg config.Config, backend mux.Backend, bin string,
	env []string, expand, restore bool) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("tower: cannot find this binary to run it again: %w", err)
	}
	live := backend.Live(bin, env)
	panesOf := func(session string) ([]mux.PaneRef, bool) {
		return backend.PanesOf(bin, session, env)
	}
	agent := install.AgentBinary(cfg.Slots.Agent)
	mirrors := watchable(panesOf, agent, live)
	if len(mirrors) == 0 {
		return fmt.Errorf("no sessions with an agent to watch\n" +
			"      the tower shows agent panes of running sessions; 'bothy ls' lists them")
	}

	// A tower already running is stale: its panes mirror what was there when it
	// opened, and Open would attach to that rather than build the layout just
	// computed. Nothing in a tower is worth keeping, so it is replaced.
	if slices.Contains(live, towerSession) {
		_ = backend.Kill(bin, env, towerSession)
		live = backend.Live(bin, env)
	}

	if restore {
		fmt.Printf("collapsed %d pane(s)\n", toggle(backend, bin, env, collapsible(mirrors, panesOf, agent)))
		return nil
	}

	// Expanded before the layout is built, because what a mirror can show is
	// exactly what its pane displays and expanding changes that: 57 columns
	// against 191, measured. The sizes are read again afterwards so the layout
	// is decided on what the mirrors will actually be.
	var expanded []mirror
	if expand {
		if expanded = expandable(mirrors); len(expanded) > 0 {
			toggle(backend, bin, env, expanded)
			mirrors = watchable(panesOf, agent, live)
		}
	}
	prof := towerProfile(mirrors, self, envInt(env, "COLUMNS"))
	// Which arrangement it chose. A mirror cannot be made wider than the pane
	// it watches, so stacking is what a window too narrow to hold them all
	// looks like, and saying so beats leaving it to be guessed at.
	shape := "stacked"
	if len(prof.Rows) == 1 {
		shape = "side by side"
	}
	fmt.Printf("watching %d agent(s), %s; %d pane(s) expanded to be worth reading\n",
		len(mirrors), shape, len(expanded))
	defer watching(p, cfg, towerSession)()
	err = backend.Open(mux.Request{
		Platform: p, Bin: bin, Session: towerSession, Dir: p.Home,
		Profile: prof, Commands: install.Commands(cfg), Env: env, Live: live,
	})
	toggle(backend, bin, env, collapsible(expanded, panesOf, agent))
	return err
}

// toggle flips each pane's fullscreen state and counts the ones that took it.
// Errors are dropped: a session that ended while the tower was open is the
// common case, and it is not worth a message on the way out.
func toggle(backend mux.Backend, bin string, env []string, panes []mirror) int {
	n := 0
	for _, m := range panes {
		if backend.Expand(bin, m.Session, m.Pane, env) == nil {
			n++
		}
	}
	return n
}

// envInt reads a numeric variable out of the session environment, 0 when it is
// absent. COLUMNS is unset rather than guessed when nothing could be measured
// (ADR-022's neighbour in SessionEnv), and 0 is what the layout treats as
// "do not assume it fits".
func envInt(env []string, key string) int {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			// A value that is not a number reads as 0, the same as absent:
			// either way the width is unknown and nothing is assumed to fit.
			n, _ := strconv.Atoi(v)
			return n
		}
	}
	return 0
}

// paneRows is how tall this pane is, 0 when it cannot be found out.
//
// stty rather than an ioctl, which would need unsafe and a constant that differs
// between Linux and macOS; stty is in coreutils and present even in a minimal
// build root. A pane that will not say its size is painted whole.
func paneRows() int {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return 0
	}
	n, _ := strconv.Atoi(f[0])
	return n
}
