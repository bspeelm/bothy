<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/bothy-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/images/bothy-light.png">
    <img alt="bothy — a stone shelter in a landscape, drawn in ASCII" src="docs/images/bothy-light.png" width="820">
  </picture>
</p>

<h1 align="center">bothy</h1>

<p align="center">
  <em>a terminal cockpit for working with an AI coding agent — that leaves no trace</em>
</p>

> **bothy** *(n., Scottish)* — a small unlocked mountain shelter, free for anyone
> to use, kept by two customs: leave it as you found it, and leave fuel for the
> next visitor.

One command opens a coding agent, a file browser showing what it is touching,
and a shell — all in one terminal window that stays put:

```sh
cd ~/some-project
bothy
```

<p align="center">
  <img alt="the bothy workspace: a Yazi file browser across the top with a file preview, an agent pane and a shell below, inside Zellij" src="docs/images/workspace.png" width="900">
</p>

bothy installs any tool you are missing, writes its own configs, arranges the
panes, then tells you what is broken and how to fix it. It leaves your dotfiles
alone. Everything it manages lives in one directory, and `bothy uninstall`
removes it.

The default layout is built for one job: watching an agent work on a repository
and staying in the loop while it does. The file browser is there so you can see
what changed, the shell so you can check it, and the whole thing is disposable
so you can try it on a machine you do not want to keep it on. bothy runs
perfectly well with `slots.agent none` if you want the workspace without the
agent — but the agent is what it was shaped around.

## Install

### What you need first

| | |
|---|---|
| **git** | Required. bothy uses it to fetch its Yazi plugins. |
| **curl** or **wget** | Only for the install script below. |
| **[Ghostty](https://ghostty.org)** | Recommended. Yazi can only draw real image previews in a terminal that supports them, which means Ghostty, Kitty or WezTerm. In anything else you get a rough approximation made of text characters. |
| **an AI agent** | The main pane runs `claude` by default. bothy will not install one — install methods and credentials are yours — and `bothy doctor` names the command for the agent you have chosen. Set `slots.agent none` to run without one. |
| **Go** and **make** | Only if you build from source. |

bothy provides everything else. If any of these are missing, `bothy doctor`
says so and gives you the command to fix it on your system.

### Getting bothy

Pick whichever suits you. You get the same program and version from all five.
Only the `.deb` is a file rather than a source your package manager can come
back to, so upgrading it means downloading the next one.

| | for | |
|---|---|---|
| **Script** | anyone on Linux or macOS | `curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh \| sh` |
| **dnf** | Fedora Workstation | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy` |
| **apt** | Debian, Ubuntu, Mint | [download the `.deb`](https://github.com/bspeelm/bothy/releases/latest) then `sudo apt install ./bothy_*.deb` |
| **Go** | if you already have Go | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| **Source** | contributors | `git clone` then `make install-binary` |

Then run this from any directory:

```sh
bothy
```

The first run sets everything up. It lists the tools you are missing, asks
before downloading them, then launches.

> On Silverblue and other image-based systems, installing with `dnf` requires
> `rpm-ostree install` and a reboot. The script needs neither.

> The `.deb` is a file, not a repository. `apt upgrade` will not bring you a new
> bothy; download the next one when you want it. If you would rather updates
> resolved themselves, use the script.

### Upgrading

`bothy upgrade` works out how this copy was installed and prints the command
for it. It never replaces the binary itself — that is your package manager's
job, or the install script's.

| installed with | upgrade with |
|---|---|
| the script | run the same one-liner again |
| dnf | `sudo dnf upgrade bothy` |
| apt | download the next `.deb` and `sudo apt install ./bothy_*.deb` |
| `go install` | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| source | `git pull && make install-binary` |

**Then run `bothy install`.** The templates are compiled into the binary, so a
newer bothy carries newer configs — and a launch does not re-render them.
`bothy doctor` says so when the two disagree, but it is easier to just do it.

### What gets installed

If you have none of them already, bothy downloads these nine tools — about
131 MB of archives, which unpack to roughly 124 MB of binaries:

| | | | |
|---|---|---|---|
| zellij | yazi | lazygit | delta |
| ripgrep | fzf | fd | jq |
| zoxide | | | |

Each download is checked against a recorded checksum before bothy installs it.

If you already have current versions, bothy downloads nothing and tells you
which of yours it is using. It never calls your package manager, never needs
root, and never adds anything to your `PATH`.

Two things it will not install for you: Ghostty, because it ships no
ready-made binaries and every way of installing it needs root, and the agent,
because your AI tools and their credentials are yours to manage. `bothy doctor`
gives you the right command for both.

## Commands

| | |
|---|---|
| `bothy` | Launch the workspace |
| `bothy attach` | Reconnect to a running session |
| `bothy doctor` | What is broken, and how to fix it (`--json` for CI) |
| `bothy install` | Re-apply your settings after changing them |
| `bothy tools` | Which tools are in use, and where they came from |
| `bothy outdated` | Which pinned tools have newer releases upstream |
| `bothy upgrade` | How to upgrade this copy of bothy |
| `bothy config set <key> <value>` | Change a setting |
| `bothy layout` | Print the layout that would be launched |
| `bothy theme example` | Print a blank palette file |
| `bothy desktop-entry` | Print a `.desktop` launcher (`--install` writes it) |
| `bothy uninstall` | Remove bothy and everything it installed |

## What it touches

```
~/.local/share/bothy/     bothy's files: configs, and any tool it installed
~/.config/bothy/          your files: settings, palette, overrides
```

Nothing else. Not `~/.config/yazi`, not `~/.vimrc`, not `~/.bashrc`, not your
git config. bothy writes its configs into its own folder and points each tool
there when it launches, using environment variables that apply only to that
session. Your shell keeps its own `PATH` and `EDITOR`.

The second folder is your setup. Put it in git, clone it on a new machine, run
`bothy`, and you have the same workspace.

### Using your own config instead

If you already have a Yazi setup you like, keep it:

```toml
# ~/.config/bothy/config.toml
passthrough = ["yazi"]
```

bothy then points Yazi at your config rather than its own. To adjust bothy's
config rather than replace it, put a file in
`~/.config/bothy/overrides/<tool>/<file>`. bothy adds it to the end of its own,
so your settings take priority.

## Swapping parts

Every part of the workspace can be changed:

| part | default | alternatives |
|---|---|---|
| terminal | ghostty | kitty, wezterm (bothy never installs these) |
| multiplexer | zellij | none yet |
| file browser | yazi | turn it off |
| editor | vim | nano, helix (bothy installs none of these) |
| agent | claude-code | any command you name |
| theme | dracula | any palette you write |

```sh
bothy config set slots.editor helix
```

bothy does not install editors. An editor is the most personal tool in the
workspace and the one you are likeliest to already have, so the slot names the
command and sets `EDITOR` for the session — it does not fetch anything. If the
one you name is not installed, `bothy doctor` says so and gives you the command
for your distribution.

Three layouts come with bothy, called profiles. `cockpit` is the default, with
the file browser on top and the agent and shell below. `editor` gives you an
editor, agent and shell side by side. `minimal` is just an agent and a shell,
for small screens and SSH.

```sh
bothy config set profile minimal
```

Profiles are short TOML files, so you can write your own and put it in
`~/.config/bothy/profiles/`.

## Theming

bothy comes with the [Dracula](https://github.com/dracula/dracula-theme)
palette and colours every tool from the same eleven values, including a vim
colour scheme it generates for you. To use a different palette, write your own:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how to use a palette you have paid for. bothy contains no colours
except its own, so anything you licensed stays on your machine.

## Where it runs

If your terminal can draw images (Ghostty, Kitty, WezTerm), bothy runs there.
If it cannot, bothy opens a Ghostty window instead, so that image previews
still work rather than silently turning into text art. Use `--in-place` or
`--window` to force either one. With no graphical display, such as over SSH, it
always runs where you are.

Inside a Toolbx or Distrobox container, bothy remembers which container it
installed its tools in and returns there when you launch it from the host. If
you install from a container that has none of the tools, bothy downloads all of
them into its own folder, and the result then works from anywhere.

## What bothy is not

- A Windows tool, unless you use WSL
- A tmux setup
- A plugin marketplace or extension API
- A bundle of the tools. It downloads official releases and verifies them
- An LSP or debugger manager
- A background service, auto-updater, or telemetry collector
- A manager for your agent's config, keys or hooks
- A Flatpak. Flathub does not accept command-line software, and bothy
  downloads its tools as it runs, which Flatpak packaging is designed to avoid

## Contributing

See [`docs/adding-a-provider.md`](docs/adding-a-provider.md). Adding support
for a new tool means writing a config file and some templates, not Go code.

Why things are the way they are is recorded in
[`docs/decisions.md`](docs/decisions.md), and the plan for the project is in
[`PLAN.md`](docs/PLAN.md).

## Credits and licence

MIT — see [`LICENSE`](LICENSE).

The built-in palette is [Dracula](https://github.com/dracula/dracula-theme)
(MIT). bothy does not bundle any of the tools it installs. It downloads their
official releases, checks them, and each keeps its own licence.
[`NOTICE`](NOTICE) lists them all.
