package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/platform"
)

// The requirement, as a test: nothing bothy does may put a file on the far
// machine. A source fence rather than a comment, because this is the kind of
// thing a later convenience quietly reintroduces.
func TestConnectPutsNothingOnTheRemote(t *testing.T) {
	banned := []string{"scp", "rsync", "curl", "wget", "install.sh", "sftp -b"}
	for _, f := range []string{"connect.go", "connectcmd.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			continue // connectcmd.go arrives in the same change; absence is not a pass
		}
		for _, b := range banned {
			if strings.Contains(string(src), b) {
				t.Errorf("%s mentions %q — nothing may be placed on the remote machine", f, b)
			}
		}
	}
}

// Everything the agent does rests on this being arithmetic. The mount holds
// the remote root, so stripping it gives the path the far machine knows.
func TestTheMountIsAPurePrefixOfTheRemotePath(t *testing.T) {
	const mount = "/home/me/.cache/bothy/remotes/<client>"
	cases := []struct {
		local, want string
		ok          bool
	}{
		{mount + "/srv/api", "/srv/api", true},
		{mount + "/srv/api/cmd/server", "/srv/api/cmd/server", true},
		{mount, "/", true},
		{"/home/me/elsewhere", "", false},
		{"/", "", false},
	}
	for _, tc := range cases {
		got, ok := remotePath(mount, tc.local)
		if got != tc.want || ok != tc.ok {
			t.Errorf("remotePath(%q) = %q, %v; want %q, %v", tc.local, got, ok, tc.want, tc.ok)
		}
	}
}

// The default is the whole machine, so a connect lands somewhere that has
// something in it. A remote home is usually ten dotfiles and nothing else,
// which reads as a connection that did not work.
func TestTheDefaultIsTheWholeMachine(t *testing.T) {
	const mount = "/c/remotes/<client>"
	if got := localPath(mount, "/"); got != mount {
		t.Errorf("localPath(/) = %q, want the mount root", got)
	}
	// sanitise() keeps only what a multiplexer accepts, so the angle brackets
	// of the placeholder host come off and "client" is what is left.
	if got := sessionFor("<client>", "/"); got != "bothy-client" {
		t.Errorf("sessionFor(<client>, /) = %q, want bothy-client", got)
	}
}

// A ~ can still be typed, and only the far machine can expand it — so it maps
// to the mount root rather than to a directory called "~" that nothing has.
func TestAnUnexpandedHomeOpensAtTheMountRoot(t *testing.T) {
	const mount = "/c/remotes/<client>"
	for _, remote := range []string{"", "~", "~/src/api"} {
		if got := localPath(mount, remote); got != mount {
			t.Errorf("localPath(%q) = %q, want the mount root", remote, got)
		}
	}
	if got := localPath(mount, "/srv/api"); got != mount+"/srv/api" {
		t.Errorf("localPath(/srv/api) = %q", got)
	}
}

// The side pane is the one with no slot and no command. Over a connect it
// belongs on the far machine, and the other two panes must not be touched:
// yazi reads the mount and the agent runs here.
func TestAllThreePanesWorkOnTheRemote(t *testing.T) {
	prof := layout.Profile{Rows: []layout.Row{
		{Panes: []layout.Pane{{Slot: "browser"}}},
		{Panes: []layout.Pane{{Slot: "agent"}, {Name: "side"}}},
	}}
	got := withRemoteShell(prof, "ssh -t -- <client> 'cd /srv/api; exec $SHELL -l'")

	if c := got.Rows[0].Panes[0].Command; c != "" {
		t.Errorf("the browser pane was rewritten to %q — it reads the mount", c)
	}
	if c := got.Rows[1].Panes[0].Command; c != "" {
		t.Errorf("the agent pane was rewritten to %q — it runs here", c)
	}
	if c := got.Rows[1].Panes[1].Command; !strings.Contains(c, "ssh") || !strings.Contains(c, "<client>") {
		t.Errorf("the shell pane is %q, want a session on the far machine", c)
	}
	if prof.Rows[1].Panes[1].Command != "" {
		t.Error("withRemoteShell mutated the profile it was given")
	}
}

// A project called api over there and one called api here are two workspaces,
// and a session name that cannot tell them apart makes the second launch join
// the first by accident.
func TestSessionNamesCarryTheHost(t *testing.T) {
	remote := sessionFor("<client>", "/srv/api")
	if remote == "bothy-api" {
		t.Fatal("the remote session is named as though it were local")
	}
	if !strings.Contains(remote, "client") || !strings.Contains(remote, "api") {
		t.Errorf("sessionFor() = %q, want the host and the directory", remote)
	}
	if got := sessionFor("10.0.0.5", "~"); strings.ContainsAny(got, "./~:") {
		t.Errorf("sessionFor() = %q, want only what a multiplexer accepts", got)
	}
}

// Choosing a key is not weakening verification. Disabling host checking is,
// and bothy never does it — at any layer, in either invocation.
func TestBothyChoosesAKeyButNeverWeakensVerification(t *testing.T) {
	weakening := []string{"StrictHostKeyChecking", "UserKnownHostsFile", "PasswordAuthentication", "NoHostAuthenticationForLocalhost"}
	argvs := [][]string{
		mountArgs("<client>", "/home/me/.ssh/id_<client>", "/c/remotes/<client>"),
		mountArgs("<client>", "", "/c/remotes/<client>"),
		{shellCommand("<client>", "/home/me/.ssh/id_<client>", "/srv/api")},
		{shimScript()},
	}
	for _, argv := range argvs {
		joined := strings.Join(argv, " ")
		for _, w := range weakening {
			if strings.Contains(joined, w) {
				t.Errorf("%q contains %q", joined, w)
			}
		}
	}
	if !strings.Contains(strings.Join(mountArgs("<client>", "/k", "/m"), " "), "IdentityFile=/k") {
		t.Error("an identity was given and sshfs was not told about it")
	}
	if strings.Contains(strings.Join(mountArgs("<client>", "", "/m"), " "), "IdentityFile") {
		t.Error("no identity was given and one was invented")
	}
}

// ssh reads a leading dash as an option, and everything else is ssh's business
// to interpret, not bothy's to parse.
func TestHostsAreHandedToSSHUnexamined(t *testing.T) {
	for _, ok := range []string{"<client>", "user@1.2.3.4", "[::1]", "<client>.example.com", "1.2.3.4"} {
		if _, err := hostArg(ok); err != nil {
			t.Errorf("hostArg(%q) = %v, want it accepted", ok, err)
		}
	}
	for _, bad := range []string{"", "   ", "-oProxyCommand=x", "two hosts"} {
		if _, err := hostArg(bad); err == nil {
			t.Errorf("hostArg(%q) was accepted", bad)
		}
	}
}

// The shim exists so the agent never has to do the prefix arithmetic itself.
// It must name the remote directory, not the mount.
func TestTheShimRunsOnTheBoxInTheRightDirectory(t *testing.T) {
	s := shimScript()
	for _, want := range []string{"BOTHY_REMOTE", "BOTHY_REMOTE_DIR", "ssh", "cd "} {
		if !strings.Contains(s, want) {
			t.Errorf("the shim does not mention %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "BOTHY_REMOTE_MOUNT") {
		t.Error("the shim uses the mount path, which does not exist on the far machine")
	}
	if !strings.Contains(s, "not connected") {
		t.Error("run outside a connect the shim should say so, not ssh to nothing")
	}
}

// Inside the cache, so uninstall removes it and a stale mount never outlives
// the directory bothy owns.
func TestTheMountIsInsideBothysOwnTree(t *testing.T) {
	cache := "/home/me/.local/share/bothy/cache"
	m := mountPoint(cache, "<client>")
	if !strings.HasPrefix(m, cache+string(filepath.Separator)) {
		t.Errorf("mountPoint() = %q, want it under %q", m, cache)
	}
}

// Measured on macOS 25.6 with FUSE-T: fusermount3 does not exist there, both
// unmount attempts exited 127, and every connect leaked its mount until a
// later one was asked to mount over a live one and failed.
func TestEveryPlatformCanReleaseItsOwnMount(t *testing.T) {
	const m = "/c/remotes/<client>"
	for _, goos := range []string{"linux", "darwin"} {
		plain, forced := unmountArgv(goos, m)
		if plain[0] != forced[0] {
			t.Errorf("%s: two unmount programs, %q and %q", goos, plain[0], forced[0])
		}
		if _, err := exec.LookPath(plain[0]); err != nil && goos == runtime.GOOS {
			t.Errorf("%s has no %q, so its mounts are never released", goos, plain[0])
		}
		// Plain first, forceful only as a fallback: reaching for force first was
		// measured tearing a mount out from under a pane still reading it.
		if force(plain) {
			t.Errorf("%s: the first unmount is the forceful one", goos)
		}
		if !force(forced) {
			t.Errorf("%s: no forceful fallback, so a busy mount stays wedged", goos)
		}
	}
}

// Measured mounting onto an occupied mount point: sshfs printed "fuse: mount
// failed with errro: -1" and exited 0, because it forks and the parent knows
// nothing. bothy reported a mount it did not have and opened the workspace on
// an empty local directory.
func TestSSHFSExitCodeIsNotProofOfAMount(t *testing.T) {
	src, err := os.ReadFile("connectcmd.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "err != nil || !mounted(mount)") {
		t.Error("connect trusts sshfs's exit status, which is 0 even when the mount failed")
	}
}

// -z is libfuse's lazy detach, -f is umount's force. Both are "do it anyway".
func force(argv []string) bool {
	for _, a := range argv[1:] {
		if a == "-z" || a == "-f" {
			return true
		}
	}
	return false
}

// Observed before this existed: the agent ran nproc and described the local
// machine, then read /etc/os-release and described it as though it were the
// far one. It has to be told, not left to deduce.
func TestTheAgentIsToldWhichMachineItIsNotOn(t *testing.T) {
	const mount = "/c/remotes/<client>"
	note := remoteNote("<client>", "/srv/api", mount)

	for _, want := range []string{"NOT running on <client>", mount, "on <command>", "/srv/api"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note does not mention %q:\n%s", want, note)
		}
	}

	// An agent whose provider declares no way to take a note gets the plain
	// command, never a flag bothy guessed at.
	cfg := config.Default()
	cfg.Slots.Agent = "aider"
	if got := agentWithNote(cfg, "<client>", "/srv/api", mount); strings.Contains(got, "--") {
		t.Errorf("agentWithNote(aider) = %q, want the bare command", got)
	}
	cfg.Slots.Agent = "claude-code"
	if got := agentWithNote(cfg, "<client>", "/srv/api", mount); !strings.Contains(got, "append-system-prompt") {
		t.Errorf("agentWithNote(claude-code) = %q, want the declared flag", got)
	}
}

// The wall is built by the podman confinement reaches, and an sshfs mount made
// where bothy runs is invisible to it — measured: 24 entries inside the
// toolbox, none from the host, nothing in the host's mount table. The bind
// would succeed and mount an empty directory, walling the agent off from the
// files it was opened for.
func TestConfineRefusesInsideAConnect(t *testing.T) {
	p := platform.Info{Root: "/home/me/.local/share/bothy"}
	mount := filepath.Join(p.CacheDir(), "remotes", "<client>", "srv", "api")

	if err := refuseInsideAConnect(p, mount); err == nil {
		t.Error("confine accepted a directory the container cannot see")
	} else if !strings.Contains(err.Error(), "empty directory") {
		t.Errorf("the refusal does not say what would go wrong: %v", err)
	}
	if err := refuseInsideAConnect(p, "/home/me/code/api"); err != nil {
		t.Errorf("confine refused an ordinary local project: %v", err)
	}
}

// An unreachable host was not an error but a wait: measured at 45 seconds and
// still going, silently, which reads as bothy hanging rather than the machine
// being unreachable.
func TestAnUnreachableHostFailsRatherThanHangs(t *testing.T) {
	opts := strings.Join(mountArgs("<client>", "", "/m"), " ")
	if !strings.Contains(opts, "ConnectTimeout=") {
		t.Error("no ConnectTimeout, so an unreachable host hangs until the kernel gives up")
	}
}

// `bothy connect 10.0.0.5` logs in as the local username, which is right on
// your own machines and wrong on somebody else's — and the failure that
// follows looks like the host being down rather than the account being wrong.
func TestTheAccountIsNamedBeforeConnecting(t *testing.T) {
	if got := orLocalUser("bryan"); got != "bryan" {
		t.Errorf("orLocalUser(bryan) = %q", got)
	}
	t.Setenv("USER", "someone")
	if got := orLocalUser(""); got != "someone" {
		t.Errorf("orLocalUser() = %q, want this machine's account", got)
	}
	t.Setenv("USER", "")
	if got := orLocalUser(""); got == "" {
		t.Error("orLocalUser() came back empty, so the message would name nobody")
	}
}
