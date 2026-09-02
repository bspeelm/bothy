package doctor

import (
	"os"

	"github.com/bspeelm/bothy/internal/confine"
)

// checkConfine reports whether `bothy confine` would work, and only once the
// recipe exists. Confinement is opt-in, so before anyone asks for it there is
// nothing to report -- a warning about an unused feature is noise, and it
// would fire on every machine that has never wanted one.
func checkConfine(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if _, err := os.Stat(confine.RecipePath(env.Platform)); err != nil {
		return skip("confinement not set up; 'bothy confine' explains it")
	}
	runtime, err := confine.Runtime(env.Platform)
	if err != nil {
		return fail("'bothy confine' is set up but podman is gone",
			"the agent would run unconfined", "install podman")
	}
	image := env.Config.Agent.Image
	if image == "" {
		image = confine.DefaultImage
	}
	if !confine.ImageBuilt(runtime, image) {
		return warn("'bothy confine' has no image to run the agent in",
			"the agent runs unconfined until one is built",
			"run 'bothy confine' — it writes the recipe and names the build command")
	}
	return pass("'bothy confine' can run the agent in " + image)
}
