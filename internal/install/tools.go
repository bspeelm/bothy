package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/fetch"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/state"
	"github.com/bothy-dev/bothy/internal/tools"
)

// ToolReport is what happened to the tools during an install.
type ToolReport struct {
	Used    []tools.Decision // the system's copy was good enough
	Fetched []tools.Decision // bothy supplied one
	Failed  []ToolFailure
	Skipped bool // --offline
}

// ToolFailure is a tool bothy meant to supply and could not.
type ToolFailure struct {
	Decision tools.Decision
	Err      error
}

// EnsureTools applies the fill-gaps policy: leave a good system tool alone,
// fetch a missing or too-old one into bothy's own bin.
//
// A failure here is reported, not fatal. Most of these tools are conveniences,
// and an install that refuses to write any config because a fuzzy finder would
// not download is worse than one that says so and carries on — the doctor will
// say it again, with the fix.
func EnsureTools(p platform.Info, cfg config.Config, offline bool) (*ToolReport, error) {
	rep := &ToolReport{Skipped: offline}

	names, err := tools.Required(cfg.Slots.Mux, cfg.Slots.Browser, cfg.Extras)
	if err != nil {
		return nil, err
	}
	decisions, err := tools.ResolveAll(names)
	if err != nil {
		return nil, err
	}

	if offline {
		for _, d := range decisions {
			if d.Action == tools.UseSystem {
				rep.Used = append(rep.Used, d)
			}
		}
		return rep, nil
	}

	m, err := state.Load(p.StateDir())
	if err != nil {
		return nil, err
	}
	lock, err := fetch.LoadLock()
	if err != nil {
		return nil, err
	}

	for _, d := range decisions {
		if d.Action == tools.UseSystem {
			rep.Used = append(rep.Used, d)
			m.RecordBinary(state.Binary{
				Name: d.Tool.Name, Path: d.Path,
				Version: d.Version.String(), Source: "system",
			})
			continue
		}

		entry, ok := lock.Get(d.Tool.Name)
		if !ok {
			rep.Failed = append(rep.Failed, ToolFailure{d,
				fmt.Errorf("%s is not pinned in bothy.lock; run 'bothy lock'", d.Tool.Name)})
			continue
		}
		res, err := fetch.Install(d.Tool, p, entry, p.BinDir())
		if err != nil {
			rep.Failed = append(rep.Failed, ToolFailure{d, err})
			continue
		}
		rep.Fetched = append(rep.Fetched, d)
		m.RecordBinary(state.Binary{
			Name: d.Tool.Name, Path: filepath.Join(p.BinDir(), d.Tool.Binary),
			Version: res.Version, SHA256: res.SHA256, Source: "bothy",
		})
	}

	if err := m.Save(p.StateDir()); err != nil {
		return nil, err
	}
	return rep, nil
}

// ToolPath resolves a binary the way bothy's session will: its own bin first,
// then the system PATH.
//
// Anything that asks a tool a question — the graphics probe, the doctor — must
// go through this. Asking the system's zellij whether it supports Kitty
// graphics, moments after fetching a newer one to bothy's bin precisely
// because it does not, produces a confident answer about the wrong binary.
func ToolPath(p platform.Info, name string) string {
	if name == "" {
		return ""
	}
	if own, ok := InstalledBinary(p, name); ok {
		return own
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return name
}

// InstalledBinary reports the path of a tool bothy supplied, if it did.
func InstalledBinary(p platform.Info, name string) (string, bool) {
	path := filepath.Join(p.BinDir(), name)
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
		return path, true
	}
	return "", false
}
