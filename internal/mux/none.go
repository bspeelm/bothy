package mux

import "github.com/bspeelm/bothy/internal/platform"

// None is the second implementation. `slots.mux = "none"` is already an
// accepted config, and the case has no layout, no session and nothing to
// count, so an interface it cannot satisfy assumes a session exists.
type None struct{}

func (None) Name() string                               { return "none" }
func (None) Dir(platform.Info) string                   { return "" }
func (None) SessionEnv(platform.Info) map[string]string { return nil }
func (None) SessionName(string) string                  { return "" }
func (None) CurrentSession() string                     { return "" }
func (None) Live(string, []string) []string             { return nil }
func (None) Panes(string, string, []string) (int, bool) { return 0, false }

// Open runs the agent here. No panes, so nothing is rendered.
func (None) Open(r Request) error {
	cmd := r.Commands["agent"]
	if cmd == "" {
		return nil
	}
	return runReplacing(r.Env, cmd)
}
