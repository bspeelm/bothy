package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bspeelm/bothy/internal/install"
)

// checkLayoutBuilt compares the layout Zellij resolved against the profile.
// The renderer's golden test covers what bothy emits; this covers whether
// Zellij still interprets it the same way. Runs only inside a live session
// (ZELLIJ_SESSION_NAME); skips everywhere else.
func checkLayoutBuilt(env Env) Result {
	session := os.Getenv("ZELLIJ_SESSION_NAME")
	if session == "" {
		return skip("not inside a zellij session")
	}
	// Only a session bothy launched: another session's layout has nothing to
	// do with bothy's profile. BOTHY_SESSION is set by SessionEnv.
	if os.Getenv("BOTHY_SESSION") == "" {
		return skip("this zellij session was not launched by bothy")
	}
	path, err := sessionLayoutPath(env.Platform.Home, session)
	if err != nil {
		return skip("no resolved layout for this session yet")
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 {
		return skip("the resolved layout is not readable yet")
	}

	got, ok := countContentPanes(string(body))
	if !ok {
		// The serialisation changed shape; a count from it would mean nothing.
		return skip("could not read the resolved layout's shape")
	}

	prof, err := install.LoadProfile(env.Platform, env.ProfileName)
	if err != nil {
		return skip("the profile does not load; see profile-renders")
	}
	want := prof.PaneCount()

	if got != want {
		return warn(fmt.Sprintf("zellij built %d panes, the profile describes %d", got, want),
			"the layout was interpreted differently than it was written: "+path,
			"if you did not add panes by hand, this is worth reporting — the "+
				"renderer and this zellij disagree")
	}
	return pass(fmt.Sprintf("zellij built the %d panes the profile describes", got))
}

// sessionLayoutPath finds the resolved layout for a session. The directory is
// keyed by zellij's version, so it is globbed rather than computed.
func sessionLayoutPath(home, session string) (string, error) {
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		cache = filepath.Join(home, ".cache")
	}
	matches, err := filepath.Glob(filepath.Join(cache, "zellij", "*", "session_info", session, "session-layout.kdl"))
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("not found")
	}
	return matches[0], nil
}

// countContentPanes counts panes running something in the first tab. Three
// things would corrupt a naive count: new_tab_template repeats the layout,
// floating_panes adds Zellij's own tip pane, and the bars are panes too.
func countContentPanes(kdl string) (int, bool) {
	lines := strings.Split(kdl, "\n")

	inTab, depth, floatingDepth := false, 0, -1
	count := 0
	for i, raw := range lines {
		line := strings.TrimSpace(raw)

		if !inTab {
			// Only the first tab; new_tab_template repeats it verbatim.
			if strings.HasPrefix(line, "tab ") || line == "tab {" {
				inTab, depth = true, 0
			} else {
				continue
			}
		}

		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")

		if strings.HasPrefix(line, "floating_panes") && floatingDepth < 0 {
			floatingDepth = depth
		}

		if floatingDepth < 0 && strings.HasPrefix(line, "pane") {
			// A plugin pane declares its plugin on the following line.
			isPlugin := i+1 < len(lines) && strings.Contains(lines[i+1], "plugin location=")
			hasCommand := strings.Contains(line, "command=")
			hasName := strings.Contains(line, "name=")
			if !isPlugin && (hasCommand || hasName) {
				count++
			}
		}

		depth += opens - closes
		if floatingDepth >= 0 && depth <= floatingDepth {
			floatingDepth = -1
		}
		if depth <= 0 {
			break // end of the first tab
		}
	}

	if count == 0 {
		return 0, false
	}
	return count, true
}
