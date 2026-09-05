// Package install renders bothy's config tree. Everything it writes lands
// under <bothy>/config (ADR-009); it never touches ~/.config/yazi, ~/.vimrc or
// your global git config, and the tools are launched pointed at bothy's tree
// instead. See cmd/bothy/dev.go for the environment that does the pointing.
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/advice"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/layout"
	"github.com/bspeelm/bothy/internal/mux"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
	"github.com/bspeelm/bothy/internal/render"
	"github.com/bspeelm/bothy/internal/slots"
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

	Container bool
	// Opener is the command Yazi hands a file to, and OpenerDesc is what Yazi
	// shows for it. A machine fact rather than a provider one, so it is
	// decided here rather than by conditionals in the template.
	Opener     string
	OpenerDesc string

	// Watermark is the image to sit behind the terminal, already expanded.
	// WatermarkOpacity is not configurable: tune it in an override, which is
	// how every other ghostty setting is tuned.
	Watermark        string
	WatermarkOpacity string
	EditorBin        string
	AgentBin         string
	BrowserBin       string
	VimColorscheme   string

	Font       string
	ProjectDir string

	PaneFrames string

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
	if !opts.DryRun && cfg.Slots.Browser == "yazi" && !cfg.PassesThrough("browser") {
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

func YaziDir(p platform.Info) string { return filepath.Join(p.ConfigRoot(), "yazi") }
func VimDir(p platform.Info) string  { return filepath.Join(p.ConfigRoot(), "vim") }
func VimRC(p platform.Info) string   { return filepath.Join(VimDir(p), "vimrc") }
func GhosttyConf(p platform.Info) string {
	return filepath.Join(p.ConfigRoot(), "ghostty.conf")
}

// plan lists every file to write. Adding a provider should mean adding entries
// here and templates beside them — never new logic.
func plan(p platform.Info, cfg config.Config, data Data) []file {
	var out []file

	for _, slot := range config.SlotNames() {
		// A slot passed through uses the user's own config directory, so
		// writing bothy's version of it would leave files nothing reads.
		if cfg.PassesThrough(slot) {
			continue
		}
		pr, ok := slots.Get(cfg.ProviderFor(slot))
		if !ok {
			continue
		}
		for _, f := range pr.Files {
			if !conditionMet(f.When, cfg, data) {
				continue
			}
			dest := strings.ReplaceAll(f.Dest, "{theme}", data.ThemeName)
			out = append(out, file{
				Dest:     filepath.Join(p.ConfigRoot(), filepath.FromSlash(dest)),
				Tool:     pr.Name,
				Template: f.Template,
			})
		}
	}

	// Not a provider: the shim fills no slot, and it goes in bin/ rather than
	// the config root. Inside Toolbx/Distrobox the opener forwards to the host
	// via flatpak-spawn. Keyed on SharedHome, not InContainer: a plain podman
	// container has no host session, and a shim there would satisfy the opener
	// check without working.
	if p.SharedHome {
		out = append(out, file{
			Dest: filepath.Join(p.BinDir(), "xdg-open"), Tool: "shell",
			Template: "templates/shell/xdg-open.tmpl",
			Exec:     true,
		})
	}

	return out
}

// conditions are the whole vocabulary a provider file's `when` may name.
// A closed set rather than an expression language: parsing one would want a
// dependency, and PLAN.md §13 allows exactly the one already spent on TOML.
// A test asserts every `when` in slots/ is a key here.
var conditions = map[string]func(config.Config, Data) bool{
	"no-images": func(_ config.Config, d Data) bool { return !d.ImagePreviews },
	"provide-editor-config": func(c config.Config, _ Data) bool {
		return c.Editor.ProvideConfig
	},
}

func conditionMet(when string, cfg config.Config, data Data) bool {
	if when == "" {
		return true
	}
	if fn, ok := conditions[when]; ok {
		return fn(cfg, data)
	}
	// An unknown condition writes the file rather than silently skipping it:
	// a typo that hides a config is harder to notice than one that shows it.
	return true
}

// muxGraphics asks the configured backend whether image previews survive it.
func muxGraphics(p platform.Info, cfg config.Config) probe.MuxGraphics {
	bin := muxBinary(cfg)
	if bin == "" {
		return probe.MuxGraphics{None: true}
	}
	b, ok := mux.For(cfg.ProviderOrDefault("mux"))
	if !ok {
		return probe.MuxGraphics{Reason: "no backend for the configured multiplexer"}
	}
	carries, reason := b.Graphics(ToolPath(p, bin))
	return probe.MuxGraphics{Carries: carries, Reason: reason}
}

// buildData assembles the template data, running the graphics probe.
func buildData(p platform.Info, cfg config.Config, pal theme.Palette) Data {
	name := slug(pal.Name)
	g := probe.CheckGraphics(p.Terminal, muxGraphics(p, cfg))

	d := Data{
		Theme:            pal,
		ThemeName:        name,
		ImagePreviews:    g.Supported,
		GraphicsReason:   g.Reason,
		Watermark:        config.Expand(cfg.Workspace.BackgroundImage, p.Home),
		WatermarkOpacity: "0.05",
		Container:        p.InContainer(),
		Opener:           opener(p),
		OpenerDesc:       openerDesc(p),
		EditorBin:        EditorBinary(cfg.Slots.Editor),
		AgentBin:         AgentBinary(cfg.Slots.Agent),
		BrowserBin:       cfg.Slots.Browser,
		Font:             cfg.Theme.Font,
		ProjectDir:       cfg.Workspace.ProjectDir,
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

// opener is the command that hands a file to the desktop. There is no portable
// answer: xdg-open is the freedesktop convention, macOS has `open`, and inside
// a container the app databases live on the host -- so a local xdg-open would
// be a working binary with no viewers behind it.
func opener(p platform.Info) string {
	switch {
	case p.InContainer():
		return "flatpak-spawn --host xdg-open"
	case p.OS == "darwin":
		return "open"
	}
	return "xdg-open"
}

// OpenerBinary is the program the opener runs, for a check that wants to know
// whether it is there. The container opener forwards through flatpak-spawn, so
// that is the binary that has to exist here.
func OpenerBinary(p platform.Info) string {
	return strings.Fields(opener(p))[0]
}

// openerDesc is what Yazi shows beside the opener, which matters when
// the file is leaving this machine's namespace.
func openerDesc(p platform.Info) string {
	if p.InContainer() {
		return "Open on host"
	}
	return "Open"
}

// EditorBinary maps an editor slot to the command it runs. Exported so the
// doctor checks the same binary the launcher uses.
func EditorBinary(slot string) string {
	if slot == "" {
		slot = "vim"
	}
	return advice.Binary(slot)
}

// AgentBinary is the command the agent slot runs as.
func AgentBinary(slot string) string {
	if slot == "" {
		slot = "claude-code"
	}
	return advice.Binary(slot)
}

// Commands maps layout slots to the commands their panes run.
func Commands(cfg config.Config) layout.Commands {
	return layout.Commands{
		"browser": cfg.Slots.Browser,
		"editor":  EditorBinary(cfg.Slots.Editor),
		"agent":   AgentBinary(cfg.Slots.Agent),
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

// SessionEnv builds the environment for bothy's process tree, which is where
// isolation takes effect: the configs are in bothy's directory and nothing
// reads them unless the tools are told to. A handful of variables scoped to
// one process tree, so the user's shell keeps its own PATH and EDITOR.
//
// The doctor uses this too, so checks run tools with the launcher's config.
// terminalSize is a seam: a test has no terminal to ask.
var terminalSize = platform.TerminalSize

func SessionEnv(p platform.Info, cfg config.Config) []string {
	env := newEnv(os.Environ())

	// bothy's own bin first, for this session only.
	env.set("PATH", p.BinDir()+string(os.PathListSeparator)+env.get("PATH"))

	// Passthrough must *unset*, not merely decline to set: the session inherits
	// the current environment, so an already-exported value would stay in place
	// and the tool would use bothy's config anyway.
	// Every backend's variables are unset before the chosen one's are set, so
	// switching multiplexers cannot leave the previous one's config pointed at.
	for _, b := range mux.All() {
		for k := range b.SessionEnv(p) {
			env.unset(k)
		}
	}
	if b, ok := mux.For(cfg.ProviderOrDefault("mux")); ok && !cfg.PassesThrough("mux") {
		for k, v := range b.SessionEnv(p) {
			env.set(k, v)
		}
	}
	if cfg.Slots.Browser == "yazi" && !cfg.PassesThrough("browser") {
		env.set("YAZI_CONFIG_HOME", YaziDir(p))
	} else {
		env.unset("YAZI_CONFIG_HOME")
	}

	// Fedora's nano-default-editor exports EDITOR=nano from /etc/profile.d,
	// and yazi, lazygit and git shell out to it. Setting it here
	// covers bothy's panes without touching the user's shell config.
	editor := EditorBinary(cfg.Slots.Editor)
	env.set("EDITOR", editor)
	env.set("VISUAL", editor)

	// VIMINIT takes precedence over ~/.vimrc, so it is only set when bothy is
	// providing a vim config. Otherwise vim is yours and loads yours.
	if cfg.Slots.Editor == "vim" && cfg.Editor.ProvideConfig && !cfg.PassesThrough("editor") {
		env.set("VIMINIT", "source "+VimRC(p))
	}

	// Cache only (ADR-022). A cache is scratch space, so keeping it here makes
	// uninstall complete without taking anything from anyone. Data and state
	// are not: redirecting those hid nvim's plugins and zoxide's database from
	// the tools that had learned them.
	env.set("XDG_CACHE_HOME", p.CacheDir())

	// Named rather than derived, so a `bothy doctor` typed in the shell pane
	// inspects the workspace it is running in.
	env.set("BOTHY_DIR", p.BothyDir())

	// A pane's command can start before the mux has sized its pty, and yazi
	// exits when the terminal reports 0x0. These are the fallback it reads
	// next, corrected by SIGWINCH once the pane is sized. Unset when nothing
	// was measured: an inherited size would be read as this pane's, and a
	// wrong size is worse than none: none makes a tool ask the ioctl.
	if cols, rows, ok := terminalSize(); ok {
		env.set("COLUMNS", strconv.Itoa(cols))
		env.set("LINES", strconv.Itoa(rows))
	} else {
		env.unset("COLUMNS")
		env.unset("LINES")
	}

	env.set("BOTHY_SESSION", "1")
	return env.slice()
}

// env is an environment being assembled. It replaces rather than appends,
// not a detail: with two PATH entries, which one a process sees
// depends on the libc and on whether anything deduplicated it on the way
// through, and the original stays first -- so bothy's tools are not found.
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
// get reads the copy being assembled, not the process environment.
func (e *env) get(k string) string { return e.values[k] }

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
