# bothy

*A room you can trust because it wants nothing. The agent, of course, is
another matter.*

You type one word in a directory and get three panes: your files above, an
agent below on the left, and a shell on the right for when the agent has done
something and you want to see whether it's true.

```
cd ~/some-project
bothy
```

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

### What you need

|                |                                                                                              |
| -------------- | -------------------------------------------------------------------------------------------- |
| **git**        | Required. bothy uses it to fetch the file browser's plugins.                                 |
| **curl** or **wget** | Only for the install script.                                                           |
| **[Ghostty](https://ghostty.org)** | Recommended, not required. Image previews need a terminal that can draw them — Ghostty, Kitty or WezTerm. In any other you get a rough approximation made of characters. |
| **an agent**   | Optional, though it is rather the point. The middle pane runs `claude` unless told otherwise, and sits empty if there is nothing to run. |
| **Go**, **make** | Only if you build from source.                                                             |

If anything is missing, `bothy doctor` will say so, and say what to type.

### Getting it

Any of these gives you the same program at the same version.

|            |                          |                                                                                             |
| ---------- | ------------------------ | ------------------------------------------------------------------------------------------- |
| **Script** | Linux or macOS           | `curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh \| sh` |
| **dnf**    | Fedora                   | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy`                            |
| **Go**     | people who have Go       | `go install github.com/bspeelm/bothy/cmd/bothy@latest`                                      |
| **Source** | contributors             | `git clone`, then `make install-binary`                                                     |

Then, from anywhere:

```
bothy
```

The first run sets things up and asks before downloading anything.

> On Silverblue and other image-based systems, `dnf` means `rpm-ostree
> install` and a reboot. The script needs neither.

### What it might download

If you have none of them, nine tools, about 131 MB — a fair amount of disk
for three panes, and mentioned here so that it is not a surprise:

|         |      |         |       |
| ------- | ---- | ------- | ----- |
| zellij  | yazi | lazygit | delta |
| ripgrep | fzf  | fd      | jq    |
| zoxide  |      |         |       |

Each download is checked against a recorded checksum before bothy keeps it.
If you already have a good enough copy, bothy leaves it alone and tells you
which of yours it is using. It never calls your package manager, never asks
for root, and never adds anything to your `PATH`.

Two things it will not fetch for you: the terminal, because Ghostty ships no
binaries and every route to it needs root; and the agent, because your AI
tools and their credentials are your business. `bothy doctor` gives you the
command for both.

## Commands

|                                  |                                                        |
| -------------------------------- | ------------------------------------------------------ |
| `bothy`                          | Open the workspace                                     |
| `bothy attach`                   | Return to one you left running                         |
| `bothy doctor`                   | What is wrong, and what to do about it (`--json` for CI) |
| `bothy install`                  | Apply your settings again after changing them          |
| `bothy tools`                    | Which tools are in use and where they came from        |
| `bothy config set <key> <value>` | Change a setting                                       |
| `bothy layout`                   | Print the layout it would open                         |
| `bothy theme example`            | Print a blank palette to fill in                       |
| `bothy desktop-entry`            | Print a `.desktop` launcher (`--install` writes it)    |
| `bothy uninstall`                | Remove it, and everything it brought                   |

## What it touches

```
~/.local/share/bothy/     its things: configs, and any tool it fetched
~/.config/bothy/          your things: settings, palette, overrides
```

And nothing else. Not `~/.config/yazi`, not `~/.vimrc`, not `~/.bashrc`, not
your git config. bothy writes into its own folder and, when it launches,
tells each tool to look there — using environment variables that exist for
that session and then stop existing. Your shell keeps its own `PATH` and its
own idea of what an editor is.

The second folder is yours. Put it in git. Clone it on the next machine. Run
`bothy`. You will have the same room, which is about as much continuity as
anyone gets.

### If you already have a setup you like

Keep it.

```
# ~/.config/bothy/config.toml
passthrough = ["yazi"]
```

bothy will point Yazi at your config instead of its own. To adjust bothy's
config rather than replace it, put a file in
`~/.config/bothy/overrides/<tool>/<file>`; it is appended after bothy's, so
yours wins.

## Swapping parts

Everything can be changed, though most of it needn't be.

| part         | default     | alternatives                                  |
| ------------ | ----------- | --------------------------------------------- |
| terminal     | ghostty     | kitty, wezterm — none of which bothy installs |
| multiplexer  | zellij      | nothing else, for now                         |
| file browser | yazi        | or turn it off                                |
| editor       | vim         | nano, helix                                   |
| agent        | claude-code | any command you care to name                  |
| theme        | dracula     | any palette you write down                    |

```
bothy config set slots.editor helix
```

Most people change the editor and nothing else. The others change
everything, once, and then also nothing else.

Three layouts are included. `cockpit` is the default: files on top, agent and
shell beneath, which is what the screenshot shows and what most of this was
built for. `editor` puts an editor, an agent and a shell side by side.
`minimal` is an agent and a shell and nothing else, for small screens and for
being somewhere else over SSH.

```
bothy config set profile minimal
```

They are short TOML files. Write your own and put it in
`~/.config/bothy/profiles/`.

## Theming

bothy ships one palette, Dracula, and colours every pane from the same eleven
values, including a vim colour scheme it writes for you. If you would prefer
another, write it:

```
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how a palette you have paid for gets in. bothy contains no
colours but its own, so anything you licensed stays on your machine.

## Where it runs

If your terminal can draw images, bothy runs in it. If it cannot, bothy opens
a Ghostty window instead, so that previews are pictures rather than
approximations. `--in-place` and `--window` will override that judgement
either way. Over SSH, with no display to open a window into, it stays where
it is, which is generally the right thing to do when somewhere else.

Inside a Toolbx or Distrobox container, bothy remembers where it put its
tools and goes back there when you launch it from the host. If you install
from a container that has none of them, it downloads the lot into its own
folder and the result works from either side.

As for which machines: Fedora and Ubuntu, on every release, installed into a
container and exercised and thrown away. That is the whole of what supported
means here — not that it ought to work, but that something proved it did this
morning.

macOS is a different sentence. The binaries are built, every tool is pinned
for it, and the install advice knows about Homebrew, but nothing has ever run
there. It will install and it will mostly work; it will also never open its
own window, because it looks for a display in the places Linux keeps one.
Untested, which is the honest word.

Anywhere else, bothy runs and the doctor tells you what it cannot do on that
machine. Nobody has checked, and it does not pretend otherwise.

## What bothy is not

- A plugin system, a marketplace, or an extension API
- A bundle of the tools. It fetches their official releases and checks them
- Anything to do with language servers or debuggers
- A background service, an auto-updater, or a collector of telemetry
- A manager for your agent's config, keys, or hooks
- **A sandbox.** The agent runs as you, in your repository, with your
  permissions. Its edits are real and are not bothy's to undo. Uninstalling
  removes bothy, not anything the agent did
- A Flatpak. Flathub does not take command-line software, and bothy downloads
  things as it goes, which Flatpak was invented to prevent
- Ambitious

It is a room. You go in, the work happens, you leave. It keeps nothing of
yours, and it does not need to be thanked.

## Contributing

See [`docs/adding-a-provider.md`](docs/adding-a-provider.md). Adding a tool
bothy fetches is one config file. Adding one it configures is a file, some
templates and a single branch. Only the multiplexer needs Go that reads the
layout and writes something else; if anything else seems to, stop, because
something else is wrong. Most contributions so far have been to the
reasons rather than the code, which is either a good sign or the only sign.

What it is aiming at is in [`docs/north-star.md`](docs/north-star.md). The
reasons things are the way they are live in
[`docs/decisions.md`](docs/decisions.md). They are longer than the code they
explain, and on balance that is the right way round.

## Credits and licence

MIT — see [`LICENSE`](LICENSE).

The palette is [Dracula](https://github.com/dracula/dracula-theme), also MIT.
bothy bundles none of the tools it fetches; each keeps its own licence, and
[`NOTICE`](NOTICE) lists them.
