package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/platform"
)

// Plugin is a Yazi plugin bothy's generated config depends on.
type Plugin struct {
	Name string `toml:"name"`
	Use  string `toml:"use"`
	// Rev is the commit to install. Pinned in the repository rather than
	// resolved on the machine, so that two installs a week apart get the same
	// plugin.
	Rev   string   `toml:"rev"`
	Gives string   `toml:"gives"`
	Needs []string `toml:"needs"`
}

type pluginFile struct {
	Plugins []Plugin `toml:"plugin"`
}

// YaziPlugins are the plugins bothy's yazi config references.
func YaziPlugins() ([]Plugin, error) {
	src, err := bothy.Slots.ReadFile("slots/plugins/yazi.toml")
	if err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}
	var f pluginFile
	if err := toml.Unmarshal(src, &f); err != nil {
		return nil, fmt.Errorf("install: slots/plugins/yazi.toml: %w", err)
	}
	return f.Plugins, nil
}

// PluginInstalled reports whether a plugin is present in bothy's yazi tree.
func PluginInstalled(p platform.Info, name string) bool {
	fi, err := os.Stat(filepath.Join(YaziDir(p), "plugins", name+".yazi"))
	return err == nil && fi.IsDir()
}

// PluginReport is what happened to the plugins during an install.
type PluginReport struct {
	Installed []Plugin
	Present   []Plugin
	Failed    []PluginFailure
}

// PluginFailure is a plugin bothy could not install.
type PluginFailure struct {
	Plugin Plugin
	Err    error
}

// packageFile is Yazi's package.toml: what `ya pkg install` reads to decide
// what to fetch and at which revision.
type packageFile struct {
	Plugin struct {
		Deps []packageDep `toml:"deps"`
	} `toml:"plugin"`
	Flavor struct {
		Deps []packageDep `toml:"deps"`
	} `toml:"flavor"`
}

type packageDep struct {
	Use  string `toml:"use"`
	Rev  string `toml:"rev"`
	Hash string `toml:"hash"`
}

// packagePath is the file `ya pkg` reads and writes.
func packagePath(p platform.Info) string {
	return filepath.Join(YaziDir(p), "package.toml")
}

// installedRevs reports the revision recorded for each plugin, so that one
// sitting at a revision other than the pinned one counts as missing.
func installedRevs(p platform.Info) map[string]string {
	out := map[string]string{}
	raw, err := os.ReadFile(packagePath(p))
	if err != nil {
		return out
	}
	var f packageFile
	if err := toml.Unmarshal(raw, &f); err != nil {
		return out
	}
	for _, d := range f.Plugin.Deps {
		out[d.Use] = d.Rev
	}
	return out
}

// writePackageFile writes the pins for `ya pkg install` to act on. No
// generated-by header, unlike every other file bothy writes: `ya pkg` rewrites
// this one from its own model, so a comment would not survive the first
// install.
func writePackageFile(p platform.Info, plugins []Plugin) error {
	var f packageFile
	for _, pl := range plugins {
		f.Plugin.Deps = append(f.Plugin.Deps, packageDep{Use: pl.Use, Rev: pl.Rev})
	}
	f.Flavor.Deps = []packageDep{}
	out, err := toml.Marshal(f)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	return os.WriteFile(packagePath(p), out, 0o644)
}

// EnsureYaziPlugins installs the plugins bothy's config references, at the
// revisions pinned in slots/plugins/yazi.toml.
//
// `ya pkg install` rather than `ya pkg add`: add resolves the revision at the
// moment it runs, which is how two installs a week apart got different
// plugins. Runs before the templates render, because a config referencing a
// plugin that is not there looks correct and fails at launch.
func EnsureYaziPlugins(p platform.Info, offline bool) (*PluginReport, error) {
	rep := &PluginReport{}
	plugins, err := YaziPlugins()
	if err != nil {
		return nil, err
	}

	dir := YaziDir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}

	have := installedRevs(p)
	var wanted []Plugin
	for _, pl := range plugins {
		if PluginInstalled(p, pl.Name) && have[pl.Use] == pl.Rev {
			rep.Present = append(rep.Present, pl)
			continue
		}
		wanted = append(wanted, pl)
	}
	if len(wanted) == 0 {
		return rep, nil
	}

	// ya ships inside the yazi archive, so when bothy supplied yazi it is in
	// bothy's bin and nowhere the ambient PATH would find it.
	ya := ToolPath(p, "ya")
	_, lookErr := os.Stat(ya)

	// `ya pkg` clones from GitHub, so it needs git -- and says only "Failed to
	// execute `git` command" when it is missing, which does not point at
	// anything actionable.
	_, gitErr := exec.LookPath("git")

	blocked := func(reason error) (*PluginReport, error) {
		for _, pl := range wanted {
			rep.Failed = append(rep.Failed, PluginFailure{pl, reason})
		}
		return rep, nil
	}
	switch {
	case offline:
		return blocked(fmt.Errorf("offline"))
	case lookErr != nil:
		return blocked(fmt.Errorf("ya is not installed; it ships with yazi"))
	case gitErr != nil:
		return blocked(fmt.Errorf("git is not installed, and ya pkg clones from GitHub"))
	}

	if err := writePackageFile(p, plugins); err != nil {
		return nil, err
	}

	cmd := exec.Command(ya, "pkg", "install")
	// XDG_CACHE_HOME as well as the config home: `ya pkg` clones the whole
	// yazi-rs/plugins repository into its package cache, which otherwise lands
	// in ~/.cache/yazi where uninstall cannot reach it.
	cmd.Env = append(os.Environ(),
		"YAZI_CONFIG_HOME="+dir,
		"XDG_CACHE_HOME="+filepath.Join(p.BothyDir(), "cache"),
	)
	out, runErr := cmd.CombinedOutput()

	// One command installs them all, so what happened to each is read from the
	// tree afterwards rather than from the exit code.
	after := installedRevs(p)
	for _, pl := range wanted {
		switch {
		case PluginInstalled(p, pl.Name) && after[pl.Use] == pl.Rev:
			rep.Installed = append(rep.Installed, pl)
		case runErr != nil:
			rep.Failed = append(rep.Failed, PluginFailure{pl,
				fmt.Errorf("%v: %s", runErr, firstLine(out))})
		default:
			rep.Failed = append(rep.Failed, PluginFailure{pl,
				fmt.Errorf("ya pkg install did not produce it at %s", shortRev(pl.Rev))})
		}
	}
	return rep, nil
}

// shortRev is a revision at the length people read.
func shortRev(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

// InstalledPlugins is the set of plugin names present, for the templates to
// key on so a generated config never references something that is not there.
func InstalledPlugins(p platform.Info) map[string]bool {
	out := map[string]bool{}
	plugins, err := YaziPlugins()
	if err != nil {
		return out
	}
	for _, pl := range plugins {
		if PluginInstalled(p, pl.Name) {
			out[pl.Name] = true
		}
	}
	return out
}

func firstLine(b []byte) string {
	for i, c := range b {
		if c == '\n' {
			return string(b[:i])
		}
	}
	return string(b)
}
