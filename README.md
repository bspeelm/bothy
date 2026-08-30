# bothy

> **bothy** *(n., Scottish)* — a small unlocked mountain shelter, free for anyone
> to use, kept by two customs: leave it as you found it, and leave fuel for the
> next visitor.

A turn-key terminal workspace built from tools you already trust. One command
opens a persistent layout with a file browser, an agent, and a shell:

```
┌──────────────────────────────────────────────────┐  tab bar
│         browser — file tree + preview            │
├─────────────────────────────┬────────────────────┤
│      agent (focused)        │   shell            │
├─────────────────────────────┴────────────────────┤  status bar
```

```sh
cd ~/some-project
dev
```

bothy installs the tools, writes their configs, lays out the panes, and then
tells you what is wrong and how to fix it. That last part is the point.

## Why

AI IDEs over-bake execution: an Electron shell, a plugin marketplace, a
background daemon, and 800 MB of RAM to show a text file. bothy is the opposite
bet — **a thin orchestrator over best-in-class terminal tools**. The IDE *is* the
tools. bothy only installs them, configures them, and arranges them.

It grew out of one working setup on Fedora Silverblue, whose cheat sheet is
included here as [`docs/origin-cheatsheet.md`](docs/origin-cheatsheet.md). Every
trap in that document is now a `doctor` check, because every one of them fails
*silently*:

| What you see | What actually happened |
|---|---|
| Your Yazi settings "didn't take" | Yazi rejected one key and discarded **the entire config** |
| vim comes up in default colours | The colorscheme is on `runtimepath` only *after* `.vimrc` is sourced |
| Your terminal config does nothing | Ghostty reads `config`, and yours is named `config.ghostty` |
| A phantom "Find next" on every image | The multiplexer mangled a capability reply; the bytes became keystrokes |

`bothy doctor` finds all four, and prints the one-line fix for each.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/bothy-dev/bothy/main/bootstrap/install.sh | sh
bothy install
```

Or from source:

```sh
git clone https://github.com/bothy-dev/bothy && cd bothy
make install-local
bothy install
```

Then open a new shell and run `dev`.

## Commands

| Command | Does |
|---|---|
| `bothy install` | Write every config, then run the doctor |
| `bothy doctor` | Report what is broken and how to fix it (`--json` for CI) |
| `dev` | Launch the workspace (`dev attach` to reattach) |
| `bothy config set <key> <value>` | Change a slot, theme, or workspace setting |
| `bothy layout` | Print the layout that would be launched |
| `bothy uninstall` | Put the machine back the way it was |

## Slots

Every component is a slot with interchangeable providers:

| Slot | Default | Also |
|---|---|---|
| terminal | ghostty | *advised, never installed — it lives on your host* |
| mux | zellij | — |
| browser | yazi | none |
| editor | vim | nano, helix |
| agent | claude-code | any command |
| theme | dracula (open) | dracula pro, via your own pack |

```sh
bothy config set slots.editor helix
bothy config set slots.agent none
bothy install
```

## Profiles

| Profile | Layout | For |
|---|---|---|
| `cockpit` *(default)* | browser on top; agent + shell below | supervising an agent on a repo |
| `editor` | editor \| agent \| shell | driving the editor yourself |
| `minimal` | agent + shell | small screens and SSH |

Profiles are small TOML files describing rows and columns. bothy renders them to
Zellij's KDL, which means you never meet Zellij's two layout traps:
`split_direction="vertical"` produces *columns*, and a one-line `plugin` node
needs a trailing semicolon. Drop your own in `~/.config/bothy/profiles/`.

## Theming, and Dracula PRO

bothy ships the **open Dracula** palette (MIT) and themes every tool from the
same eleven colour tokens — including generating a vim colorscheme, so adding a
palette themes everything at once.

[Dracula PRO](https://draculatheme.com/pro) is a paid pack, so **bothy ships none
of it**. Point bothy at the copy you bought and it reads the palette out of that:

```sh
bothy config set theme.variant pro          # or blade, buffy, lincoln,
bothy config set theme.pro_pack ~/Documents/Dracula_Theme   # morbius, van-helsing, alucard
bothy install
```

bothy parses the pack's `design/palette.md` for its colours and copies the pack's
own Ghostty theme and vim colorschemes verbatim. Nothing paid is redistributed,
and a test in this repository fails the build if a PRO colour ever appears in it.

## Nothing is written without a backup

- Every generated file starts with `# managed by bothy` and names where to put
  your own changes (`~/.config/bothy/overrides/<tool>/`, appended after the
  template so it wins).
- A file that was already there is copied to `~/.local/state/bothy/backup/`
  and recorded before it is replaced.
- A managed file you have since edited by hand is **left alone** and reported.
- `bothy uninstall` works only from that record. A file bothy did not write down
  is not bothy's to delete.

## Containers and immutable distros

The default install is user-space only: binaries into `~/.local/bin`, configs
into XDG paths. No root, nothing layered onto the host image.

Inside Toolbx or Distrobox, bothy detects the container by name, so `dev` run on
the host hops into the right one — no hardcoded container name in your shell
config. It also installs a *guarded* `xdg-open` shim that forwards to the host,
guarded because your home directory is shared and an unguarded one makes the
host exec itself forever.

## What bothy is not

- A native Windows tool without WSL (documented non-goal)
- A tmux setup (maybe v2; it would double the layout renderer)
- A plugin marketplace, extension API, or anything with a runtime plugin
- A vendor of the tools themselves — it installs upstream releases
- An LSP or debugger manager (that is your editor's job)
- A background service, auto-updater, telemetry pipeline, or account
- A manager of your agent's config, keys, MCP servers, or hooks

## Prior art worth knowing

**[Yazelix](https://github.com/luccahuguet/yazelix)** is the closest thing —
Yazi + Zellij + Helix, reproducible via Nix. bothy differs by needing no Nix,
putting an agent in the main seat, and shipping a doctor and an uninstall.
**Omakub/Omarchy** do this one level up, at the OS. Parallel-agent cockpit apps
solve "many agents at once" by shipping a new app — the weight-gain trajectory
this project exists to avoid.

## Contributing

See [`docs/adding-a-provider.md`](docs/adding-a-provider.md) — a provider should
be a data file and templates, never new Go code. Design rationale lives in
[`docs/decisions.md`](docs/decisions.md), and the working agreement in
[`PLAN.md`](PLAN.md). Every bug fix ships with a doctor check and a test.

## Licence

MIT.
