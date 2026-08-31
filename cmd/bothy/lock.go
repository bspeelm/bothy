package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bothy-dev/bothy/internal/fetch"
	"github.com/bothy-dev/bothy/internal/tools"
)

// cmdLock regenerates bothy.lock.
//
// It is a maintainer command, run in the repository and committed. It is
// deliberately not part of `bothy install`: an installer that quietly moves
// its own pins produces builds nobody can reproduce, and a checksum that
// updates itself verifies nothing.
func cmdLock(args []string) error {
	fs := flag.NewFlagSet("lock", flag.ExitOnError)
	path := fs.String("out", fetch.LockPath, "lockfile to write")
	only := fs.String("tool", "", "refresh a single tool")
	if err := fs.Parse(args); err != nil {
		return err
	}

	all, err := tools.Load()
	if err != nil {
		return err
	}
	lock, err := fetch.LoadLockFile(*path)
	if err != nil {
		return err
	}

	fmt.Println("downloading assets to compute checksums; this takes a few minutes")
	progress := func(s string) { fmt.Println(s) }

	var failed int
	for _, t := range all {
		if *only != "" && t.Name != *only {
			continue
		}
		entry, err := fetch.Relock(t, progress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", t.Name, err)
			failed++
			continue
		}
		if old, ok := lock.Get(t.Name); ok && old.Version != entry.Version {
			fmt.Printf("  %s: %s -> %s\n", t.Name, old.Version, entry.Version)
		}
		lock.Set(entry)
	}

	if err := lock.Save(*path); err != nil {
		return err
	}
	fmt.Printf("\nwrote %s (%d tools", *path, len(lock.Entries))
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println(")")
	fmt.Println("review the diff before committing — these are the binaries bothy will run")
	return nil
}
