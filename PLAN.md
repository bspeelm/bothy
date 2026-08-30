# PLAN.md — bothy: a turn-key, lightweight, atomic terminal IDE

> **bothy** *(n., Scottish)* — a small unlocked mountain shelter, free for anyone to use, kept by two customs: leave it as you found it, and leave fuel for the next visitor. This project is a bothy for developers: a minimal turn-key shelter built from tools that already stand. `bothy uninstall` honours the first custom (the machine is left exactly as found); "every bug fix ships a doctor check" honours the second (firewood for the next person). The launch verb is **`bothy`** itself. (This reverses an earlier decision that it stay `dev`; see `docs/decisions.md` ADR-008.) Name check (Aug 2026): no software project named bothy found; before publishing, verify `npm view bothy`, crates.io, and GitHub, then grab the org (`bothy-dev` or similar) and the `bothy.dev` domain.

## 0. Why this exists (read before writing any code)

AI IDEs over-bake execution: an Electron shell, a plugin marketplace, a background daemon, a telemetry pipeline, and 800 MB of RAM to show a text file. This project is the opposite bet: **a thin orchestrator over best-in-class terminal tools you already trust**. The IDE *is* the tools. bothy only installs them, writes their configs, and lays them out.

The origin is a working Fedora Silverblue + Toolbx setup (Ghostty → Zellij → Yazi → Claude Code) whose cheat sheet is included in this repo as `docs/origin-cheatsheet.md`. Every hard-won gotcha in that document becomes a `doctor` check. Treat it as the primary source of truth for the Linux path.

### Prior art — what exists, and why bothy is still worth building (surveyed Aug 2026)

Know the landscape before writing code; the README's "what bothy is not" section draws from this.

- **Yazelix** — the closest prior art: a reproducible Yazi + Zellij + Helix terminal IDE with layout orchestration and modular editor support. Its fork from bothy: **Nix/devenv is the only dependency**, it is editor-centric (file tree for Helix), no first-class agent slot, no Windows story, no doctor. bothy's differentiators against it: no Nix, pinned user-space binaries, agent-first cockpit, doctor, uninstall. Study its Zellij/Helix keybinding-conflict fixes — that work is excellent and worth learning from (not copying) for the `editor` profile.
- **Agent-manager apps** (wmux, MOLTamp, Pane, AgentsRoom, cmux) — parallel-agent cockpit GUIs with fleet views, built-in browsers, skin marketplaces. They solve *many agents at once* by shipping *a new app* — the exact weight-gain trajectory this project rejects. bothy does not compete on parallel-agent orchestration; one agent pane per profile is the v1 scope.
- **Claude Code Agent Teams / Agent View** — built on tmux; Zellij support is an open upstream feature request. Watch item: if it lands, `dev` should compose with it; do not build our own multi-agent layer in the meantime.
- **Omakub/Omarchy** — omakase at the OS tier (Omarchy even ships an editor+AI+terminal layout command). bothy is the same taste one level down: the terminal only, on the OS you already have.
- **Bivvy, berth, DevPod, devcontainers** — project *dependency/container* setup, orthogonal; bothy sets up the workspace around the project, not the project's deps. Compose, don't compete.
- **Hand-rolled setups** — this exact layout (editor | agent | lazygit/shell in Zellij) is widespread folk practice in blog posts and dotfiles. That is the demand signal: everyone builds it by hand; nobody ships it turn-key without Nix or an app.

**Positioning in one line:** the boring native version — Yazelix without Nix, plus an agent seat; the agent cockpit without the app.

### Non-negotiable principles

1. **Thin.** bothy is an installer + config writer + layout launcher + doctor. Nothing else. No daemon, no GUI, no plugin marketplace, no telemetry, no update-checker that phones home, no bundled copies of the tools.
2. **Atomic.** Default install is **user-space only**: pinned release binaries into `~/.local/bin`, configs into XDG paths. No root, no host modification, works on immutable distros. Optional `--system` mode uses the native package manager.
3. **Slots, not features.** Every component (terminal, multiplexer, browser, editor, agent, theme) is a *slot* with interchangeable *providers* described in small declarative files. Adding a provider must never require touching core code.
4. **Reversible.** `bothy uninstall` returns the machine to its prior state. Every generated file carries a header identifying it as bothy-managed; pre-existing user files are backed up, never clobbered.
5. **Every bug becomes a doctor check.** When a setup failure is fixed, the fix ships with a check that detects it. The doctor is the product's moat.
6. **Budgets are real.** Core binary ≤ 10 MB, core source ≤ ~5k LOC, install ≤ 60 s on a warm network, workspace idle RSS (all panes) ≤ 200 MB excluding the agent process. A change that breaks a budget needs a written justification in the PR.
7. **Simplicity beats cleverness.** If a feature needs an explanatory paragraph in the README, it is probably too clever. Prefer a documented manual step over an automated one that fails silently.

---

## 1. Architecture

```
bothy (single static binary, Go)
├── bootstrap   curl | sh  /  irm | iex   → downloads the right binary, runs `bothy install`
├── install     resolve slots → fetch pinned binaries → write configs → verify
├── doctor      run every check, print pass/fail with the fix for each failure
├── dev         launch the layout (in-container hop if needed); `dev attach`
├── config      show effective config; `config set slot.editor helix`; `config edit`
├── update      bump pinned versions from lockfile, re-run install + doctor
└── uninstall   remove binaries + generated configs, restore backups
```

### Language: Go (decided)

Single static binary, trivial cross-compilation to linux/darwin/windows × amd64/arm64, no runtime, fast startup, boring. Rust would also be fine (matches the Zellij/Yazi ecosystem) but Go's cross-compile story is simpler and this codebase is glue, not a systems tool. **Do not write the core in shell** — shell is unmaintainable across three OSes and two shells (bash/PowerShell). The only shell in the repo is the two ~30-line bootstrap scripts.

### Slot model

```
slots/
├── terminal/    ghostty.toml  wezterm.toml  kitty.toml  windows-terminal.toml  (detect + advise + drop theme; never installs on Linux host except via `--system`)
├── mux/         zellij.toml                                                     (only provider; tmux is a *documented non-goal* for v1)
├── browser/     yazi.toml  none.toml
├── editor/      vim.toml  nano.toml  helix.toml  neovim.toml
├── agent/       claude-code.toml  codex.toml  gemini-cli.toml  aider.toml  opencode.toml  custom.toml
├── theme/       dracula.toml  catppuccin.toml  none.toml
└── extras/      lazygit.toml  delta.toml  fzf.toml  ripgrep.toml  fd.toml  zoxide.toml  jq.toml
```

A provider file is pure data:

```toml
# slots/editor/helix.toml
name        = "helix"
binary      = "hx"
version_cmd = "hx --version"
min_version = "25.01"

[install]
release      = "helix-editor/helix"        # GitHub releases; asset pattern per platform below
asset.linux  = "helix-{version}-x86_64-linux.tar.xz"
asset.darwin = "helix-{version}-aarch64-macos.tar.xz"
asset.wsl    = "linux"
pkg.brew     = "helix"
pkg.dnf      = "helix"
pkg.apt      = "helix"          # note: may need PPA; doctor warns if version < min
pkg.pacman   = "helix"

[config]
files = ["config.toml"]          # templates in templates/editor/helix/
dest  = "~/.config/helix"

[env]
EDITOR = "hx"

[[checks]]                        # doctor checks specific to this provider
id   = "helix-runtime-dir"
cmd  = "hx --health"
pass = "exit 0"
fix  = "Missing runtime dir; re-run `bothy install` or set HELIX_RUNTIME."
```

Core code knows how to: resolve a provider, fetch a release asset (with checksum), install to `~/.local/bin`, render templates, run checks. It knows nothing about vim or helix specifically.

### Layout profiles

Profiles are small TOML rendered to Zellij KDL at launch (KDL is generated, never hand-edited by users). Three presets ship; users can add their own in `~/.config/bothy/profiles/`.

| Profile | Layout | Who it's for |
|---|---|---|
| `cockpit` (default) | browser 100%w × 50%h on top; agent 60% + shell 40% below | supervising an agent on a repo |
| `editor` | editor \| agent \| shell, three columns | the origin 3-pane vim setup |
| `minimal` | agent + shell | small screens, SSH |

Rules learned the hard way (encode in the renderer, cover with tests):
- Zellij `split_direction="vertical"` = **columns**. The renderer must own this so users never see it.
- `{ plugin location="…" }` on one line needs a trailing `;` — always emit multi-line.
- `pane size=1` tab-bar and `size=2` status-bar are fixed line counts; percentages apply to the rest.
- Layout changes apply at launch only; `dev` must print a one-line hint when the profile file is newer than the running session.
- If the agent slot is already running in the current shell (e.g. `CLAUDECODE` env set), refuse to launch a nested one; print why.

### Config strategy

- Generated configs go to the tool's real path (`~/.config/yazi/yazi.toml`, etc.) because the tools require it.
- Every generated file starts with `# managed by bothy — edit ~/.config/bothy/overrides/<tool>/ instead; regenerate with bothy install`.
- If the destination exists and lacks the header → back up to `~/.local/state/bothy/backup/<timestamp>/` and record in `manifest.json` (used by `uninstall`).
- Overrides: `~/.config/bothy/overrides/<tool>/<file>` is appended/merged after the template (append for TOML/KDL/INI lists; document the merge rule per format, keep it dumb).
- Templates are Go `text/template` with a tiny function set (`theme.color "purple"`, `slot "editor"`, `os`). No conditionals sprawl — if a template needs more than five `{{if}}`s, split providers instead.

### Lockfile

`bothy.lock` pins every provider's version + asset checksum. `bothy update` refreshes from GitHub releases API, writes the lock, and re-runs install + doctor. CI regenerates the lock weekly in a PR so bumps are reviewed, not automatic.

---

## 2. Platform matrix

| | Terminal | Mux | Notes |
|---|---|---|---|
| **Linux** (Fedora/Ubuntu/Arch, incl. Silverblue + Toolbx/Distrobox) | Ghostty (advise: rpm-ostree/apt/pacman/brew per distro), fallback: whatever `$TERM` is | Zellij | The known-good path. Port the cheat sheet first. |
| **macOS** | Ghostty via brew cask | Zellij | Straightforward; watch `~/.config` vs `~/Library` — all these tools use XDG on mac, verify per tool. |
| **Windows** | Windows Terminal (winget) or Ghostty **if a Windows build exists at implementation time — verify, do not assume** | Zellij **inside WSL2** | **Native Windows without WSL is a documented non-goal.** Zellij and Yazi's ecosystem are Unix-native. `install.ps1` installs Windows Terminal + WSL2 (Fedora or Ubuntu), drops a WT profile that opens straight into `wsl -d <distro> -- bothy dev`, then hands off to the Linux installer inside WSL. Doctor checks run from inside WSL. |

### Container awareness (Linux)

Detect `/run/.containerenv` (Toolbx/Distrobox) and devcontainers. When inside a container:
- `dev` runs directly; on the host, `dev` hops in via `toolbox run` / `distrobox enter` (configurable `bothy.container`).
- Copy the host's `xterm-ghostty` terminfo into `~/.terminfo` (shared home).
- Install a guarded `~/.local/bin/xdg-open` shim that does `flatpak-spawn --host xdg-open "$@"` only when inside a container (the guard prevents host recursion via the shared home).
- Doctor: warn if `~/.bashrc` is shared and an unguarded `zellij` alias exists.

---

## 3. Doctor — initial check list

Port every gotcha from the cheat sheet, generalised. Each check has `id`, `severity` (fail/warn), `platforms`, a detection command, and a **one-line fix**. Initial set:

**Terminal**
- `$TERM` known; Ghostty terminfo present in the current environment (esp. containers)
- Ghostty reads `~/.config/ghostty/config` — warn if a `config.ghostty` or other near-miss filename exists and is being ignored
- `ghostty +validate-config` passes (host only)

**Mux / layout**
- `zellij setup --check` passes
- Generated KDL parses; resolved layout in `~/.cache/zellij/*/session_info/*/session-layout.kdl` matches the profile's pane count (proves the layout actually built)
- Zellij ≥ min version; Kitty-graphics passthrough known-unsupported → assert Yazi image previews are routed to the placeholder previewer (see below)

**Browser (Yazi)**
- **Config-silently-discarded check:** `yazi 2>&1 | grep -iE "must be|invalid|preset"` must be **silent** — any output means Yazi threw away the whole config
- Yazi ≥ 26.x if any `yazi-rs/plugins` are installed (older Yazi refuses every current plugin)
- `[mgr]` not `[manager]`; `url` not `name` in filetype rules and fetchers (26.x renames)
- Image previews inside Zellij route to the `enter-hint` placeholder previewer (chafa block-art + phantom "Find next" keypress otherwise)
- Opener works: `xdg-open` exists *and* has something to hand off to; inside containers, the host shim is in place

**Editor**
- `EDITOR`/`VISUAL` set to the chosen editor (Fedora's `nano-default-editor` profile.d overrides otherwise — warn)
- vim: colorscheme actually loads — `vim -es -u ~/.vimrc -c "call writefile([get(g:,'colors_name','NONE')],'/tmp/cs')" -c q` (the `-u` is required or the test tests nothing); warn if colorschemes are in `pack/*/start` (runtimepath is extended *after* .vimrc)
- helix: `hx --health` clean; neovim: `nvim --headless +checkhealth`-derived quick check

**Agent**
- Chosen agent binary on PATH, version ≥ min, auth present (agent-specific check, e.g. config file exists) — never store or inspect secrets
- Not launching a nested agent session

**Packaging**
- Every binary in `~/.local/bin` matches the lockfile checksum
- `--system` mode on Fedora: known-bad COPRs absent (e.g. `pgdev/ghostty` blocks rpm-ostree upgrades; `varlad/yazi` pinned at 25.x) — keep this list in `slots/…/known-bad.toml`, not in code
- Shadowing: no second `rg`/`fd`/`fzf`/`jq` earlier on PATH with an older version

**Output format:** `✓ / ✗ / !` per check, fix line under failures, non-zero exit if any `fail`. `--json` for CI.

---

## 4. Phases (do them in order; each phase ends green in CI)

### Phase 0 — Scaffold & decisions (½ day)
- Repo layout: `cmd/bothy`, `internal/{slots,install,config,layout,doctor,platform}`, `slots/`, `templates/`, `profiles/`, `bootstrap/{install.sh,install.ps1}`, `docs/`, `test/`.
- `go.mod`, Makefile/`just` with `build`, `test`, `lint`, `release` (goreleaser, cross-compile matrix).
- CI: lint + unit tests on every PR; integration matrix (see Phase 4) nightly.
- MIT license. Write `docs/decisions.md` (ADR-style, one paragraph each): Go, user-space default, Zellij-only mux, WSL-only Windows, no plugin system.
- Copy the cheat sheet to `docs/origin-cheatsheet.md`.

### Phase 1 — Linux, the known-good path (core of the work)
1. Provider loader + lockfile + release fetcher (GitHub releases, checksum, tar/zip extract, arm64+amd64).
2. Template renderer + backup/manifest + overrides.
3. Providers: zellij, yazi (+ `enter-hint` plugin, smart-enter, chmod, git, full-border), vim, nano, helix, claude-code, dracula theme, extras.
4. Layout renderer + three profiles; `dev` launcher incl. container hop.
5. Doctor with the full §3 list.
6. Manual acceptance on: Fedora Silverblue + Toolbx (the origin), Fedora Workstation, Ubuntu 24.04, Arch. Ghostty install is *advised* per distro, not performed, unless `--system`.

### Phase 2 — macOS
- brew cask for Ghostty; verify XDG paths for every tool; Apple Silicon assets; `dev` from Ghostty and from Terminal.app (no image previews there — doctor warns, doesn't fail).

### Phase 3 — Windows via WSL2
- `install.ps1`: winget Windows Terminal, enable WSL2, install Fedora or Ubuntu, WT profile → `wsl -d … -- bothy dev`, hand off to `install.sh` inside WSL.
- Doctor: WT profile present, `wsl --status` OK, clipboard integration (`clip.exe`/`wl-clipboard` shim) for the editor's system clipboard.
- Document clearly: native Windows without WSL is not supported and won't be.

### Phase 4 — Hardening & CI matrix
- Integration tests: run `install` + `doctor --json` in Fedora, Ubuntu, Arch containers (Podman in CI), on a macOS runner, and — as far as GitHub Actions allows — a Windows runner with WSL. Where CI can't run something (Ghostty GUI, WSL kernel), run the doctor in *simulation mode* against fixture directories.
- Golden-file tests for every rendered config and layout across all provider combinations (small cartesian product — keep it small by design).
- `uninstall` round-trip test: install → uninstall → filesystem diff is empty.

### Phase 5 — Docs & release
- README: one screenshot, one command per OS, the three profiles, the slot table, "what bothy is not".
- `docs/adding-a-provider.md` (should fit on one screen — if it doesn't, the slot model is too complex).
- v0.1.0 via goreleaser; bootstrap scripts pinned to the release.

---

## 5. Agent & editor slot details

**Agents** ship as providers with only: binary name, install method (npm/curl/brew — verify current official install commands at implementation time, they change), version check, launch command, nested-session env guard. The layout's "main" pane just runs `{{ slot "agent" | launchcmd }}`. `custom.toml` lets a user point at any command. **Do not build agent-specific integrations** (no hooks, no MCP wiring, no prompt templates) — that's the agent's job.

**Editors**: vim (origin config, ported; plugins via native `pack/`, no manager), nano (minimal `.nanorc` with syntax + line numbers), helix (zero-plugin, single binary — the best fit for the project's philosophy; make it the *recommended* "something else"), neovim (kickstart-style minimal init, no distro). The `editor` profile puts the editor in the main pane; `cockpit` uses the editor only as `EDITOR` for Yazi/git/agent hand-offs.

---

## 6. Theming

- Ship **Dracula (MIT, the open theme)** as the default and Catppuccin as the second provider. That's it — no paid-theme support paths, no theme importers.
- A theme provider = one palette TOML + per-tool template. No theme engine. The origin cheat sheet's configs were themed with a licensed pack — when porting them, swap to the standard open Dracula palette (`#282A36` bg, `#BD93F9` purple, `#FF79C6` pink, `#8BE9FD` cyan, `#50FA7B` green, `#FFB86C` orange, `#FF5555` red, `#F1FA8C` yellow, `#6272A4` comment, `#44475A` selection, `#F8F8F2` fg) rather than copying the PRO values in.
- The Ghostty background-image watermark trick from the cheat sheet is an *optional* extra (`extras/watermark.toml`), off by default, with the percentage-composite ImageMagick recipe and the `fit = stretch` rationale preserved in its doc.

---

## 7. Explicit non-goals (keep this list in the README)

- Native Windows without WSL
- tmux support (v2 maybe; would double the layout renderer)
- Plugin marketplace, extension API, or runtime plugins of any kind
- Bundling or vendoring the tools themselves
- LSP/debugger management (that's the editor's domain — helix/neovim handle it)
- Background services, auto-update daemons, telemetry, accounts
- GUI of any kind
- Managing the agent's config, keys, MCP servers, or hooks

---

## 8. Working agreement for Claude Code

- Work phase by phase; open a PR per phase, green CI before the next.
- Before adding any Go dependency, ask. Stdlib + `pelletier/go-toml` + `spf13/cobra` (or a hand-rolled subcommand switch — prefer that if it stays under ~150 lines) is the expected ceiling.
- Every bug fix ships with a doctor check and a test.
- Every new provider is a TOML file + templates only; if it needs Go changes, stop and explain why in the PR.
- Keep the budgets in §0 in CI as assertions (binary size, LOC count via a script, install time on the Fedora container).
- Conventional commits. Update `docs/decisions.md` whenever a principle is bent.
- When unsure whether something belongs in bothy or in the underlying tool, it belongs in the underlying tool.
