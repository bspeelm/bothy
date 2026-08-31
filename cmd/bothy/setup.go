package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/install"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/tools"
)

// setupOptions controls one install pass.
type setupOptions struct {
	DryRun  bool
	Offline bool
	// Quiet drops the file-by-file listing, for the first-run path where the
	// point is a working workspace rather than an inventory of it.
	Quiet bool
}

// setup installs tools and writes configs. Shared by `bothy install` and by
// the first run of `bothy`, so the two cannot drift into doing different work.
func setup(p platform.Info, cfg config.Config, opts setupOptions) error {
	// Tools first: the graphics probe that decides how yazi.toml is written
	// asks the multiplexer its version, so a zellij fetched now is the one the
	// config is rendered for.
	if !opts.DryRun {
		treport, err := install.EnsureTools(p, cfg, opts.Offline,
			func(line string) { fmt.Println(line) })
		if err != nil {
			return err
		}
		printTools(treport)
	}

	res, err := install.Run(p, cfg, install.Options{DryRun: opts.DryRun, Offline: opts.Offline})
	if err != nil {
		return err
	}

	if opts.Quiet {
		fmt.Printf("wrote %d file(s) under %s\n", len(res.Written), tilde(res.Root, p.Home))
	} else {
		verb := "wrote"
		if opts.DryRun {
			verb = "would write"
		}
		fmt.Printf("%s %d file(s), %d already current, under %s\n",
			verb, len(res.Written), len(res.Unchanged), tilde(res.Root, p.Home))
		for _, f := range res.Written {
			fmt.Printf("  + %s\n", tilde(f, p.Home))
		}
	}

	if pr := res.Plugins; pr != nil {
		for _, pl := range pr.Installed {
			fmt.Printf("  + yazi plugin %s — %s\n", pl.Name, pl.Gives)
		}
		for _, f := range pr.Failed {
			fmt.Printf("  ! yazi plugin %s unavailable (%v) — %s is off\n",
				f.Plugin.Name, f.Err, f.Plugin.Gives)
		}
	}

	fmt.Printf("\nimage previews: %v — %s\n", res.Data.ImagePreviews, res.Data.GraphicsReason)
	fmt.Println("nothing outside that directory was touched.")
	return nil
}

// ensureInstalled sets the workspace up if this is the first run.
//
// bothy used to refuse here and tell you to type `bothy install` — which it
// already knew it needed, having just checked. Two commands to a working
// workspace when one would do; `flatpak install` does not ask you to run
// `flatpak configure` afterwards. `bothy install` still exists, because
// re-applying after `bothy config set` is a real thing to want.
func ensureInstalled(p platform.Info, cfg config.Config) error {
	if _, err := os.Stat(p.ConfigRoot()); err == nil {
		return nil
	}

	fmt.Println("first run: setting up your workspace")
	ok, err := confirmDownloads(p, cfg)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cancelled — run 'bothy install --offline' to set up " +
			"without downloading anything")
	}
	if err := setup(p, cfg, setupOptions{Quiet: true}); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// confirmDownloads asks before pulling tens of megabytes, when there is
// somebody there to ask. A desktop launcher cannot answer a prompt, and
// neither can a script, so both proceed.
func confirmDownloads(p platform.Info, cfg config.Config) (bool, error) {
	names, err := tools.Required(cfg.Slots.Mux, cfg.Slots.Browser, cfg.Extras)
	if err != nil {
		return false, err
	}
	decisions, err := tools.ResolveAll(names)
	if err != nil {
		return false, err
	}

	var wanted []tools.Decision
	for _, d := range decisions {
		if d.Action == tools.Fetch {
			wanted = append(wanted, d)
		}
	}
	if len(wanted) == 0 || !isTerminal(os.Stdin) {
		return true, nil
	}

	fmt.Printf("  %d tool(s) already on your system are fine\n", len(decisions)-len(wanted))
	for _, d := range wanted {
		fmt.Printf("  ↓ %s — %s\n", d.Tool.Name, d.Reason)
	}
	fmt.Print("proceed? [Y/n] ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return true, nil // no answer available; carry on
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true, nil
	}
	return false, nil
}
