package doctor

import (
	"fmt"
	"os"

	"github.com/bspeelm/bothy/internal/install"
)

// checkLayoutBuilt compares the layout the multiplexer built against the
// profile. The renderer's golden test covers what bothy emits; this covers
// whether the multiplexer still interprets it the same way. Runs only inside a
// live session; skips everywhere else.
func checkLayoutBuilt(env Env) Result {
	if env.Mux == nil {
		return skip("no multiplexer backend for the configured slot")
	}
	session := env.Mux.CurrentSession()
	if session == "" {
		return skip("not inside a multiplexer session")
	}
	// Only a session bothy launched: another session's layout has nothing to
	// do with bothy's profile. BOTHY_SESSION is set by SessionEnv.
	if os.Getenv("BOTHY_SESSION") == "" {
		return skip("this session was not launched by bothy")
	}

	got, ok := env.Mux.Panes(env.MuxBin, session, env.ToolEnv)
	if !ok {
		return skip("the multiplexer could not say what it built")
	}

	prof, err := install.LoadProfile(env.Platform, env.ProfileName)
	if err != nil {
		return skip("the profile does not load; see profile-renders")
	}
	want := prof.PaneCount()

	if got != want {
		return warn(fmt.Sprintf("%s built %d panes, the profile describes %d",
			env.Mux.Name(), got, want),
			"the layout was interpreted differently than it was written",
			"if you did not add panes by hand, this is worth reporting — the "+
				"renderer and this "+env.Mux.Name()+" disagree")
	}
	return pass(fmt.Sprintf("%s built the %d panes the profile describes",
		env.Mux.Name(), got))
}
