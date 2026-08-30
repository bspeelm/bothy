
# CLI Dev Workspace Cheat Sheet — Toolbox site-dev flow

*Fedora Silverblue 44 host · Fedora 44 Toolbx container · driving Claude Code from the CLI*
*Stack: **Ghostty** (host terminal, rpm-ostree) → **Zellij** (multiplexer) → **Yazi** (file tree + browser) → **Claude Code***

The workspace = one command (`dev`) that opens a persistent 4-region layout:

```
┌──────────────────────────────────────────────────┐
│         Yazi — tree + browser, 100% w, 50% h     │
├─────────────────────────────┬────────────────────┤
│   MAIN: claude (focused)    │  SIDE: shell  40%  │
│              60%            │  git/builds/logs   │
└─────────────────────────────┴────────────────────┘
   + Zellij tab-bar & status-bar = on-screen key hints
```

Everything except the terminal lives **inside the toolbox**. Ghostty is the host window it runs in.

---

> **Note on colours.** This document is reproduced as the historical record of
> the setup bothy was built from — every trap described here became a `doctor`
> check. That original setup was themed with **Dracula PRO**, which is a paid
> pack; its palette values have been replaced throughout with the equivalent
> **open Dracula** ones so that nothing paid is redistributed here. The lessons
> are in the traps, not the hex codes. bothy supports Dracula PRO by pointing at
> your own licensed copy of the pack — see `docs/decisions.md` ADR-006.

---


## 1. Host — install Ghostty (rpm-ostree layer, same as VS Code)

Run on the **host terminal**, NOT inside the toolbox.

```bash
# F44-compatible COPR (scottames — verified F44 builds; avoid pgdev, no F44)
sudo curl -fsSL -o /etc/yum.repos.d/_copr_scottames-ghostty.repo \
  https://copr.fedorainfracloud.org/coprs/scottames/ghostty/repo/fedora-44/scottames-ghostty-fedora-44.repo
sudo rpm-ostree install ghostty
systemctl reboot
```

- Layers like any other rpm-ostree package; rides along with `rpm-ostree upgrade`.
- ⚠️ Do **not** use the `pgdev/ghostty` COPR — no F44 builds, it will **block system upgrades**.
- Fallback COPR if scottames ever breaks: `alternateved/ghostty` (also has F44 builds).
- Ghostty is why Yazi can show **inline image previews**. GNOME Terminal works too, just no images.

After reboot: launch Ghostty → `toolbox enter <name>`.

## 2. Toolbox — install Zellij + Yazi (+ companions)

Run **inside the toolbox**.

```bash
# Zellij — varlad is still the only source (fc43 build, runs fine on F44)
sudo dnf copr enable varlad/zellij -y

# Yazi — use boobaa/yazi, NOT varlad/yazi. See the warning below.
sudo dnf copr enable boobaa/yazi -y

sudo dnf install -y zellij yazi

# Yazi preview / navigation helpers
sudo dnf install -y ffmpeg-free p7zip jq poppler-utils fd-find ripgrep fzf zoxide ImageMagick

# Optional: great TUI git for the side pane
# NOT in the F44 repos — needs its own COPR
sudo dnf copr enable atim/lazygit -y
sudo dnf install -y lazygit
```

> ⚠️ **Do not use `varlad/yazi`** — it is pinned at **25.5.31 (May 2025)** and every
> current `yazi-rs` plugin refuses to load on it (`Plugin 'git' requires at least
> Yazi 26.5.6`). Yazi is **not** in the Fedora repos at all. Use **`boobaa/yazi`**:
> it ships 26.5.6 built natively for `fedora-44-x86_64` and contains exactly one
> package. Avoid `ldivizio/tools` and `kray74/cli-tools` — they also carry 26.x but
> bundle 10+ packages that shadow `ripgrep` / `fd` / `fzf` / `jq` / `delta` from
> Fedora proper.
>
> Find newer builds without guessing:
> ```bash
> curl -s "https://copr.fedorainfracloud.org/api_3/project/search?query=yazi"
> curl -s "https://copr.fedorainfracloud.org/api_3/package?ownername=OWNER&projectname=PROJ&packagename=yazi&with_latest_build=true"
> ```
>
> Upgrading 25.x → 26.x renames two config keys: `[filetype]` rules take **`url`**
> instead of `name`, and `prepend_fetchers` takes `url` + `group` instead of `id`.

## 2b. Ghostty terminfo inside the toolbox

The toolbox image has no `xterm-ghostty` entry, so `toolbox enter` greets you with
`Error: terminfo entry not found for xterm-ghostty`. Copy it in from the host
(`~` is shared, so `~/.terminfo` covers every toolbox at once):

```bash
mkdir -p ~/.terminfo/x
cp /run/host/usr/share/terminfo/x/xterm-ghostty ~/.terminfo/x/
infocmp xterm-ghostty >/dev/null && echo OK
```

## 3. Zellij layout file

Create `~/.config/zellij/layouts/dev.kdl` (inside the toolbox).

**Two KDL gotchas, both learned the hard way:**

1. `split_direction` is the opposite of what it sounds like — **`"vertical"` puts
   panes side by side as columns**; the default (`"horizontal"`) stacks them as rows.
2. A `{ plugin location="…" }` written on **one line needs a `;`** after the node,
   or you get `Failed to deserialize KDL node`. Put it on its own line instead.

Sanity check: `pane size=1` for the tab-bar only makes sense as a 1-line *row*, which
tells you the top level stacks rows. To verify what zellij actually built, read
`~/.cache/zellij/<ver>/session_info/<session>/session-layout.kdl` — the resolved truth.

```kdl
layout {
    pane size=1 borderless=true {
        plugin location="zellij:tab-bar"
    }
    // Top: Yazi = file tree AND browser, full width
    pane size="50%" {
        command "yazi"
    }
    // Bottom: claude + side shell as COLUMNS (vertical = side by side)
    pane split_direction="vertical" {
        pane focus=true name="claude" {
            command "claude"
        }
        pane size="40%" name="side"
    }
    pane size=2 borderless=true {
        plugin location="zellij:status-bar"
    }
}
```

```
┌──────────────────────────────────────────────────┐  tab-bar
│              Yazi — 100% w, 50% h                │
├─────────────────────────────┬────────────────────┤
│      claude  60%            │    side  40%       │
├─────────────────────────────┴────────────────────┤  status-bar
```

Layout changes apply **at launch only** — `Ctrl-q`, then `dev` again.
Live-tune with `Ctrl-n` + arrows (5% per press), then write the number back here.

## 3b. Yazi + shell polish

`~/.config/yazi/yazi.toml` — the middle column eats ~50% of the pane by default:

```toml
# Yazi 25.5+ — section is [mgr] (renamed from [manager] in 25.4)
[mgr]
# parent | current | preview     (default = [1, 4, 3])
ratio = [2, 2, 4]   # parent == current; preview gets half, it's the useful one
show_hidden = true  # .gitignore / .env / .vscode are half the config in a repo
```

> §9 has the complete file. The snippets in this section are illustrative.

**Image previews + Zellij — don't fight this one.** Zellij (0.42) can't pass the
Kitty graphics protocol through, and it also mangles the terminal's reply when Yazi
queries for cell size / graphics support. Two symptoms, one root cause:

| Symptom | Why |
|---|---|
| Previews are low-res block art | Falls back to `chafa`, which maps to character cells |
| Phantom **"Find next"** on every image preview | Mangled query reply is parsed as keystrokes (`n` = `find_arrow`) |

Diagnosis tools: `yazi --debug` prints `Emulator` / `Adapter.matches` / dependencies.
`grep -a "Find next" /usr/bin/yazi` vs `/usr/bin/zellij` proves which program owns
a mystery string. **The decisive test is running `yazi` in a bare Ghostty tab** —
outside Zellij images are crisp and the phantom key never fires.

Chosen fix: turn image previews off inside the layout and press `Enter` to view the
real image in the host viewer.

```toml
[plugin]
prepend_previewers = [{ mime = "image/*", run = "enter-hint" }]
prepend_preloaders = [{ mime = "image/*", run = "empty" }]
```

`enter-hint` is a tiny local plugin (§9) that prints *"(press Enter to preview)"*.
The built-in `empty` previewer works too, but it labels the file **"Empty file"** —
which is a lie: the file has content, it just isn't being drawn.

> Alternative if you want true previews: run Yazi in a native **Ghostty split**
> (`ctrl+shift+e` down / `ctrl+shift+o` right) and keep Zellij for claude + side.

⚠️ On a bad config Yazi prints *"Press &lt;Enter&gt; to continue with preset settings"*
and **silently ignores your whole file**. Validate after every edit:
`yazi 2>&1 | grep -iE "must be|invalid|preset"`. (`image_delay` max is 100, not
free-form — that one bites.)

**`Enter` on a png/jpg → "No such file or directory"**: the toolbox has no
`xdg-open`, which is Yazi's default opener. Do **not** just `dnf install xdg-utils` —
that gets you a working binary with nothing to hand off to (`gio mime image/png`
in the container = *"No default applications"*; only the host has Loupe et al.).
The D-Bus session bus is shared, but app databases are read from the filesystem,
so that doesn't help. Hand the open to the host instead:

```toml
[opener]
open = [
    { run = 'flatpak-spawn --host xdg-open "$@"', desc = "Open on host", orphan = true },
]
```

> Yazi-only. If lazygit / `gh browse` / anything else needs it too, drop a guarded
> shim at `~/.local/bin/xdg-open` that does `flatpak-spawn --host xdg-open "$@"`
> when `/run/.containerenv` exists, and `exec /usr/bin/xdg-open` otherwise — the
> guard is required, or the shared home makes the host recurse into itself.

Add to `~/.bashrc` — Fedora's `nano-default-editor` package forces `EDITOR=nano`
in `/etc/profile.d/`, which is what Yazi/lazygit/git shell out to:

```bash
# Host has only vi (vim-minimal); toolbox has full vim — pick what exists
if command -v vim >/dev/null 2>&1; then export EDITOR=vim
elif command -v vi  >/dev/null 2>&1; then export EDITOR=vi
fi
export VISUAL="$EDITOR"

# Compact prompt — a narrow side pane can't spare "user@host".
# Surgical edit, NOT a fresh PS1, so the toolbox ⬢ marker survives:
#   ⬢ [user@host dir]$   ->   ⬢ dir$
if [ -n "$PS1" ]; then
    PS1="${PS1//\\u@\\h /}"      # drop user@host
    PS1="${PS1//\[\\W\]/\\W}"    # drop the brackets around the dir
fi
```

## 4. One-command launcher

Add to `~/.bashrc`. Note `~/.bashrc` is **shared between host and toolbox**, but
zellij only exists inside the box — a plain alias gives you `zellij: command not
found` on the host. This function works from either side:

```bash
dev() {
    local dir="${PROJECT_DIR:-$PWD}"
    if [ -f /run/.containerenv ]; then
        cd "$dir" && zellij --layout dev
    else
        toolbox run --container "${DEV_TOOLBOX:-aip}" \
            bash -lc "cd '$dir' && zellij --layout dev"
    fi
}
# Optional: pin the site so `dev` always opens there
# export PROJECT_DIR=/path/to/site
# Optional: use a different toolbox
# export DEV_TOOLBOX=aip
```

`cd` into the repo → run **`dev`**. Reattach after any disconnect → **`zellij attach`**.

---

## 4b. Dracula PRO theming

Source pack: `~/Documents/Dracula_Theme` (palette in `design/palette.md`). Variants:
`pro` · `blade` · `buffy` · `lincoln` · `morbius` · `van-helsing` · `alucard` (light).

| Tool | Install | Activate |
|---|---|---|
| Ghostty | `cp themes/ghostty/pro ~/.config/ghostty/themes/dracula-pro` | `theme = dracula-pro` in `~/.config/ghostty/config` |
| Zellij | write `~/.config/zellij/themes/dracula-pro.kdl` | `theme "dracula-pro"` in `config.kdl` |
| Yazi | write `~/.config/yazi/theme.toml` | automatic |
| Vim | `cp themes/vim/colors/*.vim ~/.vim/colors/` | `colorscheme dracula_pro` in `~/.vimrc` |
| delta | — | `git config --global delta.syntax-theme Dracula` |

⚠️ **Ghostty reads `~/.config/ghostty/config` — no extension.** A file named
`config.ghostty` is silently ignored.

⚠️ **Vim: put colorschemes in `~/.vim/colors/`, not `pack/themes/start/`** (which is
what the pack's own install.md says). `pack/*/start` is added to `runtimepath`
*after* `.vimrc` is sourced, so `colorscheme` can't find it — and `packadd!` only
searches `opt/`. It fails silently. Verify with:
```bash
vim -es -u ~/.vimrc -c "call writefile([get(g:,'colors_name','NONE')],'/tmp/cs')" -c q; cat /tmp/cs
```
(`vim -es` alone does **not** source `~/.vimrc` — the `-u` is required, or you're
testing nothing.)

**Watermark behind the side pane.** A terminal can't scope an image to one pane,
but Ghostty 1.3+ has `background-image`, and the `dev` layout puts `side` at
bottom-right — so the image can be made to *land* there.

⚠️ **Do not use `fit = none` + a corner anchor.** That positions in absolute
pixels, so it is correct on exactly one monitor: too small and too low on 4K,
oversized on 2K, and it spills below the pane into the status bar.

Instead build a **mostly-transparent canvas the shape of the window**, composite
the art at the *percentage* where the pane sits, and stretch it. `fit = stretch`
maps the image onto the window, so those percentages hold at any resolution:

```bash
# side pane measured at x 60-99%, y 47-93% of the window
magick -size 1920x1080 xc:none \
  \( art.png -resize 232x324! \) -geometry +1621+626 -composite  tux.png
```
```
background-image = /path/to/tux.png
background-image-opacity = 0.04     # ASCII art over text needs to be VERY faint
background-image-fit = stretch      # `position` is ignored under stretch
background-image-repeat = false
```

Leave extra margin at the **bottom**: pane sizes are percentages and scale
cleanly, but the tab-bar/status-bar are fixed line counts, so at lower
resolutions they eat a bigger fraction and push the pane's bottom edge up.

Opacity guide (on the `#282A36` Dracula background): a solid silhouette reads at
`0.10`; sparse line-art ASCII needs `~0.28`; **dense** ASCII over live text wants
`0.04–0.07` or it garbles the glyphs on top of it. Below `0.04` it vanishes.

Restrictions on the image: PNG/JPEG (no SVG — rasterise first), no animation,
alpha is honoured and effectively required. Valid `fit` values are
`contain cover stretch none`; valid `position` values are the nine
`top/center/bottom`-`left/center/right` combinations plus `center`.

## 4c. Plugins that earn their place

```bash
# Yazi (needs Yazi >= 26 — see the COPR warning in §2)
ya pkg add yazi-rs/plugins:git          # per-file git status in the listing
ya pkg add yazi-rs/plugins:chmod        # c m — chmod without dropping to a shell
ya pkg add yazi-rs/plugins:smart-enter  # l — open file OR enter dir
ya pkg add yazi-rs/plugins:full-border
ya pkg upgrade                          # after every Yazi upgrade

# Vim — native vim8 packages, no plugin manager
mkdir -p ~/.vim/pack/plugins/start && cd $_
git clone --depth 1 https://github.com/tpope/vim-commentary.git   # gcc / gc
git clone --depth 1 https://github.com/junegunn/fzf.vim.git       # :Files :Rg
```

`fzf.vim` needs the base fzf vim plugin — Fedora's `fzf` package already installs it
at `/usr/share/vim/vimfiles/plugin/fzf.vim`, so nothing extra is required.

`git.yazi` needs BOTH `init.lua` (`require("git"):setup{}`) and a
`[[plugin.prepend_fetchers]]` block in `yazi.toml`. Plugin READMEs track Yazi HEAD,
so on an older Yazi the documented keys may be rejected — trust the error, not the README.

**Zellij: skip plugins.** `zjstatus` is the popular one but replaces the status bar
with a fiddly format-string config; the built-in bar already shows the key hints.

**CLI worth having:** `git-delta` (syntax-highlighted diffs — wire it up with
`git config --global core.pager delta`), plus `awscli2` / `glab` if the project
needs them. Check the git remote before installing `gh` — a GitLab repo wants `glab`.

⚠️ The layout starts its own `claude` — running `dev` from inside a Claude session
gives you a second one. Exit first.

---

## 5. Daily driving — the keys you actually need

Zellij is **modal**: hit a mode key, then act. The bottom bar always shows what's available.

| Do this | Keys |
|---|---|
| Move focus between panes | `Ctrl-p` then arrows / `hjkl` (or just **click**) |
| Resize panes | `Ctrl-n` then arrows |
| Fullscreen the focused pane (toggle) | `Ctrl-p` then `f` |
| New tab / switch tabs | `Ctrl-t` then `n` / `Tab` |
| Scroll up in a pane | `Ctrl-s` then arrows / PgUp |
| Detach (leave it running) | `Ctrl-o` then `d` |
| Reattach later | `zellij attach` (or `zellij attach <name>`) |
| Quit the whole session | `Ctrl-q` |

Yazi (left pane): arrows / `hjkl` to move, `Enter` opens, `q` quits, `~` or `F1` = help.

---

## 6. Tweaks

- **Auto-run a tool in the side pane:** give it a command in the KDL, e.g.
  `pane size="35%" name="side" { command "lazygit" }`.
- **Second Claude for side tasks:** `command "claude"` in the side pane too.
- **Want a dedicated collapsible fuzzy tree** instead of Yazi on the left:
  `sudo dnf install broot`, then swap the left pane to `command "broot"`.
- **No image previews?** Confirm you're in Ghostty, not GNOME Terminal (`echo $TERM`).

---

## 7. Quick reference

| Task | Command |
|---|---|
| Install Ghostty (host) | `rpm-ostree install ghostty` (COPR from §1) |
| Install workspace (toolbox) | `sudo dnf install zellij yazi lazygit` |
| Launch workspace | `dev`  (= `zellij --layout dev`) |
| Reattach after drop | `zellij attach` |
| List sessions | `zellij list-sessions` |
| Kill a session | `zellij kill-session <name>` |
| Edit the layout | `~/.config/zellij/layouts/dev.kdl` |
| Versions | `ghostty --version` · `zellij --version` · `yazi --version` |

---
*Zellij keeps sessions alive while its server runs (survives terminal/SSH drops), but NOT across a
full host reboot — just run `dev` again. Nothing here touches the base image except Ghostty in §1.*

---

## 8. Transfer to a new machine — ordered checklist

Assumes Fedora Silverblue + a Fedora toolbox. Everything below §8.1 runs **inside
the toolbox** unless stated. §9 has the complete contents of every config file.

**8.1 Host** — layer Ghostty (§1), reboot, then `toolbox create <name>`.

**8.2 Toolbox packages**

```bash
sudo dnf copr enable varlad/zellij -y      # zellij: only source
sudo dnf copr enable boobaa/yazi   -y      # yazi 26.x — NOT varlad/yazi (§2)
sudo dnf copr enable atim/lazygit  -y
sudo dnf install -y zellij yazi lazygit \
  ffmpeg-free p7zip jq poppler-utils fd-find ripgrep fzf zoxide ImageMagick \
  vim git-delta awscli2 glab
```

**8.3 Ghostty terminfo** (§2b) — else `toolbox enter` errors on `xterm-ghostty`:

```bash
mkdir -p ~/.terminfo/x && cp /run/host/usr/share/terminfo/x/xterm-ghostty ~/.terminfo/x/
```

**8.4 Config files** — create everything in §9. Directories needed:

```bash
mkdir -p ~/.config/zellij/{layouts,themes} ~/.config/yazi/plugins/enter-hint.yazi \
         ~/.config/ghostty/themes ~/.vim/{colors,undo,backup,swap} \
         ~/.vim/pack/plugins/start
```

**8.5 Yazi plugins** (needs Yazi ≥ 26)

```bash
ya pkg add yazi-rs/plugins:git
ya pkg add yazi-rs/plugins:chmod
ya pkg add yazi-rs/plugins:smart-enter
ya pkg add yazi-rs/plugins:full-border
```

**8.6 Vim plugins**

```bash
cd ~/.vim/pack/plugins/start
git clone --depth 1 https://github.com/tpope/vim-commentary.git
git clone --depth 1 https://github.com/junegunn/fzf.vim.git
```

**8.7 Dracula PRO** — from the theme pack (`~/Documents/Dracula_Theme`):

```bash
cp ~/Documents/Dracula_Theme/themes/vim/colors/*.vim ~/.vim/colors/
cp ~/Documents/Dracula_Theme/themes/ghostty/pro ~/.config/ghostty/themes/dracula-pro
rm -rf ~/Documents/Dracula_Theme/__MACOSX          # 890 files of macOS junk
```

**8.8 delta as the git pager**

```bash
git config --global core.pager "delta"
git config --global interactive.diffFilter "delta --color-only"
git config --global delta.navigate true
git config --global delta.syntax-theme "Dracula"
git config --global delta.line-numbers true
git config --global merge.conflictstyle "zdiff3"
```

**8.9 Tux watermark** — regenerate from the ASCII in §9, then re-measure the pane
percentages against your own screenshot if the layout differs (§4b).

**8.10 Verify** — each of these should print nothing but success:

```bash
zellij setup --check                      # config parses
yazi 2>&1 | grep -iE "must be|invalid|preset"   # SILENT = valid; any output = config DISCARDED
vim -es -u ~/.vimrc -c "call writefile([get(g:,'colors_name','NONE')],'/tmp/cs')" -c q; cat /tmp/cs
ghostty +validate-config                  # run on the HOST
```

> **Faster alternative:** if you still have the old machine, skip §9 entirely and
> copy the live files — home is shared with the host, so a plain tar works:
> ```bash
> tar czf workspace-config.tgz -C ~ .config/zellij .config/yazi .config/ghostty \
>     .vimrc .vim/colors .terminfo
> ```
> `~/.vim/pack` and `~/.local/state/yazi` are omitted on purpose — re-clone the
> vim plugins and re-run `ya pkg add` instead of copying git checkouts.

---

## 9. Appendix — complete config files

Generated from the working setup. Paths are relative to `~`.
Everything here lives in the **shared home**, so it is identical on host and toolbox.

### `~/.config/zellij/themes/dracula-pro.kdl`

```kdl
// Dracula PRO — palette from ~/Documents/Dracula_Theme/design/palette.md
// bg #282A36 is the PRO background (plain Dracula uses #282A36).
themes {
    dracula-pro {
        fg      "#F8F8F2"
        bg      "#44475A"
        black   "#282A36"
        red     "#FF5555"
        green   "#50FA7B"
        yellow  "#F1FA8C"
        blue    "#BD93F9"
        magenta "#FF79C6"
        cyan    "#8BE9FD"
        white   "#F8F8F2"
        orange  "#FFB86C"
    }
}
```

### `~/.config/yazi/yazi.toml`

```toml
# Yazi 26.5.6 — section is [mgr] (renamed from [manager] in 25.4).
[mgr]
# parent | current | preview     (default = [1, 4, 3], middle ate ~50%)
# parent == current, preview gets half the pane since it's the useful one.
ratio = [2, 2, 4]
show_hidden = true    # .gitignore, .env.example, .vscode … all matter in this repo

# Image previews are OFF inside Zellij, deliberately.
#
# Zellij (0.42) can't pass the Kitty graphics protocol through, so Yazi fell back
# to chafa block-art. Worse, drawing an image makes Yazi query the terminal for
# cell size / graphics support; Zellij mangles the reply, and the stray bytes get
# parsed as keystrokes — which is what made a phantom "Find next" fire on every
# image preview. Verified: outside Zellij, in a bare Ghostty tab, images are crisp
# and the phantom keypress never happens.
#
# Routing image/* away from the image previewer stops the draw, so Yazi never
# queries the terminal and the stray keypress goes away. Press Enter to see the
# real image in the host viewer (see [opener] below).
#
# `enter-hint` is a local plugin (plugins/enter-hint.yazi/main.lua) — the built-in
# `empty` previewer would say "Empty file", which is a lie: the file has content,
# we're just not drawing it.
[plugin]
prepend_previewers = [
    { mime = "image/*", run = "enter-hint" },
]
prepend_preloaders = [
    { mime = "image/*", run = "empty" },
]

# git.yazi — per-file status in the listing. Needs Yazi >= 26.5.6.
[[plugin.prepend_fetchers]]
url   = "*"
run   = "git"
group = "git"

[[plugin.prepend_fetchers]]
url   = "*/"
run   = "git"
group = "git"

[preview]
max_width  = 900
max_height = 900

# The toolbox has no xdg-open (and no GUI apps anyway) — Yazi's default opener
# is `xdg-open`, hence "No such file or directory" on png/jpg. Hand it to the
# HOST instead, so files open in the real desktop viewer.
[opener]
open = [
    { run = 'flatpak-spawn --host xdg-open "$@"', desc = "Open on host", orphan = true },
]
```

### `~/.config/yazi/keymap.toml`

```toml
# prepend_keymap wins over the defaults; everything not listed here is untouched.

[[mgr.prepend_keymap]]
on   = "l"
run  = "plugin smart-enter"
desc = "Enter the child directory, or open the file"

[[mgr.prepend_keymap]]
on   = [ "c", "m" ]
run  = "plugin chmod"
desc = "Chmod on selected files"
```

### `~/.config/yazi/init.lua`

```lua
-- Per-file git status in the listing (registered as a fetcher in yazi.toml).
require("git"):setup {
	order = 1500,
}

-- Close the pane borders so the Dracula border colour reads as a full frame.
require("full-border"):setup {
	type = ui.Border.ROUNDED,
}
```

### `~/.config/yazi/plugins/enter-hint.yazi/main.lua`

```lua
--- Placeholder previewer.
--- Image previews are disabled inside Zellij (it can't pass the Kitty graphics
--- protocol, and its mangled query replies fire a phantom "Find next"), so this
--- stands in for the built-in `empty` previewer, whose "Empty file" text is
--- misleading — the file isn't empty, we just aren't drawing it.
local M = {}

function M:peek(job)
	local hint = ui.Text({
		ui.Line(""),
		ui.Line("(press Enter to preview)"),
	}):area(job.area):align(ui.Align.CENTER)

	ya.preview_widget(job, hint)
end

function M:seek() end

return M
```

### `~/.config/ghostty/config`

```ini
# Ghostty config
# NOTE: Ghostty reads exactly this file — ~/.config/ghostty/config (no extension).
# The old empty `config.ghostty` sitting next to it was never being loaded.

theme = dracula-pro

# --- ASCII Tux watermark ---------------------------------------------------
#
# tux.png is a mostly-transparent 1920x1080 canvas with Tux composited at
# x 84.4-96.5%, y 58-88% — the fraction of the window where the `side` pane sits.
#
# `fit = stretch` is doing the real work: it maps the image onto the window, so
# those percentages hold at ANY resolution. The obvious approach (`fit = none`
# plus a small image nudged with transparent padding) positions in absolute
# pixels, so it drifts out of the pane the moment the monitor changes — small and
# too low on 4K, oversized on 2K.
#
# `position` is ignored under stretch. Re-measure the percentages if the dev
# layout's pane sizes ever change.
background-image = /var/home/shadowthebearded/.config/ghostty/tux.png
background-image-opacity = 0.04
background-image-fit = stretch
background-image-repeat = false
```

### `~/.vimrc`

```vim
" ---------------------------------------------------------------------------
" Dracula PRO — schemes live in ~/.vim/colors/. NOT installed as a vim8 package:
" pack/*/start is added to 'runtimepath' AFTER .vimrc is sourced, so a
" `colorscheme` call here wouldn't find it (and `packadd!` only searches opt/).
" Variants: dracula_pro_{blade,buffy,lincoln,morbius,van_helsing,alucard}.
" `alucard` is the light one.
" ---------------------------------------------------------------------------
if has('termguicolors')
    set termguicolors
endif
syntax enable
colorscheme dracula_pro

" --- sane minimums ---------------------------------------------------------
set number relativenumber      " hybrid line numbers
set ignorecase smartcase       " case-insensitive until you type a capital
set incsearch hlsearch
set expandtab shiftwidth=2 softtabstop=2   " matches the JSX/SCSS in aip-app-site
set autoindent smartindent
set scrolloff=4
set hidden                     " switch buffers without saving
set mouse=a
set clipboard=unnamedplus      " share the system clipboard
set wildmenu wildmode=longest:full,full
set undofile undodir=~/.vim/undo
set backupdir=~/.vim/backup directory=~/.vim/swap

" Python lambdas in api/ want 4-space indents
autocmd FileType python setlocal shiftwidth=4 softtabstop=4

" --- plugins ---------------------------------------------------------------
" vim-commentary  : gcc = toggle line, gc = toggle motion/selection
" fzf.vim         : needs the base fzf plugin, which Fedora's fzf package
"                   already drops in /usr/share/vim/vimfiles/plugin/fzf.vim
" Both live in ~/.vim/pack/plugins/start/ — no plugin manager involved.
let mapleader = " "
nnoremap <leader>f :Files<CR>
nnoremap <leader>g :Rg<CR>
nnoremap <leader>b :Buffers<CR>

" Comment JSX with // rather than the HTML-ish default
autocmd FileType javascriptreact,javascript setlocal commentstring=//\ %s

" <Esc> clears search highlight
nnoremap <silent> <Esc> :nohlsearch<CR>
```

### `~/.config/yazi/theme.toml`

```toml
# Dracula PRO for Yazi — palette from ~/Documents/Dracula_Theme/design/palette.md
#   bg #282A36 · comment #6272A4 · selection #44475A · fg #F8F8F2
#   cyan #8BE9FD · green #50FA7B · orange #FFB86C · pink #FF79C6
#   purple #BD93F9 · red #FF5555 · yellow #F1FA8C

[mgr]
cwd = { fg = "#8BE9FD" }

hovered         = { fg = "#282A36", bg = "#BD93F9" }
preview_hovered = { underline = true }

find_keyword  = { fg = "#F1FA8C", italic = true }
find_position = { fg = "#FF79C6", bg = "reset", italic = true }

marker_copied   = { fg = "#50FA7B", bg = "#50FA7B" }
marker_cut      = { fg = "#FF5555", bg = "#FF5555" }
marker_marked   = { fg = "#8BE9FD", bg = "#8BE9FD" }
marker_selected = { fg = "#F1FA8C", bg = "#F1FA8C" }

tab_active   = { fg = "#282A36", bg = "#BD93F9" }
tab_inactive = { fg = "#F8F8F2", bg = "#44475A" }
tab_width    = 1

border_symbol = "│"
border_style  = { fg = "#6272A4" }

[mode]
normal_main = { fg = "#282A36", bg = "#BD93F9", bold = true }
normal_alt  = { fg = "#BD93F9", bg = "#44475A" }
select_main = { fg = "#282A36", bg = "#50FA7B", bold = true }
select_alt  = { fg = "#50FA7B", bg = "#44475A" }
unset_main  = { fg = "#282A36", bg = "#FF79C6", bold = true }
unset_alt   = { fg = "#FF79C6", bg = "#44475A" }

[status]
progress_label  = { fg = "#F8F8F2", bold = true }
progress_normal = { fg = "#BD93F9", bg = "#44475A" }
progress_error  = { fg = "#FF5555", bg = "#44475A" }

[input]
border   = { fg = "#BD93F9" }
title    = {}
value    = {}
selected = { reversed = true }

[pick]
border   = { fg = "#BD93F9" }
active   = { fg = "#FF79C6", bold = true }
inactive = {}

[confirm]
border     = { fg = "#BD93F9" }
title      = { fg = "#BD93F9" }
content    = {}
list       = {}
btn_yes    = { reversed = true }
btn_no     = {}

[tasks]
border  = { fg = "#BD93F9" }
title   = {}
hovered = { fg = "#FF79C6", underline = true }

[which]
mask            = { bg = "#44475A" }
cand            = { fg = "#8BE9FD" }
rest            = { fg = "#6272A4" }
desc            = { fg = "#FF79C6" }
separator       = "  "
separator_style = { fg = "#44475A" }

[help]
on      = { fg = "#8BE9FD" }
run     = { fg = "#FF79C6" }
hovered = { reversed = true, bold = true }
footer  = { fg = "#44475A", bg = "#F8F8F2" }

[notify]
title_info  = { fg = "#50FA7B" }
title_warn  = { fg = "#FFB86C" }
title_error = { fg = "#FF5555" }

# git.yazi status signs
[git]
untracked = { fg = "#6272A4" }
ignored   = { fg = "#44475A" }
modified  = { fg = "#FFB86C" }
added     = { fg = "#50FA7B" }
deleted   = { fg = "#FF5555" }
updated   = { fg = "#F1FA8C" }

[filetype]
rules = [
    { mime = "image/*", fg = "#FFB86C" },
    { mime = "{audio,video}/*", fg = "#FF79C6" },
    { mime = "application/{zip,gzip,x-tar,x-bzip*,x-7z*,x-rar,xz}", fg = "#FF79C6" },
    { mime = "application/{json,x-ndjson}", fg = "#F1FA8C" },
    { mime = "text/*", fg = "#F1FA8C" },
    # 26.x renamed the `name` key to `url` in filetype rules
    { url = "*/", fg = "#BD93F9" },
    { url = "*", fg = "#F8F8F2" },
]
```

### `~/.bashrc` — append this block

```bash
# Override Fedora's /etc/profile.d/nano-default-editor.sh — this is what Yazi,
# lazygit, git, etc. shell out to. Host has only vi (vim-minimal), toolbox has vim.
if command -v vim >/dev/null 2>&1; then
    export EDITOR=vim
elif command -v vi >/dev/null 2>&1; then
    export EDITOR=vi
fi
export VISUAL="$EDITOR"

# Compact prompt — the 25%-wide side pane can't spare "user@host".
# Surgical edit rather than a fresh PS1, so the toolbox ⬢ marker and any
# shell-integration wrappers survive:  ⬢ [user@host dir]$  ->  ⬢ dir$
if [ -n "$PS1" ]; then
    PS1="${PS1//\\u@\\h /}"      # drop user@host
    PS1="${PS1//\[\\W\]/\\W}"    # drop the brackets around the dir
fi

# ~/.bashrc is shared host<->toolbox, but zellij only exists inside the box.
# So: in the toolbox, launch directly; on the host, hop in first (same $PWD, home is shared).
dev() {
    local dir="${PROJECT_DIR:-$PWD}"
    if [ -f /run/.containerenv ]; then
        cd "$dir" && zellij --layout dev
    else
        toolbox run --container "${DEV_TOOLBOX:-aip}" \
            bash -lc "cd '$dir' && zellij --layout dev"
    fi
}
# Optional: pin the site so `dev` always opens there
# export PROJECT_DIR=/path/to/site
# Optional: use a different toolbox
# export DEV_TOOLBOX=aip
```

### Tux watermark — source art + regeneration

`tux.png` is a rasterised artefact; this is the source it comes from. Save as
`tux.txt`, then run the two commands below.

```text
         .88888888:.
        88888888.88888.
      .8888888888888888.
      888888888888888888
      88' _`88'_  `88888
      88 88 88 88  88888
      88_88_::_88_:88888
      88:::,::,:::::8888
      88`:::::::::'`8888
     .88  `::::'    8:88.
    8888            `8:888.
  .8888'             `888888.
 .8888:..  .::.  ...:'8888888:.
.8888.'     :'     `'::`88:88888
.8888        '         `.888:8888.
888:8         .           888:88888
.888:88        .:           888:88888:
8888888.       ::           88:888888
`.::.888.      ::          .88888888
.::::::.888.    ::         :::`8888'.:.
::::::::::.888   '         .::::::::::::
::::::::::::.8    '      .:8::::::::::::.
.::::::::::::::.        .:888:::::::::::::
:::::::::::::::88:.__..:88888:::::::::::'
 `'.:::::::::::88888888888.88:::::::::'
       `':::_:' -- '' -'-' `':_::::'`
```

```bash
# 1. render the ASCII to a transparent PNG (any mono TTF works)
magick -background none -fill "#BD93F9" \
  -font ~/Documents/Dracula_Theme/fonts/jetbrains-mono/jetbrains-mono-variable.ttf \
  -pointsize 28 label:@tux.txt -trim +repage -resize x480 tux-art.png

# 2. composite onto a window-shaped canvas at the side pane's PERCENTAGE
#    (232x324 at +1621+626 on 1920x1080 == x 84.4-96.5%, y 58-88%)
magick -size 1920x1080 xc:none \
  \( tux-art.png -resize 232x324! \) -geometry +1621+626 -composite \
  ~/.config/ghostty/tux.png
```

Then set `background-image-fit = stretch` (§4b). Re-measure the offsets from a
screenshot if your `dev` pane sizes differ.

