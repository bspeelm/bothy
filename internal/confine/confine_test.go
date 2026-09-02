package confine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
)

func sandbox(t *testing.T) platform.Info {
	t.Helper()
	home := t.TempDir()
	return platform.Info{Home: home, DataDir: filepath.Join(home, "share"), OS: "linux"}
}

// The wall is the mount set. If a mount goes missing the agent either cannot
// work or can read the thing this exists to hide, and neither shows up as a
// test failure anywhere else.
func TestTheInvocationMountsTheProjectAndNothingElse(t *testing.T) {
	p := sandbox(t)
	args := Command(p, []string{"podman"}, "img", "/work/project", "claude", nil)
	line := strings.Join(args, " ")

	for _, want := range []string{
		"/work/project:/work:rw", // the project, writable
		"--userns=keep-id",       // as this user, not root
		"label=disable",          // no relabel of the user's files
		"img claude",             // image then agent, in that order
	} {
		if !strings.Contains(line, want) {
			t.Errorf("invocation is missing %q:\n  %s", want, line)
		}
	}
	if strings.Contains(line, p.Home+":") {
		t.Errorf("$HOME is mounted whole, which is the thing the wall is for:\n  %s", line)
	}
}

// Without its own credentials the agent cannot authenticate and the wall
// protects nothing anyone wanted. A path the provider names but the machine
// does not have is not invented either.
func TestCredentialsAreMountedOnlyWhenTheyExist(t *testing.T) {
	p := sandbox(t)
	pr := slots.Provider{Credentials: []string{"~/.claude"}}

	if got := Credentials(p, nil, pr); len(got) != 0 {
		t.Errorf("Credentials = %q for a path that does not exist", got)
	}
	if err := os.MkdirAll(filepath.Join(p.Home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Credentials(p, nil, pr)
	if len(got) != 1 || !strings.HasSuffix(got[0], ".claude") {
		t.Fatalf("Credentials = %q, want the one that exists", got)
	}
	line := strings.Join(Command(p, []string{"podman"}, "i", "/d", "a", got), " ")
	if !strings.Contains(line, ".claude:/agent/.claude:rw") {
		t.Errorf("credentials exist and were not mounted:\n  %s", line)
	}
}

// An agent bothy has not learned the paths for is the user's to name, and
// config wins over the provider so they need not wait for bothy to learn.
func TestConfigOverridesWhatTheProviderDeclares(t *testing.T) {
	p := sandbox(t)
	if err := os.MkdirAll(filepath.Join(p.Home, ".mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	pr := slots.Provider{Credentials: []string{"~/.claude"}}
	got := Credentials(p, []string{"~/.mine"}, pr)
	if len(got) != 1 || !strings.HasSuffix(got[0], ".mine") {
		t.Errorf("Credentials = %q, want the configured path", got)
	}
}

// The recipe is the user's once written: bothy prints it, never builds it, and
// must not undo an edit on the next run.
func TestWriteRecipeDoesNotOverwriteAnEdit(t *testing.T) {
	p := sandbox(t)
	path, err := WriteRecipe(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("FROM mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRecipe(p); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "FROM mine\n" {
		t.Error("a later run overwrote the user's Containerfile")
	}
}

// Inside a toolbox there is no podman and the host's is reachable through
// flatpak-spawn. Without a shared home there is no host to reach.
func TestRuntimeNeedsAHostToHopTo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Runtime(platform.Info{SharedHome: false}); err == nil {
		t.Error("claimed a runtime with no podman and no host")
	}
}
