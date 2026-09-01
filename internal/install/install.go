// Package install renders bothy's config tree.
//
// Everything it writes lands under <bothy>/config (ADR-009). It never touches
// ~/.config/yazi, ~/.vimrc, ~/.bashrc.d or your global git config; the tools
// are launched pointed at bothy's tree instead. See cmd/bothy/dev.go for the
// environment that does the pointing.
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
	"github.com/bspeelm/bothy/internal/render"
	"github.com/bspeelm/bothy/internal/theme"
)

// Data is what every template sees.
type Data struct {
	Theme     theme.Palette
	ThemeName string

	// ImagePreviews and GraphicsReason come from the probe, not from config —
	// see docs/decisions.md ADR-007.
	ImagePreviews  bool
	GraphicsReason string

	Container     bool
	ContainerName string

	EditorBin      string
	AgentBin       string
	BrowserBin     string
	VimColorscheme string

	Font       string
	ProjectDir string

	Watermark        bool
	WatermarkPath    string
	WatermarkOpacity string
	PaneFrames       string

	// Plugins is the set of Yazi plugins actually installed. Templates key on
	// it so a generated config never references something that is not there.
	Plugins map[string]bool
}

// Result reports what an install did.
type Result struct {
	Written   []string
	Unchanged []string
	Root      string
	Data      Data
	Plugins   *PluginReport
}

// Options configures one install run.
type Options struct {
	DryRun bool
	// Offline skips anything needing the network, including plugin install.
	Offline bool
}

// Run renders bothy's config tree.
func Run(p platform.Info, cfg config.Config, opts Options) (*Result, error) {
	pal, err := cfg.Palette(p)
	if err != nil {
		return nil, err
	}
	res0 := &Result{}

	// Plugins first: the generated config is written to match what is actually
	// installed, so it has to know before the templates render.
	if !opts.DryRun && cfg.Slots.Browser == "yazi" && !cfg.PassesThrough("yazi") {
		pr, err := EnsureYaziPlugins(p, opts.Offline)
		if err != nil {
			return nil, err
		}
		res0.Plugins = pr
	}

	w := render.NewWriter(p.BothyDir(), filepath.Join(p.UserConfigDir(), "overrides"))
	w.DryRun = opts.DryRun

	data := buildData(p, cfg, pal)
	res := res0
	res.Root = p.BothyDir()
	res.Data = data

	for _, f := range plan(p, cfg, data) {
		body, err := renderFile(w, f, data)
		if err != nil {
			return nil, err
		}
		var changed bool
		if f.Exec {
			changed, err = w.WriteExec(f.Dest, body)
		} else {
			changed, err = w.Write(f.Dest, body)
		}
		if err != nil {
			return nil, err
		}
		if changed {
			res.Written = append(res.Written, f.Dest)
		} else {
			res.Unchanged = append(res.Unchanged, f.Dest)
		}
	}
	return res, nil
}

// file is one destination and the template that fills it.
type file struct {
	Dest     string
	Tool     string // names the ~/.config/bothy/overrides/<tool>/ directory
	Template string // path inside the embedded templates FS
	// Asset is copied byte-for-byte instead of rendered, for content a text
	// header would corrupt.
	Asset string
	Exec  bool
}

func renderFile(w *render.Writer, f file, data Data) ([]byte, error) {
	if f.Asset != "" {
		// No header: a PNG with a comment glued to the front is not a PNG.
		return bothy.Templates.ReadFile(f.Asset)
	}
	src, err := bothy.Templates.ReadFile(f.Template)
	if err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}
	return w.Render(f.Dest, f.Tool, filepath.Base(f.Template), string(src), data)
}

// Destinations inside bothy's config root. The launcher points each tool at
// these, so they are named once and used from both places.

func ZellijDir(p platform.Info) string { return filepath.Join(p.ConfigRoot(), "zellij") }
func YaziDir(p platform.Info) string   { return filepath.Join(p.ConfigRoot(), "yazi") }
func VimDir(p platform.Info) string    { return filepath.Join(p.ConfigRoot(), "vim") }
func VimRC(p platform.Info) string     { return filepath.Join(VimDir(p), "vimrc") }
func GhosttyConf(p platform.Info) string {
	return filepath.Join(p.ConfigRoot(), "ghostty.conf")
}

// plan lists every file to write. Adding a provider should mean adding entries
// here and templates beside them — never new logic.
func plan(p platform.Info, cfg config.Config, data Data) []file {
	var out []file

	// A slot passed through uses the user's own config directory, so writing
	// bothy's version of it would leave files nothing ever reads.
	if cfg.Slots.Mux == "zellij" && !cfg.PassesThrough("zellij") {
		z := ZellijDir(p)
		out = append(out,
			file{Dest: filepath.Join(z, "config.kdl"), Tool: "zellij",
				Template: "templates/mux/zellij/config.kdl.tmpl"},
			file{Dest: filepath.Join(z, "themes", data.ThemeName+".kdl"), Tool: "zellij",
				Template: "templates/mux/zellij/theme.kdl.tmpl"},
		)
	}

	if cfg.Slots.Browser == "yazi" && !cfg.PassesThrough("yazi") {
		y := YaziDir(p)
		out = append(out,
			file{Dest: filepath.Join(y, "yazi.toml"), Tool: "yazi",
				Template: "templates/browser/yazi/yazi.toml.tmpl"},
			file{Dest: filepath.Join(y, "keymap.toml"), Tool: "yazi",
				Template: "templates/browser/yazi/keymap.toml.tmpl"},
			file{Dest: filepath.Join(y, "init.lua"), Tool: "yazi",
				Template: "templates/browser/yazi/init.lua.tmpl"},
			file{Dest: filepath.Join(y, "theme.toml"), Tool: "yazi",
				Template: "templates/browser/yazi/theme.toml.tmpl"},
		)
		// The placeholder previewer only stands in for images, so it is only
		// written when images are actually turned off.
		if !data.ImagePreviews {
			out = append(out, file{
				Dest: filepath.Join(y, "plugins", "enter-hint.yazi", "main.lua"), Tool: "yazi",
				Template: "templates/browser/yazi/enter-hint.lua.tmpl",
			})
		}
	}

	// The editor is yours. bothy sets $EDITOR for its own session and stops
	// there — unless you have no config and want one, which is the same
	// gap-filling rule the binaries follow.
	if cfg.Slots.Editor == "vim" && cfg.Editor.ProvideConfig && !cfg.PassesThrough("vim") {
		out = append(out,
			file{Dest: VimRC(p), Tool: "vim",
				Template: "templates/editor/vim/vimrc.tmpl"},
			file{Dest: filepath.Join(VimDir(p), "colors", data.ThemeName+".vim"), Tool: "vim",
				Template: "templates/theme/vim-colorscheme.vim.tmpl"},
		)
	}

	// Ghostty's config carries the palette inline rather than naming a theme:
	// theme *lookup* paths are not relocatable, so a `theme = x` reference
	// would send it hunting in ~/.config/ghostty/themes and defeat the point.
	if cfg.Slots.Terminal == "ghostty" && !cfg.PassesThrough("ghostty") {
		out = append(out, file{
			Dest: GhosttyConf(p), Tool: "ghostty",
			Template: "templates/terminal/ghostty/config.tmpl",
		})
		if data.Watermark {
			out = append(out, file{
				Dest:  data.WatermarkPath,
				Asset: "templates/extras/watermark/tux.png",
			})
		}
	}

	// Inside a container there is no desktop to open a file with, so the
	// opener forwards to the host. The shim lives in bothy's own bin/, which
	// is on PATH for bothy's session only — revision 1 put it in ~/.local/bin,
	// where it was on the host's PATH too and needed a guard against the host
	// executing it and recursing into itself. Scoping removes that hazard; the
	// guard stays anyway, because it is three lines and PATH is fickle.
	//
	// SharedHome rather than InContainer: the shim forwards through
	// flatpak-spawn, which exists in Toolbx and Distrobox and nowhere else. A
	// plain podman or docker container has no host session to reach, so
	// writing it there produced a shim that could not work -- and then passed
	// the opener check, because that check only asks whether xdg-open
	// resolves and this put something on PATH that answered to the name.
	if p.SharedHome {
		out = append(out, file{
			Dest: filepath.Join(p.BinDir(), "xdg-open"), Tool: "shell",
			Template: "templates/shell/xdg-open.tmpl",
			Exec:     true,
		})
	}

	return out
}

// buildData assembles the template data, running the graphics probe.
func buildData(p platform.Info, cfg config.Config, pal theme.Palette) Data {
	name := slug(pal.Name)
	g := probe.CheckGraphics(ToolPath(p, muxBinary(cfg)), p.Terminal)

	d := Data{
		Theme:            pal,
		ThemeName:        name,
		ImagePreviews:    g.Supported,
		GraphicsReason:   g.Reason,
		Container:        p.InContainer(),
		ContainerName:    ContainerFor(p, cfg),
		EditorBin:        EditorBinary(cfg.Slots.Editor),
		AgentBin:         agentBinary(cfg.Slots.Agent),
		BrowserBin:       cfg.Slots.Browser,
		Font:             cfg.Theme.Font,
		ProjectDir:       cfg.Workspace.ProjectDir,
		Watermark:        cfg.Workspace.Watermark,
		WatermarkPath:    filepath.Join(p.ConfigRoot(), "watermark.png"),
		WatermarkOpacity: "0.05",
		PaneFrames:       cfg.Workspace.PaneFrames,
		Plugins:          InstalledPlugins(p),
	}
	d.VimColorscheme = cfg.Theme.VimColorscheme
	if d.VimColorscheme == "" {
		d.VimColorscheme = name
	}
	return d
}

// muxBinary returns the multiplexer to interrogate, or "" if there is none.
func muxBinary(cfg config.Config) string {
	if cfg.Slots.Mux == "" || cfg.Slots.Mux == "none" {
		return ""
	}
	return cfg.Slots.Mux
}

// EditorBinary maps an editor slot to the command it runs. Exported because
// the doctor has to check the same binary the launcher will use; two spellings
// of that mapping is how a check comes to report on the wrong thing.
func EditorBinary(slot string) string {
	switch slot {
	case "helix":
		return "hx"
	case "neovim":
		return "nvim"
	case "":
		return "vim"
	default:
		return slot
	}
}

func agentBinary(slot string) string {
	switch slot {
	case "claude-code", "":
		return "claude"
	case "gemini-cli":
		return "gemini"
	default:
		return slot
	}
}

// Commands maps layout slots to the commands their panes run.
func Commands(cfg config.Config) layout.Commands {
	return layout.Commands{
		"browser": cfg.Slots.Browser,
		"editor":  EditorBinary(cfg.Slots.Editor),
		"agent":   agentBinary(cfg.Slots.Agent),
	}
}

// LoadProfile reads a profile by name, preferring the user's own copy in
// ~/.config/bothy/profiles over the shipped one.
func LoadProfile(p platform.Info, name string) (layout.Profile, error) {
	user := filepath.Join(p.UserConfigDir(), "profiles", name+".toml")
	if _, err := os.Stat(user); err == nil {
		return layout.LoadProfile(user)
	}
	shipped := "profiles/" + name + ".toml"
	src, err := bothy.Profiles.ReadFile(shipped)
	if err != nil {
		return layout.Profile{}, fmt.Errorf("install: no profile named %q", name)
	}
	return layout.ParseProfile(src, shipped)
}

// slug makes a palette name safe as a filename and as a Zellij theme
// identifier, both of which dislike spaces.
func slug(name string) string {
	if name == "" {
		return "bothy"
	}
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == ' ', r == '.':
			return '-'
		}
		return -1
	}, name)
	if out == "" {
		return "bothy"
	}
	return out
}

// SessionEnv builds the environment for bothy's process tree.
//
// This is where isolation actually takes effect: the configs were written into
// bothy's directory, and nothing reads them unless the tools are told to.
// Telling them is a handful of environment variables scoped to one process
// tree, so the user's shell keeps its own PATH and EDITOR.
//
// The doctor uses this too, deliberately. A check that runs a tool with a
// different config than the launcher will is worse than no check, because it
// reports confidently about the wrong file.
func SessionEnv(p platform.Info, cfg config.Config) []string {
	env := newEnv(os.Environ())

	// bothy's own bin first, so a tool it had to supply is used here and only
	// here. Nothing it installs changes what a command means in your shell.
	env.set("PATH", p.BinDir()+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Passthrough must *unset*, not merely decline to set. The session inherits
	// the current environment, so if one of these is already exported — by a
	// bothy session this was launched from, or by hand — declining to set it
	// leaves the inherited value in place and the tool quietly uses bothy's
	// config anyway. Found by the test suite failing when run from inside a
	// bothy session, which is the only place the difference shows.
	if cfg.Slots.Mux == "zellij" && !cfg.PassesThrough("zellij") {
		env.set("ZELLIJ_CONFIG_DIR", ZellijDir(p))
	} else {
		env.unset("ZELLIJ_CONFIG_DIR")
	}
	if cfg.Slots.Browser == "yazi" && !cfg.PassesThrough("yazi") {
		env.set("YAZI_CONFIG_HOME", YaziDir(p))
	} else {
		env.unset("YAZI_CONFIG_HOME")
	}

	// Fedora's nano-default-editor exports EDITOR=nano from /etc/profile.d,
	// and that is what yazi, lazygit and git shell out to. Setting it here
	// covers bothy's panes without touching the user's shell config.
	editor := EditorBinary(cfg.Slots.Editor)
	env.set("EDITOR", editor)
	env.set("VISUAL", editor)

	// VIMINIT takes precedence over ~/.vimrc, so it is only set when bothy is
	// providing a vim config. Otherwise vim is yours and loads yours.
	if cfg.Slots.Editor == "vim" && cfg.Editor.ProvideConfig && !cfg.PassesThrough("vim") {
		env.set("VIMINIT", "source "+VimRC(p))
	}

	// The XDG directories, pointed inside bothy's tree.
	//
	// ADR-009 says bothy writes nothing outside its own directory, and until
	// now that covered only the files bothy writes itself. It said nothing
	// about what the tools bothy *runs* decide to write -- and they write
	// plenty: `yazi --clear-cache` clears ~/.cache/yazi, `zellij setup
	// --check` touches zellij's own cache and data directories, and both are
	// invoked by the doctor, which uses this same environment. So the promise
	// held for install and quietly leaked at every other command.
	//
	// plugins.go said this gap "has to be closed one subprocess at a time".
	// It does not: the tools all agree on where to look, so telling them once
	// closes it for every subprocess at once, including the ones added later.
	env.set("XDG_CACHE_HOME", p.CacheDir())
	env.set("XDG_STATE_HOME", p.StateDir())
	env.set("XDG_DATA_HOME", p.ShareDir())

	env.set("BOTHY_SESSION", "1")
	return env.slice()
}

// env is an environment being assembled.
//
// It replaces rather than appends, which is not a detail: an environment with
// two PATH entries is ambiguous — which one a process sees depends on the libc
// and on whether anything deduplicated it on the way through. Appending a
// second PATH looked right and left the original one first in the list, so the
// tools bothy supplied were not actually found.
type env struct {
	keys   []string
	values map[string]string
}

func newEnv(existing []string) *env {
	e := &env{values: map[string]string{}}
	for _, kv := range existing {
		if k, v, ok := strings.Cut(kv, "="); ok {
			e.set(k, v)
		}
	}
	return e
}

func (e *env) set(k, v string) {
	if _, seen := e.values[k]; !seen {
		e.keys = append(e.keys, k)
	}
	e.values[k] = v
}

// unset removes a key entirely, so the child does not inherit it.
func (e *env) unset(k string) {
	if _, seen := e.values[k]; !seen {
		return
	}
	delete(e.values, k)
	for i, key := range e.keys {
		if key == k {
			e.keys = append(e.keys[:i], e.keys[i+1:]...)
			return
		}
	}
}

func (e *env) slice() []string {
	out := make([]string, 0, len(e.keys))
	for _, k := range e.keys {
		out = append(out, k+"="+e.values[k])
	}
	return out
}
