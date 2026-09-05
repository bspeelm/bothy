# Toolboxes

A **box** is a toolbox or distrobox container: your home directory, your files,
a different set of installed packages. Fedora Silverblue makes them the normal
place to install things, and it is common to end up with several — one per kind
of work.

bothy makes boxes easier to manage from a session. It does not replace
`toolbox`: toolbox still creates the containers and still enters them. What
bothy adds is keeping track of **which project belongs in which box**, and
showing you which sessions are running in each.

If you have neither toolbox nor distrobox installed, none of this applies.
bothy works the same as it does on macOS, where there are no boxes at all.

## Which box a project opens in

Four rules, in order. The first that answers wins:

| | rule | where it comes from |
|---|---|---|
| 1 | `workspace.container` | `bothy config set workspace.container <name>` — explicit, and applies everywhere |
| 2 | the box bothy is running in | if you launched from inside a box, that is the answer |
| 3 | this project's recorded box | what you answered the first time you opened it |
| 4 | where bothy installed its tools | the last resort, when nothing above has an answer |

`bothy box` tells you which one answered, and names the project's own record
when a rule above it is the one answering — otherwise a recorded box looks
ignored rather than waiting its turn:

```
$ bothy box
~/code/api
  box       dev
  because   this project is recorded for it
```

Rule 4 is a poor answer to the question, and it is last for that reason. It
knows where bothy installed its own tools, which is not the same as where a
project belongs. It is still better than nothing: your home directory is shared
between the host and its boxes, but `PATH` is not, so a bothy installed inside
a box has tools the host cannot see.

Rule 1 applies to every project at once, so it is a blunt instrument for a
single one — but when it is set, it wins, and rules 2 to 4 never run. Clear it
with `bothy config set workspace.container ""`.

## Being asked, once

The first time you open a project on a machine with more than one box, bothy
asks:

```
~/code/api has not been opened before. which toolbox should it use?
  0) the host
  1) dev                    running
  2) docs                   exited
  3) rust                   exited
choose [dev]:
```

The default is what bothy would have done anyway, so Enter changes nothing.
"The host" is a real answer and is remembered as firmly as a box name — you are
not asked again for having chosen it.

It stays quiet when there is nothing worth asking: bothy is already inside a
box, `workspace.container` is set, you have answered for this project before,
there is no terminal to answer on, or you have fewer than two boxes. In all of
those cases nothing is recorded either.

## `bothy box ls`

```
$ bothy box ls
* dev                      running   bothy-api
  docs                     exited
  legacy                   running   bothy-legacy
  rust                     exited
  (the host)                         bothy-notes
```

The star marks this project's box. The sessions listed are where they actually
are: bothy looks at the running processes rather than trusting its own record,
so if a session ended up somewhere unexpected, you will see it there.

## Moving a project

```
$ bothy box use rust
bothy-api is running and has to end before this project can move to rust.
end it? [y/N] y
ended bothy-api
~/code/api now opens in rust
bothy resolved its tools in dev, so some may not exist in rust.
      run 'bothy doctor' there to find out
```

The session has to end because the multiplexer server runs **inside** the
container. There is no carrying a running session across; reopening brings the
layout back, but not the scrollback.

`--yes` answers for you, before or after the box name. Without a terminal and
without `--yes` it refuses — the opposite of every other prompt in bothy, and
deliberate: refusing a download costs a download, refusing this costs a running
session.

Run from a pane of the session it is moving, the workspace comes back in a new
window: the one you typed it in is torn down with the session.

`bothy box use host` moves a project out of every box. That is a real answer
and is remembered as one.

The note about tools is reported, never checked: finding out for certain would
mean entering the box, and entering starts it. `bothy doctor`, run in the box,
is the thing that actually knows.

## Stopping a box

```
$ bothy box stop docs
stopped docs — its packages are still there, 'toolbox run' starts it again
```

Stopping is not removing. Everything installed in the box survives, and the
next `toolbox run` starts it again. To delete a box, use `bothy box rm`.

A box with a live session in it is refused, and the refusal names the session:

```
$ bothy box stop dev
bothy: dev is in use by bothy-api
      end it with 'bothy kill bothy-api' first
```

Only a running session stops you. A project that is merely *recorded* for the
box does not, so an old project you have long since deleted will never block
you from stopping a box.

## Making a box

```
$ bothy box create scratch
...toolbox creates it...
~/code/api now opens in scratch
install bothy's tools in it now? [Y/n]
```

bothy passes the name to `toolbox create` and gets out of the way. It adds the
two steps that come after: this project now uses the new box, and you can
install bothy's tools there straight away instead of finding out they are
missing when the first workspace fails.

`create` and `rm` are the two box commands that need toolbox installed. bothy
manages boxes; it does not make or unmake them itself.

## Removing a box

```
$ bothy box rm scratch
removing scratch deletes the container and everything installed in it.
remove it? [y/N] y
removed scratch
~/code/api now opens in dev — where bothy installed its tools
```

`toolbox rm` does the deletion. bothy adds the last line: any project that was
using the box is told where it opens now, so nothing is left pointing at a box
that no longer exists.

`rm` is stricter than `stop`, because you cannot undo it. A box that is merely
running is refused rather than stopped for you, and bothy never passes
`--force` to toolbox, so you cannot delete a box with work in it by accident.
Stop it first, then remove it.

You can remove any box, not just ones bothy created.

## Why bothy talks to podman

Entering a box stays `toolbox`'s job. The `podman exec` command toolbox builds
carries about twenty environment variables, a user, a working directory and a
capability wrapper — writing a second version of that would only drift out of
step with the first.

Everything else goes to podman, because toolbox gives no way to do it:

- `toolbox list` has no machine-readable output.
- There is no `toolbox stop`.
- **`toolbox run` starts the container it is asked about**, so inspecting a box
  with it would start every box being inspected.

That is also what makes distrobox work: distrobox containers are podman
containers too, and differ only by a label.

bothy never installs podman. Rootless podman needs setuid helpers, root-owned
`/etc/subuid` and a container runtime — on an immutable distro that is
`rpm-ostree` and a reboot, which bothy does not do to anyone's machine. It uses
the podman that is already on the host, reaching it through `flatpak-spawn`
when bothy itself is inside a box.

## Tools that exist in one box but not another

When bothy installs, it reuses a tool you already have if yours is good enough,
and records where it found it — `/usr/bin/jq`, say. That path exists on the
host and in some boxes but not others, so a pane that needs it can die with
"command not found".

`bothy doctor` reports it: it checks every recorded tool from wherever it is
run and names the container in the fix. If you move a project to a box that has
never had `bothy install` run in it, run the doctor there first.

## A box is not the agent's wall

`bothy confine` uses a container too, but for a different purpose. It is a
plain podman container built from bothy's own Containerfile, and it mounts your
project and nothing else from `$HOME`. It carries no toolbox label and is
deleted when it exits, so it never shows up in `bothy box ls` and no box
command can touch it. See [Walling off the agent](Walling-off-the-agent).

A confined agent does open in this project's box, because `bothy confine`
launches the same way `bothy` does.

## Commands

| | |
|---|---|
| `bothy box` | which box this project uses, and which rule chose it |
| `bothy box ls` | every box, whether it is running, and the sessions in it |
| `bothy box use <name>` | move this project to another box (`host` for none) |
| `bothy box stop <name>` | stop a box nothing is using |
| `bothy box create <name>` | make one, use it for this project, offer to install the tools |
| `bothy box rm <name>` | delete one, and say where its projects open now |

[All commands](Commands) · [Walling off the agent](Walling-off-the-agent)
