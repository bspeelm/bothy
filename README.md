<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/bothy-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/images/bothy-light.png">
    <img alt="bothy — a stone shelter in a landscape, drawn in ASCII" src="docs/images/bothy-light.png" width="820">
  </picture>
</p>

<h1 align="center">bothy</h1>

<p align="center">
  <em>a room you can trust because it wants nothing. the agent, of course, is another matter.</em>
</p>


You type one word in a directory and get three panes: your files above, an
agent below on the left, and a shell on the right for when the agent has done
something and you want to see whether it's true.

```sh
cd ~/some-project
bothy
```

<p align="center">
  <img alt="the bothy workspace: a Yazi file browser across the top with a file preview, an agent pane and a shell below, inside Zellij" src="docs/images/workspace.png" width="900">
</p>

That is the entire idea. It is not a large one. Most of the work that went
into it was making sure it did nothing else.

## What happens when you type it

bothy looks at what you have. If you already own the tools, it uses them and
says so. If you are missing some, it lists them, tells you what they weigh,
and waits for you to say yes. Then it writes its configs into a folder of its
own, points each tool there for the length of the session, and opens the
window. It does this every time, and every time it is slightly surprised to
find nothing has gone wrong.

When you close the window, everything is as it was. When you uninstall it,
the folder goes and nothing else does. This is less a feature than an
absence of features, but it took some doing.

## Install

### What you need first

| | |
|---|---|
| **git** | Required. bothy uses it to fetch its Yazi plugins. |
| **curl** or **wget** | Only for the install script below. |
| **[Ghostty](https://ghostty.org)** | Recommended, not required. Yazi can only draw real image previews in a terminal that supports them, which means Ghostty, Kitty or WezTerm. In anything else you get a rough approximation made of characters. |
| **an AI agent** | Optional, though it is rather the point. The middle pane runs `claude` unless told otherwise, and sits empty if there is nothing to run. |
| **Go** and **make** | Only if you build from source. |

bothy provides everything else. If any of these are missing, `bothy doctor`
says so, and says what to type.

### Getting bothy

Any of these gives you the same program at the same version.

| | for | |
|---|---|---|
| **Script** | anyone on Linux or macOS | `curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh \| sh` |
| **dnf** | Fedora Workstation | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy` |
| **Go** | people who have Go | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| **Source** | contributors | `git clone` then `make install-binary` |

Then, from any directory:

```sh
bothy
```

The first run sets things up. It lists the tools you are missing, asks before
downloading anything, then launches.

> On Silverblue and other image-based systems, installing with `dnf` requires
> `rpm-ostree install` and a reboot. The script needs neither.

### What gets installed

If you have none of them already, nine tools, about 131 MB — a fair amount of
disk for three panes, and mentioned here so that it is not a surprise:

| | | | |
|---|---|---|---|
| zellij | yazi | lazygit | delta |
| ripgrep | fzf | fd | jq |
| zoxide | | | |

Each download is checked against a recorded checksum before bothy keeps it.

If you already have good enough copies, bothy downloads nothing and tells you
which of yours it is using. It never calls your package manager, never needs
root, and never adds anything to your `PATH`.

Two things it will not install for you: Ghostty, because it ships no
ready-made binaries and every way of installing it needs root, and the agent,
because your AI tools and their credentials are your business. `bothy doctor`
gives you the right command for both.

## Commands

| | |
|---|---|
| `bothy` | Open the workspace |
| `bothy attach` | Return to one you left running |
| `bothy doctor` | What is wrong, and what to do about it (`--json` for CI) |
| `bothy install` | Apply your settings again after changing them |
| `bothy tools` | Which tools are in use, and where they came from |
| `bothy config set <key> <value>` | Change a setting |
| `bothy layout` | Print the layout it would open |
| `bothy theme example` | Print a blank palette file to fill in |
| `bothy desktop-entry` | Print a `.desktop` launcher (`--install` writes it) |
| `bothy uninstall` | Remove bothy, and everything it brought |

## What it touches

```
~/.local/share/bothy/     bothy's files: configs, and any tool it installed
~/.config/bothy/          your files: settings, palette, overrides
```

Nothing else. Not `~/.config/yazi`, not `~/.vimrc`, not `~/.bashrc`, not your
git config. bothy writes into its own folder and, when it launches, tells each
tool to look there — using environment variables that exist for that session
and then stop existing. Your shell keeps its own `PATH` and its own idea of
what an editor is.

The second folder is yours. Put it in git. Clone it on the next machine. Run
`bothy`. You will have the same room, which is about as much continuity as
anyone gets.

### Using your own config instead

If you already have a Yazi setup you like, keep it:

```toml
# ~/.config/bothy/config.toml
passthrough = ["yazi"]
```

bothy then points Yazi at your config rather than its own. To adjust bothy's
config rather than replace it, put a file in
`~/.config/bothy/overrides/<tool>/<file>`. bothy adds it to the end of its own,
so yours wins.

## Swapping parts

Everything can be changed, though most of it needn't be.

| part | default | alternatives |
|---|---|---|
| terminal | ghostty | kitty, wezterm (bothy never installs these) |
| multiplexer | zellij | nothing else, for now |
| file browser | yazi | or turn it off |
| editor | vim | nano, helix |
| agent | claude-code | any command you care to name |
| theme | dracula | any palette you write down |

```sh
bothy config set slots.editor helix
```

Most people change the editor and nothing else. The others change
everything, once, and then also nothing else.

Three layouts come with bothy, called profiles. `cockpit` is the default:
files on top, agent and shell beneath, which is what the screenshot shows and
what most of this was built for. `editor` puts an editor, an agent and a shell
side by side. `minimal` is an agent and a shell and nothing else, for small
screens and for being somewhere else over SSH.

```sh
bothy config set profile minimal
```

Profiles are short TOML files. Write your own and put it in
`~/.config/bothy/profiles/`.

## Theming

bothy ships one palette, [Dracula](https://github.com/dracula/dracula-theme),
and colours every tool from the same eleven values, including a vim colour
scheme it writes for you. If you would prefer another, write it:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how a palette you have paid for gets in. bothy contains no
colours except its own, so anything you licensed stays on your machine.

## Where it runs

If your terminal can draw images (Ghostty, Kitty, WezTerm), bothy runs there.
If it cannot, bothy opens a Ghostty window instead, so that previews are
pictures rather than approximations. `--in-place` and `--window` will
override that judgement either way. With no graphical display, such as over
SSH, it stays where it is, which is generally the right thing to do when
somewhere else.

Inside a Toolbx or Distrobox container, bothy remembers which container it
installed its tools in and returns there when you launch it from the host. If
you install from a container that has none of the tools, bothy downloads the
lot into its own folder, and the result then works from either side.

## What bothy is not

- A Windows tool, unless you count WSL, which you may
- A tmux setup
- A plugin marketplace or extension API
- A bundle of the tools. It downloads official releases and verifies them
- An LSP or debugger manager
- A background service, auto-updater, or telemetry collector
- A manager for your agent's config, keys or hooks
- A Flatpak. Flathub does not accept command-line software, and bothy
  downloads its tools as it runs, which Flatpak packaging is designed to avoid
- Ambitious

It is a room. You go in, the work happens, you leave. It keeps nothing of
yours, and it does not need to be thanked.

## Contributing

See [`docs/adding-a-provider.md`](docs/adding-a-provider.md). Adding support
for a new tool means writing a config file and some templates; if it seems to
need Go, stop, because something else is wrong. Most contributions so far have
been to the reasons rather than the code, which is either a good sign or the
only sign.

Why things are the way they are is recorded in
[`docs/decisions.md`](docs/decisions.md). It is longer than the code it
explains, and on balance that is the right way round. The plan for the
project is in [`docs/PLAN.md`](docs/PLAN.md).

## Credits and licence

MIT — see [`LICENSE`](LICENSE).

The built-in palette is [Dracula](https://github.com/dracula/dracula-theme)
(MIT). bothy does not bundle any of the tools it installs. It downloads their
official releases, checks them, and each keeps its own licence.
[`NOTICE`](NOTICE) lists them all.
