package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bspeelm/bothy/internal/confine"
	"github.com/bspeelm/bothy/internal/install"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
)

// cmdConfine launches the workspace with the agent pane in a container. With
// no image built it writes the recipe and prints the command that builds it,
// so nobody needs to know --print exists. bothy does not run the build: that
// installs an agent, which PLAN.md §11 rules out.
func cmdConfine(args []string) error {
	fs := flag.NewFlagSet("confine", flag.ContinueOnError)
	printOnly := fs.Bool("print", false, "show the recipe and the exact command, and run neither")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, cfg, err := load()
	if err != nil {
		return err
	}
	runtime, err := confine.Runtime(p)
	if err != nil {
		return fmt.Errorf("%w\n"+
			"      confinement runs the agent in a container and has nothing to run it with\n"+
			"      install podman, or use 'bothy' to launch unconfined", err)
	}
	if p.OS != "linux" {
		fmt.Fprintln(os.Stderr,
			"bothy: confinement is tested on Linux only; on this platform podman runs a\n"+
				"       Linux VM and the wall is real but differently shaped. Continuing.")
	}

	image := cfg.Agent.Image
	if image == "" {
		image = confine.DefaultImage
	}
	dir, _ := os.Getwd()
	agent := cfg.ProviderOrDefault("agent")
	pr, _ := slots.Get(agent)
	creds := confine.Credentials(p, cfg.Agent.Credentials, pr)
	if len(creds) == 0 {
		fmt.Fprintf(os.Stderr,
			"bothy: no credential paths known for %s, so it will start unable to log in.\n"+
				"       bothy mounts the project and nothing else from $HOME, which is the\n"+
				"       point — but the agent needs its own credentials through that wall.\n"+
				"       Name them and this goes away:\n"+
				"         bothy config set agent.credentials ~/.config/%s\n", agent, agent)
	}
	cmd := strings.Join(confine.Command(p, runtime, image, dir,
		install.AgentBinary(cfg.Slots.Agent), creds), " ")

	if *printOnly {
		recipe, err := confine.Recipe()
		if err != nil {
			return err
		}
		fmt.Printf("%s\nbothy would run:\n\n  %s\n", recipe, cmd)
		return nil
	}
	if !confine.ImageBuilt(runtime, image) {
		return explainTheBuild(p, image)
	}
	return launch(p, cfg, dir, cfg.Profile, cmd)
}

// explainTheBuild writes the recipe and names the command that builds it.
func explainTheBuild(p platform.Info, image string) error {
	path, err := confine.WriteRecipe(p)
	if err != nil {
		return err
	}
	fmt.Printf("bothy: the agent needs an image to run in, and does not have one yet.\n\n"+
		"      bothy wrote the recipe to\n        %s\n\n"+
		"      build it — this is yours to run, not bothy's:\n"+
		"        podman build -t %s %s\n\n"+
		"      then: bothy confine\n", path, image, confine.Dir(p))
	return nil
}
