package doctor

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/mux"
)

// A Zellij with the three answers this check asks for replaced. Embedding
// keeps the rest of the interface real, so a method added to Backend does not
// silently start being answered by a stub.
type stubMux struct {
	mux.Zellij
	session string
	clients int
	known   bool
}

func (s stubMux) CurrentSession() string         { return s.session }
func (s stubMux) Live(string, []string) []string { return []string{s.session} }
func (s stubMux) Clients(string, []string, string, []string) (int, bool) {
	return s.clients, s.known
}

// The whole point: a second terminal on one session caps the workspace at the
// smaller of them, and the symptom people see is corrupted output.
func TestOneClientPerSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    mux.Backend
		want Severity
	}{
		{"nobody home", nil, Skip},
		{"outside a session", stubMux{session: ""}, Skip},
		{"the multiplexer will not say", stubMux{session: "s", known: false}, Skip},
		{"one terminal", stubMux{session: "s", clients: 1, known: true}, Pass},
		{"two terminals", stubMux{session: "s", clients: 2, known: true}, Warn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := Env{}
			if tc.m != nil {
				env.Mux = tc.m
			}
			got := checkOneClientPerSession(env)
			if got.Severity != tc.want {
				t.Errorf("severity = %v, want %v (%s)", got.Severity, tc.want, got.Summary)
			}
			if tc.want == Warn && !strings.Contains(got.Summary, "2 terminals") {
				t.Errorf("the warning is %q, which does not say how many", got.Summary)
			}
		})
	}
}
