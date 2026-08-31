package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bothy-dev/bothy"
	"github.com/bothy-dev/bothy/internal/platform"
)

// Plugin is a Yazi plugin bothy's generated config depends on.
type Plugin struct {
	Name  string   `toml:"name"`
	Use   string   `toml:"use"`
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

// EnsureYaziPlugins installs any missing plugin with Yazi's own package
// manager, into bothy's config tree.
//
// This runs before the templates are rendered, because the generated config is
// written to match what is actually installed. An earlier version referenced
// all four unconditionally and installed none of them, which produced a config
// that looked correct, passed `yazi --clear-cache` — that does not execute
// init.lua — and failed only when the workspace was launched.
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

	ya, lookErr := exec.LookPath("ya")
	for _, pl := range plugins {
		if PluginInstalled(p, pl.Name) {
			rep.Present = append(rep.Present, pl)
			continue
		}
		if offline {
			rep.Failed = append(rep.Failed, PluginFailure{pl, fmt.Errorf("offline")})
			continue
		}
		if lookErr != nil {
			rep.Failed = append(rep.Failed, PluginFailure{pl,
				fmt.Errorf("ya is not installed; it ships with yazi")})
			continue
		}

		cmd := exec.Command(ya, "pkg", "add", pl.Use)
		cmd.Env = append(os.Environ(), "YAZI_CONFIG_HOME="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			rep.Failed = append(rep.Failed, PluginFailure{pl,
				fmt.Errorf("%v: %s", err, firstLine(out))})
			continue
		}
		rep.Installed = append(rep.Installed, pl)
	}
	return rep, nil
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
