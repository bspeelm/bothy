# What happens when you type `bothy`

In order, with the reason for the order. This is the page to read when the
question is "why did it do that"; `docs/PLAN.md` is the architecture,
[`decisions.md`](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md) is why the architecture is that, and
[`adding-a-provider.md`](https://github.com/bspeelm/bothy/blob/main/docs/adding-a-provider.md) is how to extend it.

## 1. It looks at the machine

`platform.Detect` reads the operating system, the distribution and its
`ID_LIKE`, whether this is an image-based host, whether it is inside a
container and whether that container shares your home directory, and which
terminal emulator it is running in.

Detection happens once and is carried around as a value, so that a doctor
report and the install that produced it always agree about what machine they
were looking at. Everything Linux-shaped degrades to an empty value elsewhere
rather than failing.

Then `~/.config/bothy/config.toml` is read, if it exists. A key bothy does not
recognise is a warning and never a refusal, and so is a key whose value is the
wrong type — a config written by a newer bothy does not brick an older one.

## 2. It refuses to nest

If this shell is already inside an agent session, bothy stops. The layout starts
its own agent, so continuing would put a second one in a pane of the first.
Each agent says which environment variables it exports, in its own file under
`slots/`.

## 3. It decides where the workspace opens, before doing anything

Three questions, answered together and acted on afterwards:

**Which directory.** `--dir`, then `workspace.project_dir`, then the current
one. `~` is expanded.

**Which profile.** `--profile`, then `profile` in the config. A profile is a
short TOML file describing rows and panes.

**Whether this terminal will do.** Inline image previews need the Kitty
graphics protocol. If this terminal cannot draw them and Ghostty is available,
bothy opens a window that can — before any container hop, so the window opens
once and on the host. `workspace.launch` settles this standing; `--window` and
`--in-place` settle it for one run.

## 4. First run only: it fills the gaps

If bothy has no directory yet, it sets one up.

**Tools.** For each tool the configuration actually needs, bothy asks whether
the system's copy will do. One on `PATH` meeting the minimum version is used
as it is. Only a missing or too-old one is fetched, into bothy's own `bin/`,
pinned to a version and checksum in `bothy.lock` and verified before a byte is
written. bothy never touches the system's copy and never asks a package
manager for anything.

You are told what would be downloaded, and how much, before it is.

**Plugins, then config, in that order.** The generated config is written to
match what is actually installed, so the plugins have to be in place before the
templates render — a config referencing a plugin that is not there looks
correct and fails at launch.

**The configs.** Each provider filling a slot names the files bothy writes for
it, and where. Everything lands under `~/.local/share/bothy/config`. Nothing is
written to `~/.config/yazi`, `~/.vimrc`, or anywhere else of yours. Your own
additions go in `~/.config/bothy/overrides/` and are appended after the
generated content, so your setting wins.

## 5. It builds the layout

The profile is rendered into whatever the multiplexer takes — for Zellij, a KDL
layout file, written into bothy's tree and regenerated every launch, so editing
it does nothing. Each pane's slot becomes a command: the browser, the editor,
the agent.

## 6. It points the tools at bothy's tree, for one process tree

This is where isolation actually takes effect. The configs were written into
bothy's directory, and nothing reads them unless the tools are told to —
`ZELLIJ_CONFIG_DIR`, `YAZI_CONFIG_HOME`, `EDITOR`, `PATH` with bothy's `bin/`
first, and `XDG_CACHE_HOME` pointed inside the tree.

Every one of those is set for bothy's process tree and nothing else. Your shell
keeps its own. A slot you have passed through has its variable *unset* rather
than merely not set, because the session inherits this environment and an
already-exported value would otherwise survive into it.

## 7. It opens the workspace

A session named after the project directory, so `bothy attach` can pick it out
of several. If one is already running, bothy returns to it rather than building
a second. If one has stopped, it is replaced rather than resurrected.

## 8. Afterwards, it tells you what is wrong

`bothy doctor` runs at the end of an install and on demand. It reports the
failures that are otherwise silent — a config a tool discarded without saying
so, a theme that did not arrive, a plugin the config references and does not
have — and for each one, what to type.

It ends with what the stack can and cannot give you: panes, sessions, images,
theme, isolation.

## And when you remove it

`bothy uninstall` removes `~/.local/share/bothy` and the binary. It names what
it cannot remove for you: your settings in `~/.config/bothy`, the desktop entry
if you added one, and the container image if you built one for `bothy confine`.
