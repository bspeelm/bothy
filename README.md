<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/bothy-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/images/bothy-light.png">
    <img alt="bothy — a stone shelter in a landscape, drawn in ASCII" src="docs/images/bothy-light.png" width="820">
  </picture>
</p>

<h1 align="center">bothy</h1>

<p align="center">
  <em>a turn-key terminal workspace built from tools you already trust</em>
</p>

> **bothy** *(n., Scottish)* — a small unlocked mountain shelter, free for anyone
> to use, kept by two customs: leave it as you found it, and leave fuel for the
> next visitor.

One command opens a persistent layout with a file browser, an agent and a shell:

```sh
cd ~/some-project
bothy
```

<p align="center">
  <img alt="the bothy workspace: a Yazi file browser across the top with a file preview, an agent pane and a shell below, inside Zellij" src="docs/images/workspace.png" width="900">
</p>

bothy installs any tool you are missing, writes its own configs, arranges the
panes, and then tells you what is broken and how to fix it. It does not touch
your dotfiles: everything it manages lives in one directory, and
`bothy uninstall` removes it.

## Install

Pick one. Same program and same version either way.

| | for | |
|---|---|---|
| **Script** | anyone on Linux or macOS | `curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh \| sh` |
| **dnf** | Fedora Workstation | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy` |
| **Go** | you already have Go | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| **Source** | contributors | `git clone` then `make install-binary` |

Then, from any directory:

```sh
bothy
```

The first run sets the workspace up — installing only the tools you are
missing, after telling you which and asking — and launches it.

> On Silverblue and other image-based hosts, `dnf` means `rpm-ostree install`
> and a reboot. The script has neither cost.

### What arrives

Nine tools, about 131 MB, on a machine that has none of them — each verified
against `bothy.lock` before a byte is written:

| | | | |
|---|---|---|---|
| zellij | yazi | lazygit | delta |
| ripgrep | fzf | fd | jq |
| zoxide | | | |

On a machine that already has current versions, bothy downloads nothing and
says which it is using. Your package manager is never invoked, nothing needs
root, and nothing is added to your `PATH`.

### What does not

**A terminal that can draw images.** Ghostty publishes no binaries and every
install path needs root. Without one, previews fall back to block art.

**The agent.** Without one the main pane opens empty; `bothy config set
slots.agent none` turns it off.

For both, `bothy doctor` prints the command for the machine you are on.

## Commands

| | |
|---|---|
| `bothy` | Launch the workspace |
| `bothy attach` | Reattach to a running session |
| `bothy doctor` | What is broken, and the one-line fix (`--json` for CI) |
| `bothy install` | Re-apply after changing a setting |
| `bothy tools` | Which tools are used, and where they came from |
| `bothy config set <key> <value>` | Change a slot, theme or workspace setting |
| `bothy layout` | Print the layout that would be launched |
| `bothy theme example` | Print a blank palette file |
| `bothy desktop-entry` | Print a `.desktop` launcher (`--install` writes it) |
| `bothy uninstall` | Remove bothy's directory and its binary |

## What it touches

```
~/.local/share/bothy/     bothy's tree — configs, and any tool it supplied
~/.config/bothy/          yours — config.toml, palette, overrides   (git this)
```

Nothing else. Not `~/.config/yazi`, not `~/.vimrc`, not `~/.bashrc`, not your
global git config. bothy renders its configs into its own tree and launches
each tool pointed there, with environment variables scoped to that session, so
your shell keeps its own `PATH` and `EDITOR`.

That second directory is your setup: put it in git, clone it on a new machine,
run `bothy`.

### Using your own config instead

```toml
# ~/.config/bothy/config.toml
passthrough = ["yazi"]
```

That points the tool at your directory rather than bothy's. To adjust bothy's
config instead of replacing it, drop a file in
`~/.config/bothy/overrides/<tool>/<file>` — it is appended after the template,
so your setting wins.

## Slots and profiles

Every component is a slot with interchangeable providers:

| slot | default | also |
|---|---|---|
| terminal | ghostty | advised, never installed |
| mux | zellij | — |
| browser | yazi | none |
| editor | vim | nano, helix |
| agent | claude-code | any command |
| theme | dracula | any palette, via a file of your own |

```sh
bothy config set slots.editor helix
```

Three layouts ship. `cockpit` is the default — browser on top, agent and shell
below. `editor` puts an editor, agent and shell in three columns. `minimal` is
an agent and a shell, for small screens and SSH. Profiles are small TOML files;
drop your own in `~/.config/bothy/profiles/`.

## Theming

bothy ships the open [Dracula](https://github.com/dracula/dracula-theme)
palette and themes every tool from the same eleven colour tokens — including a
generated vim colorscheme. Any other palette is a file on your machine:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how to use a palette you have licensed. bothy ships no colour
values but its own, so what you paid for stays on your machine.

## Where it runs

In a terminal that can draw inline images — Ghostty, Kitty, WezTerm — bothy
runs there. Otherwise it opens a Ghostty window with its own config, because
image previews need a terminal that can draw them and degrading quietly is
worse. `--in-place` and `--window` force either behaviour; with no display it
always runs in place.

Inside Toolbx or Distrobox, bothy records which container it installed its
tools in and hops back there when launched from the host. Install from a
container that has none of the tools and the result is self-contained, working
from anywhere.

## What bothy is not

- A native Windows tool without WSL
- A tmux setup
- A plugin marketplace or extension API
- A vendor of the tools — it installs upstream releases and verifies them
- An LSP or debugger manager
- A background service, auto-updater, or telemetry pipeline
- A manager of your agent's config, keys or hooks
- A Flatpak. Flathub excludes console software, and bothy fetches tools at
  runtime, which the manifest model exists to prevent

## Contributing

[`docs/adding-a-provider.md`](docs/adding-a-provider.md) — a provider is a data
file and templates, not Go code. Design rationale is in
[`docs/decisions.md`](docs/decisions.md); the working agreement is in
[`PLAN.md`](PLAN.md).

## Credits and licence

MIT — see [`LICENSE`](LICENSE).

The built-in palette is [Dracula](https://github.com/dracula/dracula-theme)
(MIT). bothy vendors no tool: it downloads official release binaries, verifies
them against `bothy.lock`, and each stays under its own licence.
[`NOTICE`](NOTICE) has the full list.
