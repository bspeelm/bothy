# Commands

Fifteen, and you need three of them. `bothy` opens the workspace, `bothy
doctor` says what is wrong, `bothy config set` changes a setting. The rest are
for a specific afternoon.

`bothy --help` prints the same list; this page says what each one actually does.

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

**Closing the window does not strand the project.** Inside a container the
multiplexer client would otherwise outlive the terminal that opened it —
`podman exec` ignores the hangup — and a session with a client on it is
refused, which used to mean the project could not be opened again. bothy ends
its client when the window closes. If one is left behind anyway, by a crash or
a version older than this, the next launch ends it and says so:

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

Which sessions are running, marking the one you are in — and which have stopped
but are still kept:

```
  bothy-api                  the one you are in
  bothy-server_setup

2 stopped, kept so they can be resurrected:
  polite-galaxy
  bothy-notes
Clear them with 'bothy ls --prune'.
```

A stopped session is not junk: attaching brings its layout back as it was. But
nothing removes them either, so they accumulate. `--prune` deletes the stopped
ones and refuses anything still running.

### `bothy kill [session]`

Ends a session without attaching to it. With no name, this directory's session.

```
$ bothy kill bothy-server_setup
ended bothy-server_setup
```

Nothing is left behind — the same end state as pressing `Ctrl-q` inside it. It
refuses the session you are currently in, because that is what `Ctrl-q` is for,
and refuses one that has already stopped, because that is what
`bothy ls --prune` is for.

### `bothy box [ls|use|stop|create]`

Which toolbox this project opens in, and why:

```
$ bothy box
~/code/api runs in dev — chosen for this project
```

The reason matters as much as the name: four rules can decide it, and the one
that answered is the one to change. `bothy box ls` shows every box on the
machine and the sessions really in each, marking this project's:

```
* dev                      running   bothy-api
  docs                     exited
  legacy                   running   bothy-legacy
  rust                     exited
```

Sessions are read from the process table, not from bothy's record, so a session
somewhere unexpected is listed where it actually is.

`bothy box use <name>` moves this project to another box, and `bothy box use
host` moves it out of every box. The session has to end first — the multiplexer
server runs *inside* the container, so there is no carrying a running one
across — so it says what will end and asks:

```
$ bothy box use rust
bothy-api is running and has to end before this project can move to rust.
end it? [y/N]
```

`--yes` answers for you; without a terminal and without `--yes` it refuses,
which is the opposite of every other prompt in bothy and deliberate: refusing a
download costs a download, and refusing this costs a running session.

`bothy box stop <name>` stops a box nothing is using. It **stops** it — the
container and everything installed in it are still there, and the next
`toolbox run` starts it again. It refuses a box with a live session in it and
names the session. This is a `podman stop` under a nicer name: toolbox has no
stop of its own, and from inside a box you have no podman either.

`bothy box create <name>` hands the work to `toolbox create` and then does the
two things toolbox cannot — record that this project belongs in the new box,
and offer to install bothy's tools inside it, which is where the missing-tool
trap starts.

[Toolboxes](Toolboxes) has the rules, the first-run prompt, and what happens on
a machine with no toolboxes.

### `bothy keys`

The bindings worth knowing on a first day. They are Zellij's, not bothy's —
bothy leaves them alone.

## Finding out what is wrong

### `bothy doctor [--json]`

Twenty-eight checks against the workspace, each with a fix. This is the command
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

### `bothy outdated [--json]`

Which pinned tools have newer releases upstream. Reports; it does not upgrade.
The pins live in [`bothy.lock`](https://github.com/bspeelm/bothy/blob/main/bothy.lock).

### `bothy layout [--profile P]`

Prints the Zellij layout bothy would launch, generated from the profile. For
when you doubt what it is about to do.

### `bothy version`

The version, and whether it is a release or a source build.

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

### `bothy desktop-entry [--install]`

Prints a `.desktop` launcher that opens the workspace in a directory.
`--install` writes it; `--remove` deletes it. It lands outside bothy's tree by
necessity, so `bothy uninstall` names it rather than removing it.

## The rest

### `bothy confine`

Runs the agent pane in a rootless podman container, with the project directory
and the agent's credentials mounted and nothing else from `$HOME`. Opt-in;
there is no setting that turns it on. See
[Walling off the agent](Walling-off-the-agent) — including what it deliberately
does not stop.

### `bothy upgrade`

Works out how this copy was installed and prints the right command for it —
`brew`, `dnf`, `apt`, `go install`, the script, or a source build. It prints;
it does not upgrade.

### `bothy uninstall [--dry-run]`

Removes bothy's tree and the binary, and names the three things it leaves: your
settings, the container image if you confined the agent, and the desktop entry
if you added one. `--dry-run` shows what would go.
