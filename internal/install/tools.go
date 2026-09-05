package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/fetch"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/state"
	"github.com/bspeelm/bothy/internal/tools"
)

// ToolReport is what happened to the tools during an install.
type ToolReport struct {
	Used    []tools.Decision // the system's copy was good enough
	Fetched []tools.Decision // bothy supplied one
	Current []tools.Decision // bothy had already supplied one, at the pinned version
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
// A failure is reported, not fatal: refusing to write any config because a
// fuzzy finder would not download is worse than saying so and carrying on, and
// the doctor says it again with the fix. progress is called before each
// download, because tens of megabytes in silence looks like a hang.
func EnsureTools(p platform.Info, cfg config.Config, offline bool, bothyVer string, progress func(string)) (*ToolReport, error) {
	rep := &ToolReport{Skipped: offline}

	names, err := tools.Required(cfg.Providers(), cfg.Extras)
	if err != nil {
		return nil, err
	}
	decisions, err := tools.ResolveAll(names, p.BinDir())
	if err != nil {
		return nil, err
	}

	// Written on every path, offline included: InstalledIn is what lets a host
	// launch find the container holding the tools.
	m, err := state.Load(p.StateDir())
	if err != nil {
		return nil, err
	}
	m.InstalledIn = p.ContainerName

	if offline {
		for _, d := range decisions {
			if d.Action == tools.UseSystem {
				rep.Used = append(rep.Used, d)
				m.RecordBinary(state.Binary{
					Name: d.Tool.Name, Path: d.Path,
					Version: d.Version.String(), Source: "system",
				})
			}
		}
		return rep, m.Save(p.StateDir(), bothyVer)
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
		// Already ours, at the version this bothy pins. Skipping the download
		// is what makes install idempotent; comparing against the pin rather
		// than the minimum is what makes it an upgrade when the pin moves.
		if Supplied(p, m, d.Tool.Name, entry.Version) {
			rep.Current = append(rep.Current, d)
			continue
		}
		if progress != nil {
			progress(fmt.Sprintf("  ↓ %s %s", d.Tool.Name, entry.Version))
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

	if err := m.Save(p.StateDir(), bothyVer); err != nil {
		return nil, err
	}
	return rep, nil
}

// ours reports whether a recorded binary is one bothy put in its own bin.
// The directory is trusted over the recorded source, because a manifest can
// carry `"source": "system"` for a binary sitting in bothy's own bin.
func ours(p platform.Info, b state.Binary) bool {
	return b.Source == "bothy" ||
		filepath.Dir(filepath.Clean(b.Path)) == filepath.Clean(p.BinDir())
}

// Supplied reports whether bothy already has this tool at want, with the
// binary still there. The manifest is the record, not the filesystem: a binary
// of the right name says nothing about its version, and a moved lockfile pin
// is exactly what this has to notice.
func Supplied(p platform.Info, m *state.Manifest, name, want string) bool {
	for _, b := range m.Binaries {
		if b.Name != name {
			continue
		}
		if !ours(p, b) || b.Version != want {
			return false
		}
		_, err := os.Stat(b.Path)
		return err == nil
	}
	return false
}

// SuppliedVersion is the version bothy installed for a tool, or "" when bothy
// did not supply it.
func SuppliedVersion(p platform.Info, m *state.Manifest, name string) string {
	for _, b := range m.Binaries {
		if b.Name == name && ours(p, b) {
			return b.Version
		}
	}
	return ""
}

// PendingFetches are the tools that would actually be downloaded: those bothy
// has to supply and does not already have at the pinned version. Used by the
// first-run prompt, so it asks about the megabytes it is about to spend rather
// than every tool the system lacks.
func PendingFetches(p platform.Info, decisions []tools.Decision) []tools.Decision {
	m, err := state.Load(p.StateDir())
	if err != nil {
		return fetchesIn(decisions)
	}
	lock, err := fetch.LoadLock()
	if err != nil {
		return fetchesIn(decisions)
	}
	var out []tools.Decision
	for _, d := range decisions {
		if d.Action != tools.Fetch {
			continue
		}
		if entry, ok := lock.Get(d.Tool.Name); ok && Supplied(p, m, d.Tool.Name, entry.Version) {
			continue
		}
		out = append(out, d)
	}
	return out
}

func fetchesIn(decisions []tools.Decision) []tools.Decision {
	var out []tools.Decision
	for _, d := range decisions {
		if d.Action == tools.Fetch {
			out = append(out, d)
		}
	}
	return out
}

// ToolPath resolves a binary the way bothy's session will: its own bin first,
// then the system PATH. Anything that asks a tool a question must go through
// this -- asking the system's zellij about Kitty graphics moments after
// fetching a newer one because it lacks them answers about the wrong binary.
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

// InstalledIn is the container bothy resolved its tools in, from the manifest.
// Empty when bothy has not been installed, or was installed on the host.
func InstalledIn(p platform.Info) string {
	m, err := state.Load(p.StateDir())
	if err != nil {
		return ""
	}
	return m.InstalledIn
}

// Box is which container a project runs in and which rule decided it, so
// `bothy box` can explain the answer rather than only assert it.
type Box struct {
	Name   string
	Reason string
}

// Resolve decides which container a project directory runs in, and says which
// rule decided. In order: an explicit workspace.container, the container bothy
// is already inside, this project's recorded answer, and finally where the
// tools were resolved.
//
// The recorded answer beats installed_in even when it is "": that entry is
// someone choosing the host, and falling through would send them back to a box
// they turned down. installed_in is the floor because it answers a different
// question -- where the tools are, not where the project lives -- and is only
// better than having no answer at all.
func Resolve(p platform.Info, cfg config.Config, dir string) Box {
	if cfg.Workspace.Container != "" {
		return Box{cfg.Workspace.Container, "set by workspace.container"}
	}
	if p.ContainerName != "" {
		return Box{p.ContainerName, "the container bothy is running in"}
	}
	if name, ok := ProjectBoxes(p)[dir]; ok {
		return Box{name, "chosen for this project"}
	}
	if in := InstalledIn(p); in != "" {
		return Box{in, "where bothy installed its tools"}
	}
	return Box{}
}

// ContainerFor is Resolve's answer alone, for the callers that only act on it.
func ContainerFor(p platform.Info, cfg config.Config, dir string) string {
	return Resolve(p, cfg, dir).Name
}

// ProjectBoxes is the project-to-box record, empty when it cannot be read: an
// unreadable record means unanswered, which costs one prompt and repairs
// itself, where failing the launch would not.
func ProjectBoxes(p platform.Info) state.Boxes {
	b, err := state.LoadBoxes(p.StateDir())
	if err != nil {
		return state.Boxes{}
	}
	return b
}

// ForgetBox drops every project's claim on a box, for when the box is gone.
// It returns the directories that had one: they fall back to the next rule,
// and being told is the difference between a move and a surprise.
func ForgetBox(p platform.Info, name string) ([]string, error) {
	boxes := ProjectBoxes(p)
	var freed []string
	for dir, in := range boxes {
		if in == name {
			freed = append(freed, dir)
			delete(boxes, dir)
		}
	}
	if len(freed) == 0 {
		return nil, nil
	}
	slices.Sort(freed)
	return freed, boxes.Save(p.StateDir())
}

// RecordBox remembers which container a project directory belongs in.
func RecordBox(p platform.Info, dir, name string) error {
	b := ProjectBoxes(p)
	b[dir] = name
	return b.Save(p.StateDir())
}
