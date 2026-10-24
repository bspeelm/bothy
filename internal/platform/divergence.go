package platform

// Where a platform cannot be made to behave like the others.
//
// ADR-031 already governs the compile-time half: a build tag only when the
// compiler refuses, and `buildTagged` in the docs test lists every file that
// earns one. This is the runtime half, which was the larger one and had no
// list at all -- a per-OS branch was a comment beside an `if`, and a claim
// about a platform was a paragraph in an ADR that nothing re-read. ADR-059
// said returning from a take-over on Windows was impossible for two releases
// after it stopped being true.
//
// So an entry is not a note. It names the ADR that decided it, the test that
// holds the compensation up, and the day it was last measured, and a test
// fails when any of the three is missing or no longer resolves.

// A Divergence is one place where a platform does not behave as the reference
// does, and what bothy does about it.
type Divergence struct {
	// ID is the name used in the `// divergence: <id>` marker that every
	// branch compensating for this must carry.
	ID string
	// OS is the platform that diverges. Reference platforms never appear.
	OS string
	// Mux narrows the entry to one multiplexer; empty means any.
	Mux string
	// Capability is one of doctor's five, or empty where what diverges is a
	// behaviour rather than something the stack either delivers or does not.
	// A string, not doctor.Capability: doctor imports this package.
	Capability string
	// Reference is what the reference platforms do, and Here is what this one
	// does instead. Both are what was seen, not what was expected.
	Reference string
	Here      string
	// Carried is how bothy compensates. Empty is ADR-054's *cost*: named,
	// reported, and not worked around.
	Carried string
	// ADR decided it; Test holds the compensation up; Measured is when it was
	// last seen to be true, because a workaround nobody has re-measured is a
	// bug waiting to be found rather than a feature.
	ADR      string
	Test     string
	Measured string
}

// divergences is the register. Adding a per-OS branch without adding an entry
// fails TestEveryPlatformBranchIsRegistered; adding an entry whose ADR, test
// or marker does not resolve fails its own guard.
var divergences = []Divergence{{
	ID:        "take-over-descend",
	OS:        "windows",
	Mux:       "zellij",
	Reference: "the host descends into the nested session: one tab bar reading `bothy-tower > bothy-proj-a`, and Ctrl-o ] returns to the tower",
	Here:      "the host does not descend under either nested_session_handling value, keeping its own chrome and keybindings, so the window draws two of everything and Ctrl+q ends the tower",
	Carried:   "the window is handed over whole -- the mirror leaves the session's name under the state directory and ends the tower, and the process that opened the tower attaches to that session in its place",
	ADR:       "ADR-061",
	Test:      "TestTheWindowIsHandedOverWhereTheMultiplexerWillNotDescend",
	Measured:  "2026-10-24, Windows 11 with zellij 0.45.1, and again with two plain sessions and no bothy in the way",
}, {
	ID:         "session-env-case",
	OS:         "windows",
	Capability: "panes",
	Reference:  "environment variable names are compared exactly, so PATH is found and extended",
	Here:       "the name is spelled Path, so an exact lookup misses it and a second PATH is written that the system collapses, leaving every launched process with bothy's bin alone",
	Carried:    "the session environment folds names to compare them, keeping the spelling it was given",
	ADR:        "ADR-060",
	Test:       "TestTheSessionKeepsTheMachinesPathOnWindows",
	Measured:   "2026-10-23, Windows 11",
}, {
	ID:         "window-close-hangup",
	OS:         "windows",
	Capability: "sessions",
	Reference:  "closing the window hangs up the client, and bothy ends the session it recorded",
	Here:       "there is no hangup to act on, so the session outlives its window and is left to be reattached or killed by name",
	ADR:        "ADR-054",
	Test:       "TestBothyInstallsOnWindows",
	Measured:   "2026-10-22, Windows 11",
}, {
	ID:         "isolation-cache",
	OS:         "windows",
	Capability: "isolation",
	Reference:  "every path the multiplexer writes is redirected, so uninstalling is one directory",
	Here:       "the config directory is honoured but the cache and socket directories are not, so uninstalling is one directory and two named paths",
	ADR:        "ADR-054",
	Test:       "TestBothyInstallsOnWindows",
	Measured:   "2026-10-24, Windows 11 -- a harness that redirected only the home shared a socket directory with the real session",
}}

// Active is what this machine diverges in. A pure function of the machine and
// the multiplexer, so one runner can be asked about every platform: the
// reference arms are asserted rather than assumed.
func Active(i Info, mux string) []Divergence {
	var out []Divergence
	for _, d := range divergences {
		if d.OS != i.OS {
			continue
		}
		if d.Mux != "" && d.Mux != mux {
			continue
		}
		out = append(out, d)
	}
	return out
}

// Divergences is the whole register, for the tests that hold it together and
// for a report that names what is not covered here.
func Divergences() []Divergence { return divergences }
