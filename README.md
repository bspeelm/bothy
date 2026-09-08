<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/images/bothy-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="docs/images/bothy-light.png">
    <img alt="bothy — a stone shelter in a landscape, drawn in ASCII" src="docs/images/bothy-light.png" width="820">
  </picture>
</p>

<h1 align="center">bothy</h1>

<p align="center">
  <em>It stands plain as a wardrobe, what we know,<br>
  Have always known, know that we can’t escape,<br>
  Yet can’t accept. One side will have to go.</em><br>
  — Philip Larkin, &ldquo;Aubade&rdquo;
</p>



A terminal workspace assembled from tools you already have. 
One command opens a file browser, an agent and a shell in one window, configured and checked.

```sh
cd ~/some-project
bothy
```

<p align="center">
  <img alt="the bothy workspace: a Yazi file browser across the top with a file preview, an agent pane and a shell below, inside Zellij" src="docs/images/workspace.png" width="900">
</p>

That is the entire idea. It is not a large one, and most of the effort went
into making sure it did nothing else.

## What happens when you type it

bothy looks at what you already have. If the tools are there, it uses them and
tells you so. If some are missing, it lists them and guides you through getting them.
Then it writes its configs into a folder of its own, points each tool
there for exactly as long as the session lasts, arranges the panes, and opens
the window. Afterwards it tells you what is broken, if anything is, and what
to do about it.

When you close the window, everything is as it was. When you uninstall it, the
folder it wrote goes, and it names the few things it cannot remove for you.

The same thing in order, with the reasons for the order, is
[what happens when you type it](https://github.com/bspeelm/bothy/wiki/What-happens-when-you-type-bothy).

## Install

You need **git**. Everything else bothy brings, or tells you how to get. A
terminal that can draw images — Ghostty, Kitty, WezTerm — makes previews real
pictures rather than block art, and an AI agent is optional, though it is what
the middle pane is for (and sort of the point).

Pick the one that fits your machine:

| | |
|---|---|
| **Fedora** | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy` |
| **Debian, Ubuntu, Mint** | download the `.deb` from [the latest release](https://github.com/bspeelm/bothy/releases/latest), then `sudo apt install ./bothy_*.deb` |
| **macOS** | `brew install --cask bspeelm/bothy/bothy` |
| **you already have Go** | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |

Then, from any directory:

```sh
bothy
```

The first run lists what you are missing, asks before downloading anything,
then opens the window. It is not fast. It does not need to be; you will do this
once.

### If none of those fit

There is an install script, and one case where it is genuinely the better
answer: an image-based system like Silverblue, where `dnf` means `rpm-ostree`
and a reboot for a binary that runs perfectly well out of `~/.local/bin`. It
needs no root and layers nothing onto the host.

```sh
curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh
```

The cost: the script is fetched over HTTPS and run **unsigned**, before bothy
exists to verify anything. No signature on a later artifact fixes that. It is
the same trade as any `curl | sh`.

There are six ways in, counting source, and each verifies what it fetched
differently. [All of them, and what checks what](https://github.com/bspeelm/bothy/wiki/Installing) ·
[What bothy downloads and what that proves](https://github.com/bspeelm/bothy/wiki/Security)

## Commands

Four of them matter:

| | |
|---|---|
| `bothy` | open the workspace |
| `bothy tower` | watch every running agent in one window |
| `bothy doctor` | what is wrong, and what to type (`--json` for machines) |
| `bothy config set <key> <value>` | change a setting |

There are nineteen. [All of them, with their flags](https://github.com/bspeelm/bothy/wiki/Commands), and
[how to read a doctor report](https://github.com/bspeelm/bothy/wiki/The-doctor).

## Watching several at once

Several sessions open across several windows, each with an agent working, and no
way to see them at once. `bothy tower` puts every running agent in one window.

```
$ bothy tower
watching 3 agent(s), side by side; 2 pane(s) expanded to be worth reading
```

Each pane mirrors one agent. Move between them with `Alt+h/j/k/l`, and type a
reply into the one that wants you — it goes to that agent as if you had typed it
in its own window. The bottom rows of a mirror are yours, so a reply stays on
screen while the agent above it keeps working.

**The tower reads panes and sends nothing of its own.** It never composes a
message, never answers on your behalf, and never sends anything on a timer.
Everything an agent receives came from your keyboard. It also attaches to
nothing, so watching a session cannot disturb it.

It expands each watched agent pane to fill its window first, because a mirror
can only show what its pane displays and a cockpit gives its agent about a
quarter of the screen. Your panes are put back when the tower closes.
[How it works, in
full](https://github.com/bspeelm/bothy/wiki/Commands#bothy-tower---mirror-session---every-duration).

## Toolboxes

A toolbox is a container that shares your home directory but keeps its own
installed packages. On Fedora Silverblue they are the normal place to install
things, and it is easy to end up with several — one per kind of work.

Toolboxes are great. Managing them is not. `toolbox list` prints a table no
script can read, there is no `toolbox stop` at all, and stopping one means a
`podman` command that isn't even available from inside a toolbox.

bothy makes them easy to manage from a session, and it remembers which toolbox
each project belongs in:

```
$ bothy box
~/code/legacy
  box       legacy
  because   this project is recorded for it

$ bothy box ls
* dev                      22 busy   bothy-api
  docs                     exited
  legacy                   idle      bothy-legacy
```

| | |
|---|---|
| `bothy box` | which toolbox this project uses, and why |
| `bothy box ls` | every toolbox, how much is running in it, and the sessions it holds |
| `bothy box use <name>` | move this project to a different toolbox |
| `bothy box stop <name>` | stop a toolbox nothing is using |
| `bothy box create <name>` | make one, and use it for this project |
| `bothy box rm <name>` | delete one |

The first time you open a project, bothy asks which toolbox to use, then
remembers your answer. If you have one toolbox, or none, it never asks.

bothy does not replace `toolbox`. Toolbox still makes the containers and still
enters them; bothy keeps track of which project goes where, which is the part
toolbox knows nothing about.
[The full rules](https://github.com/bspeelm/bothy/wiki/Toolboxes).

## Another machine

`bothy connect <client>` opens the workspace against a different machine. The file
browser shows its files, the shell pane is a real login session on it, and the
agent works on its files.

```
$ bothy connect <client>
<client> mounted at ~/.local/share/bothy/cache/remotes/<client>
```

**bothy puts nothing on the machine you connect to** — not itself, not a tool,
not a credential, not a temporary file. The only thing it uses over there is
the SSH server already running. If you can `ssh <client>` today, you can
`bothy connect <client>`.

The workspace itself still runs on your machine, which is why sessions,
`bothy ls` and `bothy attach` all keep working normally, and why a dropped
connection costs you the mount and not the session. Inside the workspace,
`on make test` runs something on that machine in the right directory.

It needs sshfs installed **on your own machine**, and bothy tells you the
command if it is missing. [How it works, in
full](https://github.com/bspeelm/bothy/wiki/Connecting).

## What it touches

```
~/.local/share/bothy/     bothy's things: configs, and any tool it fetched
~/.config/bothy/          your things: settings, palette, overrides
```

Nothing else. Not `~/.config/yazi`, not `~/.vimrc`, not `~/.bashrc`, not your
git config. bothy writes into its own folder and, when it launches, points each
tool there — with environment variables that last for that session and then do
not.

The second folder is yours. Put it in git, clone it on the next machine, run
`bothy`, and you have the same room.

`bothy uninstall` removes the first folder and the binary, and names the three
things it leaves rather than leaving you to find them.

[Using your own tool config instead of bothy's](https://github.com/bspeelm/bothy/wiki/Swapping-parts-and-theming) ·
[what uninstall leaves, and why](https://github.com/bspeelm/bothy/wiki/Installing#removing-it).

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
- A bundle of the tools. It downloads their official releases and checks them against `bothy.lock`
- An LSP or debugger manager
- A background service, an auto-updater, or a collector of telemetry. It does not run when you are not looking, and has nothing to report if it did
- A manager for your agent's config, keys or hooks. Those are yours, and so are the consequences
- A sandbox. The agent runs as you, in your repository, with your permissions. Its edits and commits are real and are not bothy's to undo. Uninstalling removes bothy — its tools, its configs — and nothing the agent did
- A Flatpak. Flathub does not accept command-line software, and bothy downloads its tools as it goes, which Flatpak packaging was invented to prevent

It is a room. You go in, the work happens, you leave, and it keeps nothing of
yours.

## What you can depend on

Within a major version: the `config.toml` keys, the profile and palette
schemas, the two directories, and the `doctor --json` shape. `config.toml`
carries `schema = 1`.

I will continue to make tweaks and push maintenance builds, but these are the features it ships with and most likely will stick with. 

[What that obliges, and what is deliberately not covered](https://github.com/bspeelm/bothy/wiki/What-you-can-depend-on).

## Contributing

See [`docs/adding-a-provider.md`](docs/adding-a-provider.md). Adding a tool
bothy fetches is one config file. Adding one it configures is a file and some
templates. Only the multiplexer needs Go, because it reads the layout and
writes something else; if anything else seems to, stop and say so — that is a
bug in the provider format, not in you.

What it is aiming at is in [`docs/north-star.md`](docs/north-star.md). Why
things are the way they are is recorded in
[`docs/decisions.md`](docs/decisions.md). 
The plan for the project is in [`docs/PLAN.md`](docs/PLAN.md), kept current
where the project has moved on from it.

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
