package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/layout"
)

// The half of `bothy connect` that starts nothing: what to mount, what each
// pane runs, and how a path here maps to a path over there. Kept apart so the
// whole shape is testable on a machine with no sshfs and no network.
//
// Nothing in this file may put anything on the far machine (ADR-046). It
// builds one sshfs invocation and one ssh invocation, and neither copies.

// hostArg refuses only what ssh would misread as an option. Aliases from
// ~/.ssh/config, user@host, bracketed IPv6, ports and ProxyJump chains are all
// ssh's business: bothy parsing that file would be a second and worse answer
// to a question ssh already answers.
func hostArg(s string) (string, error) {
	switch {
	case strings.TrimSpace(s) == "":
		return "", fmt.Errorf("usage: bothy connect <host>")
	case strings.HasPrefix(s, "-"):
		return "", fmt.Errorf("%q starts with a dash, which ssh would read as an option", s)
	case strings.ContainsAny(s, " \t\n"):
		return "", fmt.Errorf("%q has whitespace in it, which is not a host", s)
	}
	return s, nil
}

// mountPoint is where a machine's root appears on this one. Inside the cache,
// so `bothy uninstall` takes it away with everything else.
func mountPoint(cacheDir, host string) string {
	return filepath.Join(cacheDir, "remotes", host)
}

// remotePath turns a path under the mount back into the path the far machine
// knows it by.
//
// The mount holds the remote root, so this is arithmetic and not a lookup --
// which is the whole reason the shim can be three lines and the agent never
// has to reason about it. Outside the mount there is no answer, and saying so
// beats returning something that looks like one.
func remotePath(mount, local string) (string, bool) {
	rel, err := filepath.Rel(mount, local)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if rel == "." {
		return "/", true
	}
	return "/" + filepath.ToSlash(rel), true
}

// localPath is remotePath backwards: where a path on the far machine appears
// here. The default remote directory is "~", which only that machine can
// expand, so an unexpanded one maps to the mount root and yazi opens there.
func localPath(mount, remote string) string {
	if remote == "" || remote == "~" || strings.HasPrefix(remote, "~/") {
		return mount
	}
	return filepath.Join(mount, remote)
}

// mountArgs is the sshfs invocation.
//
// reconnect and the keepalives are what turn a dropped network into a mount
// that comes back rather than a pane wedged forever. The whole remote root is
// mounted, not the working directory, so remotePath stays arithmetic when
// somebody navigates out of it in yazi.
func mountArgs(host, identity, mount string) []string {
	// ConnectTimeout, or an unreachable host is not an error but a wait.
	// Measured against an address that cannot route: 45 seconds and still
	// going, silently, which reads as bothy hanging rather than the host being
	// unreachable. With it, ten seconds and sshfs says why.
	opts := "reconnect,ServerAliveInterval=15,ServerAliveCountMax=3,ConnectTimeout=10"
	if identity != "" {
		// Forwarded to ssh, which takes any config keyword after -o. Choosing a
		// key is not weakening verification; StrictHostKeyChecking and friends
		// would be, and are never passed (ADR-046).
		opts += ",IdentityFile=" + identity
	}
	return []string{host + ":/", mount, "-o", opts}
}

// unmountArgs releases the mount, and unmountLazyArgs is the fallback.
//
// Plain first: a lazy unmount detaches the directory but leaves sshfs running
// until every reference to it drops, which was measured leaving a process
// behind after the workspace had gone. Lazy is what a hung connection needs,
// where a plain unmount fails and leaves the directory unusable -- so it is
// the second attempt rather than the first.
func unmountArgs(mount string) []string     { return []string{"-u", mount} }
func unmountLazyArgs(mount string) []string { return []string{"-u", "-z", mount} }

// alreadyMounted reports whether something is mounted there. A workspace that
// was killed never ran its unmount, and the next connect must not stack a
// second sshfs on top of the first.
func alreadyMounted(mount string) bool {
	body, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return false
	}
	return strings.Contains(string(body), " "+mount+" ")
}

// shellCommand is what the shell pane runs: a login session on the far
// machine, which is what you would type by hand.
func shellCommand(host, identity, remoteDir string) string {
	cmd := "ssh"
	if identity != "" {
		cmd += " -i " + shellQuote(identity)
	}
	// -t because a login shell wants a terminal, and the remote cd happens
	// inside the shell so a directory that is gone leaves you logged in rather
	// than dropping the connection.
	return fmt.Sprintf("%s -t -- %s %s", cmd, shellQuote(host),
		shellQuote(fmt.Sprintf("cd %s 2>/dev/null; exec $SHELL -l", shQuote(remoteDir))))
}

// withRemoteShell puts that session in every pane that would otherwise be a
// plain local shell. A pane with no slot and no command is the side pane, and
// over a connect the side pane belongs on the far machine.
func withRemoteShell(prof layout.Profile, command string) layout.Profile {
	rows := make([]layout.Row, len(prof.Rows))
	copy(rows, prof.Rows)
	for i, r := range rows {
		panes := make([]layout.Pane, len(r.Panes))
		copy(panes, r.Panes)
		for j, pane := range panes {
			if pane.Slot == "" && pane.Command == "" {
				panes[j].Command = command
			}
		}
		rows[i].Panes = panes
	}
	prof.Rows = rows
	return prof
}

// connectEnv tells the panes where they are. The agent reads these: without
// them it knows only a local path, and an agent that reasons "I am in $(pwd),
// so ssh host \"cd $(pwd)\"" names a directory the far machine does not have.
func connectEnv(host, remoteDir, mount string) []string {
	return []string{
		"BOTHY_REMOTE=" + host,
		"BOTHY_REMOTE_DIR=" + remoteDir,
		"BOTHY_REMOTE_MOUNT=" + mount,
	}
}

// shimScript is `on`, which runs a command on the far machine in the directory
// the workspace is open on. Written into bothy's own bin on this machine and
// never copied anywhere -- it is three lines of shell whose whole job is to
// spare the agent the prefix arithmetic.
func shimScript() string {
	return `#!/bin/sh
# bothy: run a command on the machine this workspace is connected to.
# Written by 'bothy connect'; removed with the rest of bothy's directory.
[ -n "$BOTHY_REMOTE" ] || { echo "on: not connected to anything" >&2; exit 1; }
exec ssh -t -- "$BOTHY_REMOTE" "cd ${BOTHY_REMOTE_DIR:-.} && $*"
`
}

// sessionFor names the session, host and all.
//
// The multiplexer names a session after the directory's last component, and
// the mount's last component is whatever the far machine calls that directory.
// Without the host in the name, a project called api over there and one called
// api here are one session in `bothy ls`, and the second launch joins the
// first by accident.
func sessionFor(host, remoteDir string) string {
	base := strings.Trim(filepath.Base(strings.TrimSuffix(remoteDir, "/")), "~/.")
	if base == "" {
		return "bothy-" + sanitise(host)
	}
	return "bothy-" + sanitise(host) + "-" + sanitise(base)
}

// sanitise keeps a session name to what every multiplexer accepts, matching
// what mux.SessionName does to a directory.
func sanitise(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// shQuote wraps a value for the remote shell, which is not this shell.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// remoteNote is what the agent is told about the workspace it is opening in.
//
// Without it the agent has to work this out by experiment, and gets it wrong:
// it runs `nproc` and describes the local machine, or reads /etc/os-release
// and describes it as though it were the far one. Both were observed.
func remoteNote(host, remoteDir, mount string) string {
	return fmt.Sprintf(
		"This workspace is connected to the machine %q over SSH.\n"+
			"You are NOT running on %s. Your process runs on the local machine.\n"+
			"The working directory is an sshfs mount of %s's whole filesystem, so "+
			"everything under %s belongs to %s. Absolute paths like /etc and /home "+
			"are this machine's: %s's /etc is %s/etc.\n"+
			"To run a command on %s, use `on <command>` -- it runs in %s there. "+
			"Reading and writing files under the mount needs no ssh; they are %s's already.",
		host, host, host, mount, host, host, mount, host, remoteDir, host)
}

// agentWithNote is the agent command carrying that note, or the plain command
// when the provider declares no way to take one.
func agentWithNote(cfg config.Config, host, remoteDir, mount string) string {
	agent := install.AgentBinary(cfg.Slots.Agent)
	flag := install.AgentContextFlag(cfg.Slots.Agent)
	if flag == "" {
		return agent
	}
	return agent + " " + flag + " " + shellQuote(remoteNote(host, remoteDir, mount))
}

// sshUser is who ssh would log in as, from ssh's own resolution of the host.
//
// `bothy connect 10.0.0.5` uses the local username, which is right on your own
// machines and wrong on somebody else's -- and the failure that follows looks
// like the host being down rather than the account being wrong. Saying it
// first costs one local call and no round trip.
func sshUser(host string) string {
	out, err := exec.Command("ssh", "-G", "--", host).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if u, ok := strings.CutPrefix(line, "user "); ok {
			return strings.TrimSpace(u)
		}
	}
	return ""
}
