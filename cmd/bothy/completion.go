package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy completion` -- the completion scripts, for the ways in that no package
// manager reaches.
//
// dnf, apt and pacman place these files themselves, the way they place the
// licence and the docs. The install script, `go install` and Homebrew's cask do
// not, so those three had nothing. Like the desktop entry, the file has to live
// where another program looks for it, which is outside bothy's tree -- hence a
// command that says what it is about to do and can be undone.

// completionShells is where each shell expects to find the file, under the
// user's data directory. bash reads its path without being told; zsh has no
// per-user default, so its directory has to be named in ~/.zshrc.
var completionShells = map[string]struct{ Dir, Name string }{
	"bash": {"bash-completion/completions", "bothy"},
	"zsh":  {"zsh/site-functions", "_bothy"},
}

func cmdCompletion(args []string) error {
	fs := flag.NewFlagSet("completion", flag.ExitOnError)
	doInstall := fs.Bool("install", false, "write it (outside bothy's tree)")
	remove := fs.Bool("remove", false, "delete a previously written file")
	// The shell is taken before the flags are parsed, because Go's flag package
	// stops at the first operand: `completion bash --install` would otherwise
	// leave both words as arguments and no flag set.
	shell := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		shell, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	where, known := completionShells[shell]
	if !known {
		return fmt.Errorf("usage: bothy completion <bash|zsh> [--install|--remove]")
	}

	p, _, err := load()
	if err != nil {
		return err
	}
	dest := filepath.Join(p.DataDir, where.Dir, where.Name)

	if *remove {
		return removeCompletion(dest, p.Home)
	}
	body, err := bothy.Completions.ReadFile("completions/" + completionFile(shell))
	if err != nil {
		return err
	}
	if !*doInstall {
		fmt.Print(string(body))
		fmt.Fprintf(os.Stderr, "\n# write it with: bothy completion %s --install\n"+
			"# it would go to %s, which is outside bothy's tree --\n"+
			"# the shell has to find it there. '--remove' undoes it.\n",
			shell, tilde(dest, p.Home))
		return nil
	}
	return writeCompletion(p, shell, dest, body)
}

// completionFile is what the script is called in the repository, which is what
// each shell insists on rather than a choice: bash matches the command name and
// zsh matches #compdef.
func completionFile(shell string) string {
	if shell == "zsh" {
		return "_bothy"
	}
	return "bothy.bash"
}

func removeCompletion(dest, home string) error {
	if err := os.Remove(dest); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("no completion at %s\n", tilde(dest, home))
			return nil
		}
		return err
	}
	fmt.Printf("removed %s\n", tilde(dest, home))
	return nil
}

func writeCompletion(p platform.Info, shell, dest string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", tilde(dest, p.Home))
	fmt.Println("this is outside bothy's tree -- 'bothy uninstall' will not remove it,")
	fmt.Printf("but 'bothy completion %s --remove' will.\n", shell)
	// zsh looks only where it has been told to look, and there is no per-user
	// directory it reads by default. Saying so beats a file that silently does
	// nothing.
	if shell == "zsh" {
		fmt.Printf("\nzsh needs that directory on its fpath. In ~/.zshrc, before compinit:\n"+
			"  fpath=(%s $fpath)\n", tilde(filepath.Dir(dest), p.Home))
	}
	return nil
}
