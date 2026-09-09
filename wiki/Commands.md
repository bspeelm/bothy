# Commands

One entry per command: what it does, and its flags. Anything with a page of its
own is linked from its entry rather than explained twice.

`bothy` opens the workspace, `bothy tower` watches every agent at once, and
`bothy doctor` says what is wrong. The rest are for a specific afternoon.
`bothy --help` prints the same list.

## Opening the workspace

### `bothy`

Opens the workspace in the current directory: file browser across the top, the
agent and a shell below. First run fills any gaps first — it lists what is
missing, asks before downloading, then opens.

| flag | |
|---|---|
| `--dir <path>` | open somewhere other than the current directory |
| `--profile <name>` | use a layout profile other than the configured one |
| `--window` | always open a new Ghostty window |
| `--in-place` | always run in the terminal you are already in |

**Closing the window ends the session.** That is what closing it usually means,
and a session left running is one you cannot tell from a session you are using.
What is written down is untouched: the agent keeps its own transcript, so its
conversation comes back with that agent's own resume command. What is lost is
whatever turn was in flight.

Inside a container the multiplexer client would otherwise outlive the terminal
that opened it — `podman exec` ignores the hangup — and a session with a client
on it is refused, which used to mean the project could not be opened again. If
one is left behind anyway, by a crash or a kill, the next launch ends it and
says so:

```
bothy: reclaimed bothy-work from a closed window
```

A session someone is actually looking at is still refused; see [ADR-042]
(https://github.com/bspeelm/bothy/blob/main/docs/decisions.md) for how it tells
the two apart.

`--window` and `--in-place` override bothy's own judgement for one run. It
normally decides by asking whether your terminal can draw images. To make the
override permanent, `bothy config set workspace.launch here` (or `window`); the
flags still win for a single run.

### `bothy attach [session]`

Reattach to this project's session, which has gone on without you. With no
argument it picks the session for the current directory.

### `bothy ls [--prune]`

Which sessions are running, marking the one you are in and the ones nothing is
looking at — and which have stopped but are still kept:

```
  bothy-api                  the one you are in
  bothy-notes                detached

1 stopped, kept so it can be resurrected:
  polite-galaxy
Clear them with 'bothy ls --prune'.
```

**detached** means the session is running with no window on it. That happens
when you detach with `Ctrl-o d`, which is the point of detaching. Closing a
window ends its session instead, so it will not appear here at all. Nothing is
said when the multiplexer will not answer: "could not ask" is not "nobody is
looking".

A **stopped** session is one whose server is gone but whose layout zellij kept —
after a reboot or a crash, mostly, since the ordinary ways of ending a session
remove it outright. Attaching brings the layout back, though not quite as it
was: commands come back suspended behind "Waiting to run", and a profile changed
since then is ignored. Nothing removes them on their own, so they accumulate.
`--prune` deletes the stopped ones and refuses anything still running.

### `bothy kill [session]`

Ends a session without attaching to it. With no name, this directory's session.

```
$ bothy kill bothy-notes
ended bothy-notes
```

Nothing is left behind — the same end state as pressing `Ctrl-q` inside it. It
refuses the session you are currently in, because that is what `Ctrl-q` is for,
and refuses one that has already stopped, because that is what
`bothy ls --prune` is for.

### `bothy keys`

The bindings worth knowing on a first day. They are Zellij's, not bothy's —
bothy leaves them alone.

## Watching several at once

### `bothy tower [--mirror session] [--every duration]`

One window showing every running agent, so several sessions can be watched from
one place. Each row mirrors one session's agent pane; type into a mirror and the
line goes to that agent. The tower reads panes and sends nothing of its own.

| flag | |
|---|---|
| `--mirror <session>` | watch one session and nothing else |
| `--every <duration>` | the refresh interval |
| `--no-expand` | leave your panes as they are, at the cost of thin mirrors |
| `--restore` | collapse agent panes a closed terminal left expanded |

[The tower](The-tower) — reading a mirror, answering an agent, and why it
expands the panes it watches.

## Where it opens

### `bothy box [ls|use|stop|create|rm]`

Which toolbox this project opens in, and why:

```
$ bothy box
~/code/api
  box       dev
  because   this project is recorded for it
```

The reason matters as much as the name: several rules can decide it, and the one
that answered is the one to change.

| | |
|---|---|
| `bothy box` | which box this project uses, and which rule chose it |
| `bothy box ls` | every box, whether it is running, and the sessions in it |
| `bothy box use <name>` | move this project to another box (`host` for none) |
| `bothy box stop <name>` | stop a box nothing is using |
| `bothy box create <name>` | make one, use it, offer to install the tools |
| `bothy box rm <name>` | delete one, and say where its projects open now |

Sessions are read from the process table, not from bothy's record, so a session
somewhere unexpected is listed where it actually is. `use` and `rm` end a
running session first and say so; `--yes` answers for you.

[Toolboxes](Toolboxes#which-box-a-project-opens-in) has the rules, the
first-run prompt, and what happens on a machine with no toolboxes.

### `bothy connect [edit] <host>`

Opens the workspace against another machine. The file browser shows its files,
the shell pane is a login session on it, and the agent can reach it the same
way you can.

```
$ bothy connect <client>
directory on <client> [/]: /srv/api
ssh key, if that host needs one named [none]:
<client> mounted at ~/.local/share/bothy/cache/remotes/<client>
```

bothy puts nothing on the machine you connect to. It needs **sshfs on your own
machine** and refuses with the install command if it is missing. `--dir` sets
the directory without being asked; `bothy connect edit <client>` changes what was
remembered.

Inside the workspace, `on <command>` runs something on that machine in the
right directory — the agent needs it because the paths on the two machines
differ. [Connecting](Connecting) explains all of it.

### `bothy confine`

Runs the agent pane in a rootless podman container, with the project directory
and the agent's credentials mounted and nothing else from `$HOME`. Opt-in;
there is no setting that turns it on. See
[Walling off the agent](Walling-off-the-agent) — including what it deliberately
does not stop.

## Finding out what is wrong

### `bothy doctor [--json]`

Checks the workspace, each with a fix. This is the command
the project is built around: [The doctor](The-doctor) explains the output,
the severities and the capability grouping.

### `bothy tools`

Which tools are in use, which version, where each came from — bothy's own copy
or one already on your `PATH` — and, for the ones bothy fetched, where the
pinned checksum came from:

```
✓ fd        a faster find        10.5.0    supplied by bothy  pin: download
✓ ripgrep   a faster grep        15.2.0    supplied by bothy  pin: upstream
```

`pin: upstream` means the checksum in `bothy.lock` matched one the project
published, so the release cannot have been changed after publication.
`pin: download` means the project publishes no checksum, so the pin is the hash
of what bothy downloaded on the day it was pinned. Neither says the release
itself is good; see [Security](Security).

### `bothy layout [--profile P]`

Prints the Zellij layout bothy would launch, generated from the profile. For
when you doubt what it is about to do.

## Changing things

### `bothy config [get|set|edit|path]`

```sh
bothy config              # print the whole config
bothy config get <key>    # one value
bothy config set <key> <value>
bothy config edit         # open it in $EDITOR
bothy config path         # where the file is
```

`config.toml` carries `schema = 1`, which is bookkeeping rather than a setting —
`config set` refuses it. Unrecognised keys warn rather than fail, and `bothy
doctor` names a retired key's replacement.

### `bothy install [--dry-run]`

Writes the configs from your settings, then runs the doctor against the result.
Run it after changing a setting, or after replacing the binary. `--dry-run`
shows what it would write.

Every generated file says it is bothy's and names where to put your own changes.

### `bothy theme example`

Prints a blank eleven-token palette to fill in. Point bothy at the result with
`bothy config set theme.palette <path>`.

## Files outside bothy's tree

### `bothy completion <bash|zsh> [--install] [--remove]`

Prints the completion script for a shell. With `--install`, writes it where that
shell looks for it.

```
$ bothy completion bash --install
wrote ~/.local/share/bash-completion/completions/bothy
this is outside bothy's tree -- 'bothy uninstall' will not remove it,
but 'bothy completion bash --remove' will.
```

You need this only if bothy came from the install script, Homebrew or
`go install`. dnf, apt, pacman and the AUR place these files themselves.

[Installing](Installing#tab-completion) covers the zsh `fpath` line and why the
file sits outside bothy's tree.

### `bothy desktop-entry [--install]`

Prints a `.desktop` launcher that opens the workspace in a directory.
`--install` writes it; `--remove` deletes it. It lands outside bothy's tree by
necessity, so `bothy uninstall` names it rather than removing it.

## Upgrading and removing

### `bothy version`

The version, and whether it is a release or a source build.

### `bothy outdated [--json]`

Which pinned tools have newer releases upstream. Reports; it does not upgrade.
The pins live in [`bothy.lock`](https://github.com/bspeelm/bothy/blob/main/bothy.lock).

### `bothy upgrade`

Works out how this copy was installed and prints the right command for it —
`brew`, `dnf`, `apt`, `go install`, the script, or a source build. It prints;
it does not upgrade.

### `bothy uninstall [--dry-run]`

Removes bothy's tree and the binary, and names what it leaves rather than
leaving you to find them. `--dry-run` shows what would go.

[Installing](Installing#removing-it) lists what stays behind, and why.
