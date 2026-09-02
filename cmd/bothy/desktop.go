package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bspeelm/bothy/internal/platform"
	"strings"
	"text/template"

	bothy "github.com/bspeelm/bothy"
)

// The desktop entry is the one file bothy writes outside its own tree, because
// it has to live in the desktop's search path to work at all. Hence a separate
// command that says what it is about to do and can be undone.
func cmdDesktop(args []string) error {
	fs := flag.NewFlagSet("desktop-entry", flag.ExitOnError)
	doInstall := fs.Bool("install", false, "write the entry (outside bothy's tree)")
	remove := fs.Bool("remove", false, "delete a previously written entry")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, _, err := load()
	if err != nil {
		return err
	}
	dest := desktopEntryPath(p.DataDir)

	// Removing is allowed anywhere: an older bothy wrote these without asking
	// what platform it was on, and a file it left behind should be removable
	// by the thing that left it.
	if !*remove {
		if err := desktopEntriesBelongHere(p); err != nil {
			return err
		}
	}

	if *remove {
		if err := os.Remove(dest); err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("no entry at %s\n", tilde(dest, p.Home))
				return nil
			}
			return err
		}
		fmt.Printf("removed %s\n", tilde(dest, p.Home))
		return nil
	}

	body, err := renderDesktopEntry(p.LocalBin)
	if err != nil {
		return err
	}

	if !*doInstall {
		fmt.Print(string(body))
		fmt.Fprintf(os.Stderr, "\n# write it with: bothy desktop-entry --install\n"+
			"# it would go to %s, which is outside bothy's tree —\n"+
			"# the desktop has to find it there. 'bothy desktop-entry --remove' undoes it.\n",
			tilde(dest, p.Home))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", tilde(dest, p.Home))
	fmt.Println("this is outside bothy's tree — 'bothy uninstall' will not remove it,")
	fmt.Println("but 'bothy desktop-entry --remove' will.")
	return nil
}

// desktopEntryPath is where the XDG desktop-entry spec says to look.
// desktopEntriesBelongHere reports whether a .desktop file means anything on
// this machine. macOS wants a bundle and Windows a shortcut, and writing an
// inert file reports success while producing nothing. Listed as the two
// platforms known to lack XDG rather than the one known to have it, so a BSD
// keeps working.
func desktopEntriesBelongHere(p platform.Info) error {
	switch p.OS {
	case "darwin":
		return fmt.Errorf("desktop entries are a freedesktop convention and macOS has none\n" +
			"      the nearest equivalent is an application bundle, which bothy does not make\n" +
			"      point your launcher at ~/.local/bin/bothy instead")
	case "windows":
		return fmt.Errorf("desktop entries are a freedesktop convention and Windows has none\n" +
			"      make a shortcut to bothy instead")
	}
	return nil
}

func desktopEntryPath(dataDir string) string {
	return filepath.Join(dataDir, "applications", "bothy.desktop")
}

func renderDesktopEntry(localBin string) ([]byte, error) {
	src, err := bothy.Templates.ReadFile("templates/desktop/bothy.desktop.tmpl")
	if err != nil {
		return nil, err
	}
	t, err := template.New("desktop").Parse(string(src))
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	// An absolute path: a desktop launcher does not get the shell's PATH.
	if err := t.Execute(&sb, struct{ Exec string }{filepath.Join(localBin, "bothy")}); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}
