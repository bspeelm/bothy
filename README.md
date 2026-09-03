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
agent below on the left, and a shell on the right for when the agent says it
has done something and you would like to see whether it has.

```sh
cd ~/some-project
bothy
```

<p align="center">
  <img alt="the bothy workspace: a Yazi file browser across the top with a file preview, an agent pane and a shell below, inside Zellij" src="docs/images/workspace.png" width="900">
</p>

That is the entire idea. It is not a large one. Most of the effort went into
making sure it did nothing else, and it still occasionally has to be reminded.

## What happens when you type it

bothy looks at what you already have. If the tools are there, it uses them and
tells you so, with the faint air of someone who expected to have to do more.
If some are missing, it lists them, says what they weigh, and waits for you to
say yes. Then it writes its configs into a folder of its own, points each tool
there for exactly as long as the session lasts, arranges the panes, and opens
the window. Afterwards it tells you what is broken, if anything is, and what
to type about it.

When you close the window, everything is as it was. When you uninstall it, the
folder it wrote goes, and it names the few things it cannot remove for you.
This is less a feature than an absence of features, and it was harder than the
features.

The same thing in order, with the reasons for the order, is
[`docs/what-happens.md`](docs/what-happens.md).

## Install

### What you need first

| | |
|---|---|
| **git** | Required. bothy uses it to fetch its Yazi plugins. You have it. Everyone has it. |
| **curl** or **wget** | Only for the install script below, and only one of them. |
| **[Ghostty](https://ghostty.org)** | Recommended, not required. Yazi can only draw real image previews in a terminal that can draw images, which means Ghostty, Kitty or WezTerm. In anything else you get an approximation made of characters, which is roughly what terminals have been offering since 1978. |
| **an AI agent** | Optional, though it is rather the point. The middle pane runs `claude` unless told otherwise, and sits empty if there is nothing to run. |
| **Go** and **make** | Only if you build from source, which nobody has to. |

bothy brings everything else. If any of the above is missing, `bothy doctor`
says so, says what to type, and leaves it at that.

### Getting bothy

Six ways in. They all arrive at the same program and the same version.

| | for | |
|---|---|---|
| **Script** | anyone on Linux or macOS, which is most of the people who would want this | `curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh \| sh` |
| **Homebrew** | macOS, and Linux if you already use brew | `brew install --cask bspeelm/bothy/bothy` — read [macOS and Gatekeeper](#macos-and-gatekeeper) first |
| **dnf** | Fedora Workstation | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy` |
| **apt** | Debian, Ubuntu, Mint | [download the `.deb`](https://github.com/bspeelm/bothy/releases/latest), then `sudo apt install ./bothy_*.deb` |
| **Go** | people who already have Go | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| **Source** | contributors, and those who like to see for themselves | `git clone` then `make install-binary` |

### macOS and Gatekeeper

bothy is not signed with an Apple Developer ID, so macOS may refuse to run a
copy a browser or Homebrew downloaded. The Homebrew cask clears the flag for
you as it installs — a Gatekeeper check skipped on your behalf, which you
should know about. `curl` never attaches the flag, so the script is unaffected.

[What the warning means, and how to clear it by hand](https://github.com/bspeelm/bothy/wiki/Installing-and-verifying#macos-and-gatekeeper).

### Checking what you got

Every release artifact is signed by the workflow that built it, in a public
log — so a swapped download is detectable whether or not anyone checks. dnf and
`go install` verify automatically; for the rest the signature is one command,
opt-in because it needs the `gh` CLI.

[Which channel checks what, and the two commands](https://github.com/bspeelm/bothy/wiki/Installing-and-verifying#checking-what-you-got).

Then, from any directory you happen to be in:

```sh
bothy
```

The first run sets things up. It lists what you are missing, asks before
downloading anything, then opens the window. It is not fast. It does not need
to be; you will do this once.

### What gets installed

If you have none of them, eight tools: about 49 MB to download, roughly 124 MB
on disk once unpacked. That is a fair amount of disk for three panes, and it is
written here so that you find out from the README rather than from `du`.

| | | | |
|---|---|---|---|
| zellij | yazi | lazygit | ripgrep |
| fzf | fd | jq | zoxide |

Each is pinned to a version and a checksum in `bothy.lock`, and checked before
bothy keeps it. These are other projects' releases, so a checksum is as far as
it goes — bothy does not sign what it did not build.

If you already have good enough copies, bothy downloads nothing and tells you
which of yours it is using. What it did fetch, it keeps: when a later bothy
pins a newer version, the next `bothy install` fetches that one and leaves
your own copies exactly where they were.

It never calls your package manager, never asks for root, and never adds
anything to your `PATH`. It is aware that this is unusual, and would prefer
not to discuss it.

Two things it will not install for you. Ghostty, because it ships no
ready-made binaries and every route to it needs root. And the agent, because
your AI tools and their credentials are your business, and bothy has enough
of its own. `bothy doctor` gives you the right command for both and then goes
quiet.

## Commands

Three of them matter:

| | |
|---|---|
| `bothy` | open the workspace |
| `bothy doctor` | what is wrong, and what to type (`--json` for machines) |
| `bothy config set <key> <value>` | change a setting |

There are fifteen. [All of them, with their flags](https://github.com/bspeelm/bothy/wiki/Commands), and
[how to read a doctor report](https://github.com/bspeelm/bothy/wiki/The-doctor).

## What it touches

```
~/.local/share/bothy/     bothy's things: configs, and any tool it fetched
~/.config/bothy/          your things: settings, palette, overrides
```

Nothing else. Not `~/.config/yazi`, not `~/.vimrc`, not `~/.bashrc`, not your
git config. bothy writes into its own folder and, when it launches, tells each
tool to look there — using environment variables that exist for that session
and afterwards do not. Your shell keeps its own `PATH` and its own opinion of
what an editor is, and bothy has learned not to ask.

The second folder is yours. Put it in git. Clone it on the next machine. Run
`bothy`. You will have the same room, which is about as much continuity as
anyone is offered.

`bothy uninstall` removes the first folder and the binary. Three things are
left, and it names each on the way out rather than leaving you to find them:

| left behind | why | remove it with |
|---|---|---|
| `~/.config/bothy` | your settings, not bothy's | `rm -r ~/.config/bothy` |
| the container image, if you confined the agent | ~550 MB bothy did not build | `podman rmi bothy-agent` |
| the desktop entry, if you added one | outside the tree by necessity | `bothy desktop-entry --remove` |

### Using your own config instead

If you already have a Yazi setup you like, keep it. Most people who have one
do.

```toml
# ~/.config/bothy/config.toml
passthrough = ["browser"]
```

bothy then points Yazi at your config rather than its own, and does not read
yours on the way past. Name the slot rather than what is in it — `"yazi"` is
understood too, and stops meaning anything the day you put something else in
the browser slot. To adjust bothy's config rather than replace it, put a
file in `~/.config/bothy/overrides/<tool>/<file>`. bothy adds it to the end
of its own, so yours wins, which is the correct ending to most arguments
between a tool and the person who has to use it.

## Swapping parts

bothy has five slots — terminal, multiplexer, browser, editor, agent — and each
is a name in a config file. Change one, run `bothy install`, and it tells you
what that stack can and cannot give you.

[Swapping parts, and theming](https://github.com/bspeelm/bothy/wiki/Swapping-parts-and-theming).

## Walling off the agent

The agent slot runs a command with everything you can reach: every repository,
`~/.ssh`, your shell history. That is the same access it would have if you
started it by hand, so bothy is not making it worse — but bothy owns the
launch, which is a position to make it better.

`bothy confine` runs the agent pane in a rootless podman container. Nothing
else changes: the same layout, the same file browser, the same shell.

**It is opt-in and there is no setting that turns it on.** Never type the
command and nothing about bothy is different.

### What it stops, and what it does not

**Stops:** every other project, `~/.ssh`, `~/.aws`, your shell history, the
rest of `$HOME`. Verified, not assumed — from inside the pane those paths do
not exist.

**Does not stop, on purpose:**

| | |
|---|---|
| the agent's own credentials | mounted, or it cannot log in and the wall protects nothing you wanted. The paths come from the agent's own file in `slots/`; for one bothy has not learned, set `agent.credentials` |
| the network | the agent calls its API; that is the job. This is a filesystem wall, not a network one |
| the project directory | mounted writable, because editing it is the point |

If the credentials are missing the agent starts and says "Not logged in"
rather than failing — that is the agent's behaviour, not bothy's.

[Setting it up, the toolbox case, configuration and cleanup](https://github.com/bspeelm/bothy/wiki/Walling-off-the-agent).

## Where it runs

Linux and macOS. Fedora, Ubuntu, Debian and Arch are installed, exercised and
uninstalled in containers on every release, and macOS on a real Mac — that is
what supported means here. Silverblue and the Debian derivatives get advice
bothy cannot test in a container, and says so.

[Which terminals, which stacks, and what is untested](https://github.com/bspeelm/bothy/wiki/Where-it-runs).

## What bothy is not

- A plugin marketplace or extension API
- A bundle of the tools. It downloads their official releases and checks them, which is different, and the difference is the point
- An LSP or debugger manager
- A background service, an auto-updater, or a collector of telemetry. It does not run when you are not looking, and has nothing to report if it did
- A manager for your agent's config, keys or hooks. Those are yours, and so are the consequences
- A sandbox. The agent runs as you, in your repository, with your permissions. Its edits and commits are real and are not bothy's to undo. Uninstalling removes bothy — its tools, its configs — and nothing the agent did
- A Flatpak. Flathub does not accept command-line software, and bothy downloads its tools as it goes, which Flatpak packaging was invented to prevent
- Ambitious

It is a room. You go in, the work happens, you leave. It keeps nothing of
yours, and it does not need to be thanked.

## What you can depend on

Within a major version: the `config.toml` keys, the profile and palette
schemas, the two directories, and the `doctor --json` shape. `config.toml`
carries `schema = 1`.

[What that obliges, and what is deliberately not covered](https://github.com/bspeelm/bothy/wiki/What-you-can-depend-on).

## Contributing

See [`docs/adding-a-provider.md`](docs/adding-a-provider.md). Adding a tool
bothy fetches is one config file. Adding one it configures is a file and some
templates — it used to need a branch in Go as well, and no longer does. Only
the multiplexer needs Go, because it reads the layout and writes something
else; if anything else seems to, stop, because something has gone wrong, and
it is probably ours. Most contributions so
far have been to the reasons rather than the code, which is either a good sign
or the only one.

What it is aiming at is in [`docs/north-star.md`](docs/north-star.md). Why
things are the way they are is recorded in
[`docs/decisions.md`](docs/decisions.md). It is longer than the code it
explains, and on balance that is the right way round. The plan for the
project is in [`docs/PLAN.md`](docs/PLAN.md), and has survived contact with
the project better than most plans do, which is to say partially.

## Authorship
I wanted an easy way to make my development setup portable — all of the things I use and nothing I don't. It started as a cheatsheet I'd hand to an agent to set up the configuration; that's still in the docs if you're curious, and it's probably where this should have stopped. But while working on another project I felt the pull to go through every step of shipping something with a fully open AI workflow, and this was about as low-stakes a candidate as it gets.

I designed the architecture and the constraints and made the decisions, while Claude planned and executed within them. The decisions are recorded in [`docs/decisions.md`](docs/decisions.md), and the rules Claude worked under are in [`CLAUDE.md`](CLAUDE.md).

Claude wrote most of this code. I've since reviewed the load-bearing paths and the tests, with particular attention to the sensitive bits: the install script, the uninstall path, and the container invocation. The rest is verified by process — the test suite, the code and comment budgets enforced in the Makefile, and audits by AI systems other than the one that wrote the code.

To say it plainly: this is a small, zero-stakes project where keeping iterations fast while manufacturing the rigor was the point. Learning to use these tools can still produce something useful, and that is where we find ourselves.

A longer account of how this was built is
[here](https://bspeelm.github.io/bothy/how-it-was-built.html).

## Credits and licence

MIT — see [`LICENSE`](LICENSE).

The built-in palette is [Dracula](https://github.com/dracula/dracula-theme),
also MIT. bothy does not bundle the tools it installs. It downloads their
official releases, checks them, and each keeps its own licence.
[`NOTICE`](NOTICE) lists them all, as is only right.
