package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/layout"
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
	const mount = "/home/me/.cache/bothy/remotes/abbey"
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

// "~" is the default and only the far machine can expand it, so it maps to the
// mount root rather than to a directory called "~" that nothing will contain.
func TestAnUnexpandedHomeOpensAtTheMountRoot(t *testing.T) {
	const mount = "/c/remotes/abbey"
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
	got := withRemoteShell(prof, "ssh -t -- abbey 'cd /srv/api; exec $SHELL -l'")

	if c := got.Rows[0].Panes[0].Command; c != "" {
		t.Errorf("the browser pane was rewritten to %q — it reads the mount", c)
	}
	if c := got.Rows[1].Panes[0].Command; c != "" {
		t.Errorf("the agent pane was rewritten to %q — it runs here", c)
	}
	if c := got.Rows[1].Panes[1].Command; !strings.Contains(c, "ssh") || !strings.Contains(c, "abbey") {
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
	remote := sessionFor("abbey", "/srv/api")
	if remote == "bothy-api" {
		t.Fatal("the remote session is named as though it were local")
	}
	if !strings.Contains(remote, "abbey") || !strings.Contains(remote, "api") {
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
		mountArgs("abbey", "/home/me/.ssh/id_abbey", "/c/remotes/abbey"),
		mountArgs("abbey", "", "/c/remotes/abbey"),
		{shellCommand("abbey", "/home/me/.ssh/id_abbey", "/srv/api")},
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
	if !strings.Contains(strings.Join(mountArgs("abbey", "/k", "/m"), " "), "IdentityFile=/k") {
		t.Error("an identity was given and sshfs was not told about it")
	}
	if strings.Contains(strings.Join(mountArgs("abbey", "", "/m"), " "), "IdentityFile") {
		t.Error("no identity was given and one was invented")
	}
}

// ssh reads a leading dash as an option, and everything else is ssh's business
// to interpret, not bothy's to parse.
func TestHostsAreHandedToSSHUnexamined(t *testing.T) {
	for _, ok := range []string{"abbey", "user@1.2.3.4", "[::1]", "abbey.example.com", "1.2.3.4"} {
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
	m := mountPoint(cache, "abbey")
	if !strings.HasPrefix(m, cache+string(filepath.Separator)) {
		t.Errorf("mountPoint() = %q, want it under %q", m, cache)
	}
	if unmount := unmountArgs(m); !strings.Contains(strings.Join(unmount, " "), "-z") {
		t.Error("unmount is not lazy, so a hung connection leaves the directory unusable")
	}
}
