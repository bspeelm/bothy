package doctor

// checkPaneFields asks whether the multiplexer still describes its panes in the
// shape bothy reads. The field names are the multiplexer's, and a rename decodes
// without error, so the pinned version proves nothing about the one in use --
// slots set a version floor and no ceiling.
func checkPaneFields(env Env) Result {
	if env.Mux == nil {
		return skip("no multiplexer backend for the configured slot")
	}
	session := env.Mux.CurrentSession()
	if session == "" {
		return skip("not inside a multiplexer session")
	}
	panes, ok := env.Mux.PanesOf(env.MuxBin, session, env.ToolEnv)
	if !ok {
		return fail(env.Mux.Name()+" describes its panes in a shape bothy cannot read",
			"bothy tower finds each agent by reading the pane list",
			"this "+env.Mux.Name()+" is newer than any bothy has read; report it")
	}
	for _, p := range panes {
		if !p.Plugin && p.Command != "" && p.Dir != "" {
			return pass(env.Mux.Name() + " reports what each pane runs and where")
		}
	}
	return skip("no pane in this session runs a command")
}
