# Toolboxes

A **box** is a toolbox or distrobox container: your home directory, your files,
a different set of installed packages. Fedora Silverblue makes them the normal
place to install things, and it is common to end up with several — one per kind
of work.

bothy does not make boxes and cannot work without something that does.
`toolbox create` makes them, `toolbox run` enters them, and everything inside
one is toolbox's doing. What bothy adds is the thing toolbox has no concept of:
**which project belongs in which box**, and which sessions are in each.

On a machine with no toolbox and no distrobox there are no boxes, no prompt and
no hop. Nothing on this page happens, and bothy behaves exactly as it does on
macOS.

## Which box a project opens in

Four rules, in order. The first that answers wins:

| | rule | where it comes from |
|---|---|---|
| 1 | `workspace.container` | `bothy config set workspace.container <name>` — explicit, and applies everywhere |
| 2 | the box bothy is running in | if you launched from inside a box, that is the answer |
| 3 | this project's recorded box | what you answered the first time you opened it |
| 4 | where bothy installed its tools | the fallback, and the reason this page exists |

`bothy box` tells you which one answered:

```
$ bothy box
~/code/api runs in dev — chosen for this project
```

Rule 4 is a guess, and a bad one. It records where bothy *resolved its tools*,
which is a different question from where a project lives; before rule 3 existed
it was the only fallback there was, so every project on a machine went to
whichever box bothy happened to be installed in. It is kept because it is
better than no answer at all: home is shared between a host and its boxes but
`PATH` is not, so a bothy installed inside a box has tools the host cannot see.

**Rule 1 stays on top.** It is global, so it is the wrong tool for one project
— but bothy's own error messages tell you to reach for it, and demoting it
would make that advice a lie. If it is set, rules 2–4 never run.

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

It stays quiet when there is nothing to ask: bothy is already inside a box,
`workspace.container` is set, the project has answered before, there is no
terminal to answer on, or there are fewer than two boxes. In every one of those
cases **nothing is recorded**, so a machine with one box behaves exactly as it
did before this existed.

## `bothy box ls`

```
$ bothy box ls
* dev                      running   bothy-api
  docs                     exited
  legacy                   running   bothy-legacy
  rust                     exited
  (the host)                         bothy-notes
```

The star is this project's box. The sessions are read from the process table
and not from bothy's record, so this reports where sessions **are** — a session
in a box bothy did not expect is listed in the box it is in, which is how you
find out the two disagree.

## Why bothy talks to podman

Entering a box is `toolbox`'s job and stays that way: the `podman exec` toolbox
builds carries about twenty environment variables, a user, a working directory
and a capability wrapper, and a second implementation of that would drift.

Everything else goes to podman, because toolbox offers no way to do it:

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

## The trap worth knowing

Tools resolved in one box may not exist in another. bothy reuses a system tool
when yours is good enough, and records the path it found — `/usr/bin/jq`, say.
That path is real on the host and in some boxes and missing in others, and a
pane that needs it dies with "command not found".

`bothy doctor` reports it: it checks every recorded tool from wherever it is
run and names the container in the fix. If you move a project to a box that has
never had `bothy install` run in it, run the doctor there first.

## A box is not the agent's wall

`bothy confine` also uses a container, and it is a different thing for a
different purpose: a plain podman container built from bothy's own
Containerfile, with your project mounted and the rest of `$HOME` not. It has no
toolbox label and is removed when it exits, so it never appears in `bothy box
ls` and no box command can touch it. See [Walling off the
agent](Walling-off-the-agent).

A confined agent does open in this project's box, because `bothy confine` goes
through the same launch as `bothy`.

## Commands

| | |
|---|---|
| `bothy box` | which box this project uses, and which rule chose it |
| `bothy box ls` | every box, its state, and the sessions really in each |

[All commands](Commands) · [Walling off the agent](Walling-off-the-agent)
