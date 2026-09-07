package doctor

import (
	"os/exec"

	"github.com/bspeelm/bothy/internal/advice"
	"github.com/bspeelm/bothy/internal/install"
)

// checkSSHFS reports whether `bothy connect` would work, and only once
// somebody has connected somewhere. Like confinement it is opt-in: on a
// machine that has never named another one there is nothing to report, and a
// warning about an unused feature fires on every machine that does not want it.
func checkSSHFS(env Env) Result {
	if r, ok := env.elsewhere(); ok {
		return r
	}
	if len(env.Config.Remotes) == 0 && len(install.ProjectRemotes(env.Platform)) == 0 {
		return skip("no other machines named; 'bothy connect' explains it")
	}
	if _, err := exec.LookPath("sshfs"); err != nil {
		cmd := "install sshfs with your package manager"
		if a, err := advice.Get("sshfs"); err == nil {
			cmd = a.Command(env.Platform)
		}
		return fail("sshfs is missing, so 'bothy connect' cannot show another machine's files",
			"the file browser would have nothing to read", cmd)
	}
	return pass("sshfs is here, so 'bothy connect' can mount another machine")
}
