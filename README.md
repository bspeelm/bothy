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

One command opens a persistent layout with a file browser, an agent, and a
shell:

```
┌──────────────────────────────────────────────────┐  tab bar
│         browser — file tree + preview            │
├─────────────────────────────┬────────────────────┤
│      agent (focused)        │   shell            │
├─────────────────────────────┴────────────────────┤  status bar
```

```sh
cd ~/some-project
bothy
```

bothy supplies any tool you are missing, writes its own configs, lays out the
panes, and then tells you what is wrong and how to fix it. That last part is
the point.

It does not touch your dotfiles. Everything it writes lives in one directory,
and `bothy uninstall` removes it.

## Why

AI IDEs over-bake execution: an Electron shell, a plugin marketplace, a
background daemon, and 800 MB of RAM to show a text file. bothy is the opposite
bet — **a thin orchestrator over best-in-class terminal tools**. The IDE *is* the
tools. bothy arranges them, configures them in its own tree, and gets out of
the way.

It grew out of one working setup on Fedora Silverblue, whose cheat sheet is
included here as [`docs/origin-cheatsheet.md`](docs/origin-cheatsheet.md). Every
trap in that document is dealt with, because every one of them fails *silently*
— and a tool that fails silently is worse than one that crashes.

Some are **detected**, because they can still happen to you:

| What you see | What actually happened |
|---|---|
| Your Yazi settings "didn't take" | Yazi rejected one key and discarded **the entire config** |
| Previews are blocky, and a phantom "Find next" fires | The multiplexer could not pass the graphics protocol, and its mangled reply was parsed as keystrokes |
| A file browser feature just isn't there | A plugin the config references was never installed |
| Your workspace opens with the wrong panes | The multiplexer read the layout differently than it was written |

Others are **avoided**, because bothy no longer does the thing that caused them:

| The original trap | Why it cannot happen here |
|---|---|
| vim ignores a colorscheme in `pack/*/start` | bothy does not write your `.vimrc` at all |
| Ghostty silently ignores `config.ghostty` | bothy hands Ghostty its own file with `--config-file` |
| A host-forwarding `xdg-open` recurses into itself | The shim lives in bothy's bin, on `PATH` for bothy's session only |

`bothy doctor` prints a one-line fix under every failure. The list grew while
this was being built: bothy's own Yazi config required plugins it never
installed, and nothing noticed, because `yazi --clear-cache` — what the config
check runs — does not execute `init.lua`.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh
bothy
```

Two commands, and the second is the one you keep using. The script puts a
single binary on your `PATH`; the first `bothy` sets the workspace up —
installing only the tools you are missing, after telling you which and asking —
and then launches it.

`bothy install` exists too, and you want it after changing a setting with
`bothy config set`. You do not need it to start.

### On Fedora

```sh
sudo dnf copr enable bspeelman/bothy
sudo dnf install bothy
bothy
```

Built from source in Copr for Fedora 43, 44 and Rawhide, x86_64 and aarch64.
On Silverblue and other image-based hosts this needs `rpm-ostree install` and a
reboot, so the one-liner above is usually the better route there.

### With Go

```sh
go install github.com/bspeelm/bothy/cmd/bothy@latest
bothy
```

Resolves to the latest tag. No release download, but you need Go.

### From source

For contributors. Needs Go and `make`, which a fresh container or a clean
machine will not have — the one-liner above is the path for everyone else.

```sh
git clone https://github.com/bspeelm/bothy && cd bothy
make install-binary     # builds ./bothy, copies it to ~/.local/bin/bothy
bothy
```

`make install-binary` installs the *binary*. `bothy install` sets up the
*workspace*. Different things; the target is named for which.

### What it installs

On a machine with none of them: nine tools, about 131 MB, all inside
`~/.local/share/bothy/` and each verified against `bothy.lock` before a byte
is written to disk.

| | | | |
|---|---|---|---|
| zellij 52.5 MB | yazi 24.1 MB | lazygit 19.5 MB | delta 7.2 MB |
| ripgrep 5.4 MB | fzf 4.9 MB | fd 3.7 MB | jq 2.3 MB |
| zoxide 1.2 MB | | | |

Yazi's archive also carries `ya`, its own package manager (9.3 MB), which is
what installs the four Yazi plugins bothy's config depends on. Inside a
container a small `xdg-open` shim is added too, since there is no desktop
there to hand a file to.

On a machine that already has current versions, bothy downloads nothing — it
uses what you have and says which. Your package manager is never invoked,
nothing needs root, and nothing is added to your `PATH`.

### Two things it will not install

**A terminal that can draw images.** Ghostty publishes no release binaries, so
there is nothing to fetch, and every install path runs a package manager as
root — on an image-based host, with a reboot. bothy could start that and not
finish it, which is worse than telling you the command. Without one, bothy
still works; Yazi's image previews fall back to block art, and the doctor says
so rather than leaving you to wonder.

**The agent.** Install methods change, auth is not bothy's business, and a
workspace tool that quietly installs an AI agent is doing something you did not
ask for. Without one the main pane opens empty, and the doctor tells you.
`bothy config set slots.agent none` if you would rather not have that pane.

For both, `bothy doctor` prints the command for the machine you are actually
on — the `rpm-ostree` one with its reboot on Silverblue, the `dnf` one on
Fedora Workstation — along with the repositories to stay away from, including
the one that blocks system upgrades until it is removed.

## Commands

| Command | Does |
|---|---|
| `bothy install` | Write every config, then run the doctor |
| `bothy doctor` | Report what is broken and how to fix it (`--json` for CI) |
| `bothy` | Launch the workspace |
| `bothy attach` | Reattach to a running session |
| `bothy desktop-entry` | Print a `.desktop` launcher (`--install` to write it) |
| `bothy config set <key> <value>` | Change a slot, theme, or workspace setting |
| `bothy layout` | Print the layout that would be launched |
| `bothy theme example` | Print a blank palette file to fill in |
| `bothy tools` | Show which tools are used and where they came from |
| `bothy uninstall` | Remove bothy's directory |

## Slots

Every component is a slot with interchangeable providers:

| Slot | Default | Also |
|---|---|---|
| terminal | ghostty | *advised, never installed — it lives on your host* |
| mux | zellij | — |
| browser | yazi | none |
| editor | vim | nano, helix |
| agent | claude-code | any command |
| theme | dracula (open) | any palette, via a file of your own |

```sh
bothy config set slots.editor helix
bothy config set slots.agent none
bothy install
```

## Where it runs

If you are already in a terminal that can draw images — Ghostty, Kitty, WezTerm
— `bothy` runs there. If you are not, or you launched it from a desktop icon,
it opens a Ghostty window with its own config and runs there instead.

That is not a preference about terminals. Inline image previews need a terminal
that speaks the Kitty graphics protocol; run bothy inside GNOME Terminal and
Yazi quietly degrades to block art. Opening a window that works beats
pretending the one you have is fine.

`--in-place` and `--window` force either behaviour. With no graphical display —
SSH, a bare TTY — it always runs in place, because a window that cannot open is
worse than a workspace without image previews.

`bothy desktop-entry --install` writes a launcher so bothy appears in your
application menu. It is the one file bothy writes outside its own tree, which
is why it is a separate command that tells you where it went, and why
`bothy desktop-entry --remove` exists.

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

## Theming

bothy ships the **open Dracula** palette (MIT) and themes every tool from the
same eleven colour tokens — the multiplexer, the file browser, the terminal, and
a vim colorscheme generated from those tokens. Add a palette and everything is
themed at once.

That one palette is the only set of colour values in this repository. Every
other palette is a file on your machine:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml     # eleven colours
bothy config set theme.palette ~/.config/bothy/my-palette.toml
bothy install
```

This is also how you use a theme you have **licensed**. Copy its values into
your own file; bothy neither ships nor parses any commercial theme's files, so
what you have paid for stays on your machine and nothing is redistributed. A
test fails the build if a colour that is not open Dracula's appears anywhere in
this repository.

Already have a colorscheme installed for your editor? Name it and bothy will
reference it instead of generating one:

```sh
bothy config set theme.vim_colorscheme my_scheme
```

## What bothy touches

Two directories. That is the whole footprint.

```
~/.local/share/bothy/     bothy's own tree — configs, and any tool it had to supply
~/.config/bothy/          yours — config.toml, palette, overrides/  (worth putting in git)
```

**Your dotfiles are neither read nor written.** Not `~/.config/yazi`, not
`~/.vimrc`, not `~/.bashrc`, not your global git config. bothy renders its
configs into its own tree and launches each tool pointed there —
`ZELLIJ_CONFIG_DIR`, `YAZI_CONFIG_HOME`, and a Ghostty config handed over with
`--config-file`. Those variables are set for bothy's process tree only, so your
shell keeps its own `PATH` and `EDITOR`.

`bothy uninstall` removes one directory. It is exact by construction rather than
by careful bookkeeping, and a test asserts that installing writes nothing
outside that tree in any configuration.

It also means **your setup is portable in one folder**: put `~/.config/bothy/`
in git, clone it on a new machine, run `bothy install`.

### Using your own config instead

If you have a Yazi setup you like, keep it:

```toml
# ~/.config/bothy/config.toml
passthrough = ["yazi"]
```

That points `YAZI_CONFIG_HOME` at your directory rather than bothy's. The doctor
reports which slots are passed through and what it turns off — bothy's
image-preview handling lives in its `yazi.toml`, so passing Yazi through means
that does not apply.

To adjust bothy's config rather than replace it, drop a file in
`~/.config/bothy/overrides/<tool>/<file>`. It is appended after the template, so
your setting wins.

### Tools

bothy fills gaps rather than duplicating. A tool already on your `PATH` that
meets the minimum version is used as-is; a missing or too-old one is fetched
into bothy's own `bin/`, which is on `PATH` for bothy's session only. Installing
a newer zellij never changes what `zellij` means in your shell, and no package
manager is ever invoked.

Minimums are "the oldest that actually works", not "the newest available", so
on a normally equipped machine bothy downloads nothing. `bothy tools` shows the
decision without changing anything:

```
$ bothy tools
✓ delta     /usr/bin/delta is 0.19.1
✓ yazi      /usr/bin/yazi is 26.5.6
↓ zellij    /usr/bin/zellij is 0.42.2, below the minimum 0.45.1 —
            image previews need the Kitty graphics protocol, added in 0.45
```

Every version and checksum is pinned in `bothy.lock`; a download whose sha256
does not match is refused and nothing is written. `bothy install --offline`
skips fetching entirely and uses only what you have.

## Containers and immutable distros

The install is user-space only: no root, nothing layered onto the host image, so
it works unchanged on Silverblue, Kinoite and Bazzite.

Inside Toolbx or Distrobox, bothy detects the container by name, so `bothy` run
on the host hops into the right one — no container name hardcoded in a shell
config. It also supplies a *guarded* `xdg-open` that forwards to the host, since
a container has no desktop to open a file with. That shim lives in bothy's own
`bin/`, on `PATH` only for bothy's session — which is why it cannot be picked up
by the host and made to execute itself forever through your shared home.

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

## Credits and licence

bothy is MIT. See [`LICENSE`](LICENSE).

The built-in palette is [Dracula](https://github.com/dracula/dracula-theme)
(MIT) — thank you. It is the only palette in this repository; everything else
bothy uses belongs to someone else and stays where it is. bothy vendors no
tool: it downloads official release binaries and verifies them against
`bothy.lock`, and each tool remains under its own licence. [`NOTICE`](NOTICE)
has the full list.
