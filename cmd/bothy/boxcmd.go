package main

import (
	"fmt"
	"os"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
)

// `bothy box` -- which container this project runs in, and what boxes exist.

func cmdBox(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	dir := workspaceDir(p, cfg, cwd)
	if len(args) == 0 {
		return boxHere(p, cfg, dir)
	}
	switch args[0] {
	case "ls":
		return boxList(p, cfg, dir)
	default:
		return fmt.Errorf("usage: bothy box [ls]")
	}
}

// boxHere names the container and the rule that chose it. A launcher that
// picks a container silently is the thing being fixed, so the reason is part
// of the answer. It asks podman nothing, which is why it works everywhere.
func boxHere(p platform.Info, cfg config.Config, dir string) error {
	switch b := install.Resolve(p, cfg, dir); b.Name {
	case "":
		fmt.Printf("%s runs on the host\n", dir)
	default:
		fmt.Printf("%s runs in %s — %s\n", dir, b.Name, b.Reason)
	}
	return nil
}

// boxList shows every box and the sessions in it, marking this project's.
func boxList(p platform.Info, cfg config.Config, dir string) error {
	boxes, err := listBoxes(p)
	if err != nil {
		return err
	}
	if len(boxes) == 0 {
		fmt.Println("no toolboxes on this machine")
		return nil
	}
	backend, bin, err := muxPath(p, cfg)
	if err != nil {
		return err
	}
	where := map[string]string{}
	for _, s := range backend.Live(bin, install.SessionEnv(p, cfg)) {
		where[s] = mux.ServerBox(cfg.Slots.Mux, s)
	}
	fmt.Print(renderBoxes(boxes, where, install.Resolve(p, cfg, dir).Name))
	return nil
}
