package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bothy "github.com/bspeelm/bothy"
)

// go:embed skips files whose names begin with an underscore unless the pattern
// says all:, and zsh insists its script be called _bothy. Without the prefix the
// bash script embedded and the zsh one did not, which only showed up on trying
// to install it.
func TestBothCompletionScriptsAreEmbedded(t *testing.T) {
	for shell, want := range map[string]string{"bash": "bothy.bash", "zsh": "_bothy"} {
		body, err := bothy.Completions.ReadFile("completions/" + want)
		if err != nil {
			t.Errorf("%s: %v", shell, err)
			continue
		}
		if len(body) < 200 {
			t.Errorf("%s: the embedded script is %d bytes; it should be the real one", shell, len(body))
		}
	}
}

// The file each shell reads is named by that shell, not chosen: bash matches
// the command name and zsh matches #compdef. Getting either wrong produces a
// file that is installed and silently never loaded.
func TestEachShellGetsTheNameItInsistsOn(t *testing.T) {
	if got := completionFile("bash"); got != "bothy.bash" {
		t.Errorf("bash script is %q", got)
	}
	if got := completionFile("zsh"); got != "_bothy" {
		t.Errorf("zsh script is %q", got)
	}
	if completionShells["zsh"].Name != "_bothy" {
		t.Errorf("zsh is installed as %q; zsh loads _bothy", completionShells["zsh"].Name)
	}
	if completionShells["bash"].Name != "bothy" {
		t.Errorf("bash is installed as %q; bash-completion loads the command's name",
			completionShells["bash"].Name)
	}
}

// bash-completion loads a file named for the command from a data directory it
// searches without being told. Writing it anywhere else installs something that
// never runs.
func TestBashCompletionGoesWhereBashLooks(t *testing.T) {
	dir := completionShells["bash"].Dir
	if dir != filepath.Join("bash-completion", "completions") {
		t.Errorf("bash completion goes to %q, which bash-completion does not search", dir)
	}
}

// The scripts that ship are the ones the packages install, so a change to one
// must reach the other. They are the same file, and this says so.
func TestTheEmbeddedScriptIsTheOneThePackagesInstall(t *testing.T) {
	for _, name := range []string{"bothy.bash", "_bothy"} {
		onDisk, err := os.ReadFile(filepath.Join("..", "..", "completions", name))
		if err != nil {
			t.Fatal(err)
		}
		embedded, err := bothy.Completions.ReadFile("completions/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(onDisk) != string(embedded) {
			t.Errorf("%s embedded differs from the file the packaging installs", name)
		}
	}
}

// The shell is an operand and the flags come after it, which Go's flag package
// will not do on its own: it stops at the first operand, so `completion bash
// --install` parsed neither.
func TestTheShellIsTakenBeforeTheFlags(t *testing.T) {
	body, err := os.ReadFile("completion.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if strings.Index(src, "shell, args = args[0], args[1:]") > strings.Index(src, "fs.Parse(args)") {
		t.Error("flags are parsed before the shell is taken, so `completion bash --install` sets no flag")
	}
}
