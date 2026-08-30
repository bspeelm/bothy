// Package install turns a configuration into files on disk.
//
// It is the only package that knows which template goes where. Everything it
// writes goes through render.Writer, so every file is backed up, recorded in
// the manifest, and removable by `bothy uninstall`.
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bothy "github.com/bothy-dev/bothy"
	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/layout"
	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/probe"
	"github.com/bothy-dev/bothy/internal/render"
	"github.com/bothy-dev/bothy/internal/state"
	"github.com/bothy-dev/bothy/internal/theme"
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
}

// Result reports what an install did, for a human-readable summary.
type Result struct {
	Written   []string
	Unchanged []string
	Skipped   []string
	BackupDir string
	Data      Data
}

// Options configures one install run.
type Options struct {
	DryRun bool
}

// Run writes every config file the configuration calls for.
func Run(p platform.Info, cfg config.Config, opts Options) (*Result, error) {
	pal, err := cfg.Palette(p)
	if err != nil {
		return nil, err
	}

	m, err := state.Load(p.StateDir)
	if err != nil {
		return nil, err
	}

	w := render.NewWriter(m, p.StateDir, p.Home, filepath.Join(p.ConfigDir, "bothy", "overrides"))
	w.DryRun = opts.DryRun

	data := buildData(p, cfg, pal)
	res := &Result{BackupDir: w.BackupDir(), Data: data}

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

	// A Dracula PRO pack ships ready-made Ghostty themes and vim colorschemes.
	// Copying them beats regenerating what already exists — and bothy has no
	// PRO colours of its own to regenerate them from (ADR-006).
	if theme.IsProVariant(cfg.Theme.Variant) {
		if err := copyProAssets(w, p, cfg, res); err != nil {
			return nil, err
		}
	}

	res.Skipped = w.Skipped
	if opts.DryRun {
		return res, nil
	}
	return res, m.Save(p.StateDir)
}

// file is one destination and the template that fills it.
type file struct {
	Dest     string
	Template string // path inside the embedded templates FS
	Exec     bool
	// Literal replaces Template for content bothy generates in Go rather than
	// from a template — the layout KDL, which the layout package renders.
	Literal []byte
}

func renderFile(w *render.Writer, f file, data Data) ([]byte, error) {
	if f.Literal != nil {
		return f.Literal, nil
	}
	src, err := bothy.Templates.ReadFile(f.Template)
	if err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}
	return w.Render(f.Dest, filepath.Base(f.Template), string(src), data)
}

// plan lists every file to write for a configuration. Adding a provider should
// mean adding entries here and templates beside them — never new logic.
func plan(p platform.Info, cfg config.Config, data Data) []file {
	cfgDir := p.ConfigDir
	var out []file

	if cfg.Slots.Mux == "zellij" {
		out = append(out,
			file{
				Dest:     filepath.Join(cfgDir, "zellij", "config.kdl"),
				Template: "templates/mux/zellij/config.kdl.tmpl",
			},
			file{
				Dest:     filepath.Join(cfgDir, "zellij", "themes", data.ThemeName+".kdl"),
				Template: "templates/mux/zellij/theme.kdl.tmpl",
			},
		)
	}

	if cfg.Slots.Browser == "yazi" {
		yazi := filepath.Join(cfgDir, "yazi")
		out = append(out,
			file{Dest: filepath.Join(yazi, "yazi.toml"), Template: "templates/browser/yazi/yazi.toml.tmpl"},
			file{Dest: filepath.Join(yazi, "keymap.toml"), Template: "templates/browser/yazi/keymap.toml.tmpl"},
			file{Dest: filepath.Join(yazi, "init.lua"), Template: "templates/browser/yazi/init.lua.tmpl"},
			file{Dest: filepath.Join(yazi, "theme.toml"), Template: "templates/browser/yazi/theme.toml.tmpl"},
		)
		// The placeholder previewer only exists to stand in for images, so it
		// is only written when images are actually turned off.
		if !data.ImagePreviews {
			out = append(out, file{
				Dest:     filepath.Join(yazi, "plugins", "enter-hint.yazi", "main.lua"),
				Template: "templates/browser/yazi/enter-hint.lua.tmpl",
			})
		}
	}

	if cfg.Slots.Editor == "vim" {
		out = append(out, file{
			Dest:     filepath.Join(p.Home, ".vimrc"),
			Template: "templates/editor/vim/vimrc.tmpl",
		})
		// A PRO pack ships its own colorschemes, which are copied verbatim.
		// For every other palette bothy generates one from the same tokens.
		if !theme.IsProVariant(cfg.Theme.Variant) {
			out = append(out, file{
				Dest:     filepath.Join(p.Home, ".vim", "colors", data.ThemeName+".vim"),
				Template: "templates/theme/vim-colorscheme.vim.tmpl",
			})
		}
	}

	// The terminal is never installed, only configured — it lives on the host,
	// outside anything bothy is willing to touch.
	if cfg.Slots.Terminal == "ghostty" {
		gh := filepath.Join(cfgDir, "ghostty")
		out = append(out,
			file{Dest: filepath.Join(gh, "config"), Template: "templates/terminal/ghostty/config.tmpl"},
		)
		// For PRO variants the pack's own theme file is copied instead.
		if !theme.IsProVariant(cfg.Theme.Variant) {
			out = append(out, file{
				Dest:     filepath.Join(gh, "themes", data.ThemeName),
				Template: "templates/terminal/ghostty/theme.tmpl",
			})
		}
	}

	out = append(out, file{
		Dest:     filepath.Join(p.Home, ".bashrc.d", "bothy.sh"),
		Template: "templates/shell/bothy.sh.tmpl",
	})

	// Only inside a container: on the host this shim would shadow the real
	// xdg-open for every application, which is not bothy's business.
	if p.InContainer() {
		out = append(out, file{
			Dest:     filepath.Join(p.LocalBin, "xdg-open"),
			Template: "templates/shell/xdg-open.tmpl",
			Exec:     true,
		})
	}

	return out
}

// buildData assembles the template data, running the graphics probe.
func buildData(p platform.Info, cfg config.Config, pal theme.Palette) Data {
	name := pal.Name
	if pal.Variant != "" && pal.Variant != "open" {
		name = "dracula-" + pal.Variant
	}

	g := probe.CheckGraphics(muxBinary(cfg), p.Terminal)

	d := Data{
		Theme:            pal,
		ThemeName:        name,
		ImagePreviews:    g.Supported,
		GraphicsReason:   g.Reason,
		Container:        p.InContainer(),
		ContainerName:    cfg.ContainerFor(p),
		EditorBin:        editorBinary(cfg.Slots.Editor),
		AgentBin:         agentBinary(cfg.Slots.Agent),
		BrowserBin:       cfg.Slots.Browser,
		Font:             cfg.Theme.Font,
		ProjectDir:       cfg.Workspace.ProjectDir,
		Watermark:        cfg.Workspace.Watermark,
		WatermarkPath:    filepath.Join(p.ConfigDir, "ghostty", "watermark.png"),
		WatermarkOpacity: "0.05",
	}
	if theme.IsProVariant(pal.Variant) {
		// The pack's own colorschemes, whose names it decides.
		d.VimColorscheme = theme.VimColorscheme(pal.Variant)
	} else {
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

func editorBinary(slot string) string {
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
		"editor":  editorBinary(cfg.Slots.Editor),
		"agent":   agentBinary(cfg.Slots.Agent),
	}
}

// LoadProfile reads a profile by name, preferring the user's own copy in
// ~/.config/bothy/profiles over the shipped one.
func LoadProfile(p platform.Info, name string) (layout.Profile, error) {
	user := filepath.Join(p.ConfigDir, "bothy", "profiles", name+".toml")
	if _, err := os.Stat(user); err == nil {
		return layout.LoadProfile(user)
	}
	src, err := bothy.Profiles.ReadFile("profiles/" + name + ".toml")
	if err != nil {
		return layout.Profile{}, fmt.Errorf("install: no profile named %q", name)
	}
	tmp, err := os.CreateTemp("", "bothy-profile-*.toml")
	if err != nil {
		return layout.Profile{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(src); err != nil {
		tmp.Close()
		return layout.Profile{}, err
	}
	tmp.Close()
	return layout.LoadProfile(tmp.Name())
}

// copyProAssets copies the ready-made files out of a user's Dracula PRO pack.
func copyProAssets(w *render.Writer, p platform.Info, cfg config.Config, res *Result) error {
	pack := cfg.ProPackPath(p)
	variant := cfg.Theme.Variant

	// Ghostty theme: the pack ships one per variant, already in Ghostty's format.
	if cfg.Slots.Terminal == "ghostty" {
		src := theme.GhosttyThemeFile(pack, variant)
		body, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("install: reading %s from the pack: %w", src, err)
		}
		dest := filepath.Join(p.ConfigDir, "ghostty", "themes", "dracula-"+variant)
		changed, err := w.Write(dest, body)
		if err != nil {
			return err
		}
		record(res, dest, changed)
	}

	// vim colorschemes: all of them, so switching variant later needs no reinstall.
	if cfg.Slots.Editor == "vim" {
		dir := filepath.Join(pack, theme.DefaultPackLayout.VimColors)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("install: reading %s from the pack: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".vim") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return fmt.Errorf("install: %w", err)
			}
			dest := filepath.Join(p.Home, ".vim", "colors", e.Name())
			changed, err := w.Write(dest, body)
			if err != nil {
				return err
			}
			record(res, dest, changed)
		}
	}
	return nil
}

func record(res *Result, dest string, changed bool) {
	if changed {
		res.Written = append(res.Written, dest)
	} else {
		res.Unchanged = append(res.Unchanged, dest)
	}
}

// GitSettings are the `git config --global` keys bothy sets for delta.
// Previous values are recorded so uninstall can put them back.
func GitSettings() []state.GitSetting {
	return []state.GitSetting{
		{Key: "core.pager", Value: "delta"},
		{Key: "interactive.diffFilter", Value: "delta --color-only"},
		{Key: "delta.navigate", Value: "true"},
		{Key: "delta.line-numbers", Value: "true"},
		{Key: "delta.syntax-theme", Value: "Dracula"},
		{Key: "merge.conflictstyle", Value: "zdiff3"},
	}
}

// ApplyGitSettings wires delta up as git's pager, remembering what was there.
func ApplyGitSettings(m *state.Manifest, dryRun bool) error {
	for _, g := range GitSettings() {
		out, err := exec.Command("git", "config", "--global", "--get", g.Key).Output()
		if err == nil {
			g.Previous = strings.TrimSpace(string(out))
			g.HadPrevious = true
		}
		if g.Previous == g.Value {
			m.RecordGitSetting(g)
			continue
		}
		if !dryRun {
			if err := exec.Command("git", "config", "--global", g.Key, g.Value).Run(); err != nil {
				return fmt.Errorf("install: git config %s: %w", g.Key, err)
			}
		}
		m.RecordGitSetting(g)
	}
	return nil
}
