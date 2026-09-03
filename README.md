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

bothy is not signed with an Apple Developer ID. macOS attaches
`com.apple.quarantine` to anything a browser or Homebrew downloads, and
Gatekeeper refuses to run an unsigned file carrying it:

> **"bothy" not opened.** Apple could not verify "bothy" is free of malware
> that may harm your Mac or compromise your privacy.

That is not a claim anything is wrong with the binary. Gatekeeper asks whether
Apple can identify a paying developer account behind it; the answer is no.
bothy's releases are signed by the workflow that built them and recorded in a
public log (ADR-030), which says where the bytes came from — a stronger
statement, and a different question from the one being asked.

**The cask clears the flag for you, and you should know that it does.** There
is no longer a way to ask Homebrew not to quarantine — `--no-quarantine` was
removed in Homebrew 4.7 — so the cask runs `xattr -dr com.apple.quarantine` on
the binary as it installs. That is a Gatekeeper check being skipped on your
behalf. If you would rather it were not, install by another route: `curl` does
not attach the flag, so the script below is unaffected.

```sh
brew install --cask bspeelm/bothy/bothy
```

If you have a copy that macOS is already refusing — installed before this
landed, or downloaded from the releases page in a browser — clear it by hand:

```sh
xattr -dr com.apple.quarantine "$(which bothy)"
```

`bothy doctor` recognises the flag and prints that command with the right path
filled in.

### Checking what you got

Every release artifact is signed by the workflow that built it, in a public
log — so a swapped download is detectable whether or not anyone checks.

| installed with | checked by |
|---|---|
| Homebrew | the sha256 in the cask, automatic; the signature on request |
| dnf | dnf, against Copr's key — automatic |
| `go install` | Go, against [sum.golang.org](https://sum.golang.org) — automatic |
| script | a checksum, automatic; the signature on request |
| `.deb` | the signature on request |
| source | you compiled it yourself |

The checksum catches a corrupted download. The signature also says the bytes
came out of this repository's workflow; checking it needs the
[`gh` CLI](https://cli.github.com), so it is opt-in. Without it you are
trusting HTTPS and GitHub, as you do to clone the repository.

```sh
curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh -s -- --verify
gh attestation verify ./bothy_*.deb --repo bspeelm/bothy --bundle attestation.jsonl
```

Take `attestation.jsonl` from the release page. Neither command needs a GitHub
account, and neither passes quietly when it cannot check.

Then, from any directory you happen to be in:

```sh
bothy
```

The first run sets things up. It lists what you are missing, asks before
downloading anything, then opens the window. It is not fast. It does not need
to be; you will do this once.

> On Silverblue and other image-based systems, `dnf` means `rpm-ostree
> install` and a reboot, which is the price of an operating system that does
> not change under you. The script needs neither.

> The `.deb` is a file rather than a repository, so `apt upgrade` will not
> bring you the next one; download it when you want it. Running a Debian
> archive is a commitment, and this one has not made it.

### What gets installed

If you have none of them, eight tools, about 117 MB. That is a fair amount of
disk for three panes, and it is written here so that you find out from the
README rather than from `du`.

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

| | |
|---|---|
| `bothy` | Open the workspace |
| `bothy attach` | Go back to one you left running, which has gone on without you |
| `bothy ls` | Which of them are running |
| `bothy keys` | The bindings worth knowing, for a first day |
| `bothy confine` | Run the agent walled off from the rest of `$HOME` |
| `bothy doctor` | What is wrong, and what to do about it (`--json`, for machines that want to know) |
| `bothy install` | Apply your settings again after you have changed them |
| `bothy tools` | Which tools are in use, and where they came from, in case of dispute |
| `bothy upgrade` | How to upgrade this copy, for the way you installed it |
| `bothy outdated` | Which pinned tools have newer releases upstream (`--json`) |
| `bothy config set <key> <value>` | Change a setting |
| `bothy layout` | Print the layout it would open, should you doubt it |
| `bothy theme example` | Print a blank palette, for filling in |
| `bothy desktop-entry` | Print a `.desktop` launcher (`--install` writes it) |
| `bothy uninstall` | Remove bothy, and everything it brought, and nothing it did not |

### More than one project at a time

Each directory gets its own session, named after it, so there is nothing to
tear down when you move between them:

```sh
cd ~/other-project && bothy    # a second room; the first keeps running
bothy ls                       # both of them
bothy attach bothy-first       # back to the one you left, agent and all
```

Sessions survive closing the window. They do not survive a reboot, which is
the correct amount of permanence for a room.

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

Every part can be changed. Most of it needn't be, and most of it won't be.

| part | default | alternatives |
|---|---|---|
| terminal | ghostty | kitty, wezterm — none of which bothy installs |
| multiplexer | zellij | or turn it off, and run the agent in this terminal |
| file browser | yazi | or turn it off |
| editor | vim | nano, helix |
| agent | claude-code | any command you care to name |
| theme | dracula | any palette you write down |

```sh
bothy config set slots.editor helix
```

Most people change the editor and nothing else. The others change everything,
once, and then also nothing else.

Three layouts come with bothy, and are called profiles because everything
has to be called something. `cockpit` is the default: files on top, agent and
shell beneath — the screenshot, and the reason any of this exists. `editor`
puts an editor, an agent and a shell side by side, for people who would
rather type than watch. `minimal` is an agent and a shell and nothing else,
for small screens and for being somewhere else over SSH.

```sh
bothy config set profile minimal
```

Profiles are short TOML files. Write your own and put it in
`~/.config/bothy/profiles/`.

## Walling off the agent

The agent slot runs a command with everything you can reach: every repository,
`~/.ssh`, your shell history. That is the same access it would have if you
started it by hand, so bothy is not making it worse — but bothy owns the
launch, which is a position to make it better.

`bothy confine` runs the agent pane in a rootless podman container. Nothing
else changes: the same layout, the same file browser, the same shell.

**It is opt-in and there is no setting that turns it on.** Never type the
command and nothing about bothy is different.

### Setting it up

Three commands, once. The first one is the instructions — you do not have to
know the other two in advance:

```sh
bothy confine
# bothy: the agent needs an image to run in, and does not have one yet.
#
#       bothy wrote the recipe to
#         ~/.local/share/bothy/confine/Containerfile
#
#       build it — this is yours to run, not bothy's:
#         podman build -t bothy-agent ~/.local/share/bothy/confine
#
#       then: bothy confine

podman build -t bothy-agent ~/.local/share/bothy/confine   # about 550 MB, a few minutes
bothy confine                                              # now it launches
```

bothy writes that recipe and never builds it. Building installs an agent, and
bothy does not install agents — they change how they install, they need
credentials bothy has no business touching, and one arriving unasked is a
workspace tool overstepping. The Containerfile is yours once written: change
the agent, pin a version, add tools. bothy will not overwrite it.

`bothy confine --print` shows the recipe and the exact `podman run`, so you can
read the wall before trusting it.

### If bothy runs in a toolbox

This is the common case on Silverblue, and it has one wrinkle worth knowing:
**podman is on the host, not in the toolbox.** bothy handles that itself — it
reaches the host through `flatpak-spawn`, the same way it opens files — so
`bothy confine` works from inside a toolbox with nothing extra to configure.

Your own podman commands are the part that does not. Inside the toolbox:

```sh
podman build -t bothy-agent ~/.local/share/bothy/confine
# bash: podman: command not found

flatpak-spawn --host podman build -t bothy-agent ~/.local/share/bothy/confine
```

`bothy confine --print` prints the invocation with the hop already in it, so
you can see which case you are in.

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

### Configuration

One key, and it is optional:

```toml
[agent]
image = "bothy-agent"    # the image bothy runs the agent in; this is the default
```

There is deliberately no `confine = true`. A default that changes how the agent
runs would break for people who never asked for it and could not tell why.

### Cleaning up

`bothy uninstall` removes bothy's tree and the binary. The Containerfile is
inside the tree, so it goes with it. Three things are left, and uninstall names
each one rather than leaving you to find it:

| left behind | why | remove it with |
|---|---|---|
| `~/.config/bothy` | your settings, not bothy's | `rm -r ~/.config/bothy` |
| the container image | ~550 MB bothy did not build | `podman rmi bothy-agent` |
| the desktop entry, if you added one | outside the tree by necessity | `bothy desktop-entry --remove` |

```
$ bothy uninstall
removed ~/.local/share/bothy
  kept ~/.config/bothy (your settings — delete it yourself if you want it gone)
  kept the bothy-agent container image — remove it with: podman rmi bothy-agent
```

Inside a toolbox that last command comes back with the `flatpak-spawn` hop
already in it, because that is the command that works where you are standing.

### Where it works

Tested on Linux with podman. On macOS podman runs a Linux VM: it works, the
wall is real, and its edges are shaped differently — bothy says so and carries
on rather than pretending either way. With no podman at all, `bothy confine`
fails and tells you; it never silently runs unconfined.

## Theming

bothy ships one palette, [Dracula](https://github.com/dracula/dracula-theme),
which has outlasted most of the software it was first drawn for, and colours
every tool from the same eleven values, including a vim colour
scheme it writes for you so that you do not have to learn how. If you would
prefer another, write it:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how a palette you have paid for gets in. bothy contains no
colours except its own, so anything you licensed stays on your machine, and
bothy stays the colour it arrived in. A test checks this, because good
intentions do not.

## Where it runs

If your terminal can draw images (Ghostty, Kitty, WezTerm), bothy runs in it.
If it cannot, bothy opens a Ghostty window instead, so that previews come out
as pictures rather than as a suggestion of pictures. `--in-place` and
`--window` overrule that judgement in either direction, for those who know
better, or think they do. If you overrule it every time, make it the standing
answer with `bothy config set workspace.launch here`, or `window` for the
opposite; the flags still win for a single run. With no graphical display —
over SSH, say — it stays where it is, which is generally the sensible thing to
do when somewhere else.

iTerm2 draws images too, by a protocol of its own that Zellij does not carry,
so previews there arrive as characters after all. The doctor says which of the
two is at fault rather than leaving you to adjust the wrong setting.

Inside a Toolbx or Distrobox container, bothy remembers which container it
put its tools in and goes back there when you launch it from the host. If you
install from a container that has none of the tools, bothy downloads the lot
into its own folder, and the result works from either side. It has, on the
whole, had enough of being surprised by containers, and has written some of
this down.

As for which machines: Fedora, Ubuntu, Debian and Arch in containers, and
macOS on a Mac, on every release — installed, exercised, uninstalled, and the
whole doctor report compared against what it ought to have said. That is the
whole of what supported means here: not that it ought to work, but that
something proved it did this morning.

Two things bothy has advice for and does not test. **Silverblue and other
image-based systems**, where `dnf` means `rpm-ostree` and a reboot: bothy is
written on one and cannot put one in a container. **Mint, Pop!_OS and the other
derivatives**, which inherit Debian's advice through `ID_LIKE` — their container
images report `ID=ubuntu`, so a job using one would prove nothing the Ubuntu job
does not. That path is unit-tested instead.

macOS took eight releases to earn that sentence, having been listed from the
first on the strength of the binaries being built, which is not the same
thing. The first Mac to run it found a file opener naming a program macOS does
not have; the first machine in CI found an uninstall that reported success and
left the binary behind. Both are fixed. Neither would have been found by
reading the code.

Anywhere else, bothy runs and the doctor tells you what it cannot do on that
machine. Nobody has checked. It does not pretend otherwise.

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

These are the parts bothy will not change out from under you within a major
version. Rename or remove any of them and that is a breaking change, announced
as one.

- **The keys in `config.toml`.** Existing keys are not renamed or removed. When
  one has to change, the old name keeps working and `bothy doctor` names the
  replacement — that is what `config.Retired` is for.
- **The profile and palette TOML schemas**, so a profile you wrote keeps
  rendering.
- **The two directories.** `~/.local/share/bothy/` holds what bothy installs,
  `~/.config/bothy/` holds what you set. Neither moves.
- **The `doctor --json` shape**, and the check IDs in it. `--json` exists so
  something else can read it; IDs may be added, but an existing one keeps its
  meaning. New checks are additive, so parse defensively and ignore what you do
  not recognise.

`config.toml` carries `schema = 1`, written at the top of the file. It is
bothy's bookkeeping rather than a setting — `bothy config set` refuses it — and
it exists so a newer bothy can recognise an older file deliberately rather than
by absence. A config from a *newer* bothy still loads: `doctor` warns, and the
keys this version understands still work.

What is not covered: anything printed for a human to read, the layout of
`bothy doctor`'s normal output, and the internal Go packages. Those change when
there is a reason.

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

I designed the architecture and the constraints and made the decisions, while Claude planned and executed within them. The decisions are recorded in docs/decisions.md, and the rules Claude worked under are in CLAUDE.md.

Claude wrote most of this code. I've since reviewed the load-bearing paths and the tests, with particular attention to the sensitive bits: the install script, the uninstall path, and the container invocation. The rest is verified by process — the test suite, the code and comment budgets enforced in the Makefile, and audits by AI systems other than the one that wrote the code.

To say it plainly: this is a small, zero-stakes project where keeping iterations fast while manufacturing the rigor was the point. Learning to use these tools can still produce something useful, and that is where we find ourselves.

A longer account of how this was built is here.
A longer account of how this was built is
[here](https://bspeelm.github.io/bothy/how-it-was-built.html).

## Credits and licence

MIT — see [`LICENSE`](LICENSE).

The built-in palette is [Dracula](https://github.com/dracula/dracula-theme),
also MIT. bothy does not bundle the tools it installs. It downloads their
official releases, checks them, and each keeps its own licence.
[`NOTICE`](NOTICE) lists them all, as is only right.
