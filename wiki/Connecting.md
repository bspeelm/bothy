# Connecting to another machine

`bothy connect abbey` opens a workspace against another machine. The file
browser shows that machine's files, the shell pane is a login session on it,
and the agent works on its files.

```sh
bothy connect abbey
```

**bothy puts nothing on the machine you connect to.** Not itself, not a tool,
not a credential, not a temporary file. The only thing it uses over there is
the SSH server already running, which is what connecting over SSH means. If
you can `ssh abbey` today, you can `bothy connect abbey`.

## What you need

sshfs, **on your own machine**. bothy does not install it, and `bothy connect`
tells you the command for your system if it is missing:

```
sshfs is not installed, and it is what lets the file browser
see abbey. bothy does not install it:
  sudo dnf install -y sshfs
```

On Fedora Silverblue, install it inside the toolbox bothy runs in rather than
layering it onto the host. The mount is visible to the workspace either way,
and that route costs no reboot.

Nothing else. The far machine needs no preparation at all.

## What runs where

This is worth understanding, because it explains everything else.

| pane | runs on | how it reaches the other machine |
|---|---|---|
| file browser | your machine | reads the sshfs mount |
| shell | **the other machine** | a real `ssh` login session |
| agent | your machine | it can `ssh` there, the same as you can |

The workspace itself — the multiplexer, the session, the window — is on your
machine. That is why `bothy ls`, `bothy attach` and `bothy kill` all keep
working normally, and why a dropped connection loses the mount but never the
session.

## Where the files are

The other machine's whole filesystem appears under one directory here:

```
~/.local/share/bothy/cache/remotes/abbey/srv/api     is     /srv/api on abbey
```

Strip the first part and you have the path abbey knows. That is worth
remembering, because the file browser shows you the long path and the shell
pane shows you the short one. They are the same file.

## Running things on the other machine

The agent and the shell both run commands, but in different places. The shell
pane is *on* abbey, so anything you type there runs there. The agent is on your
machine, so a command it runs happens here, with your toolchain, against files
fetched over the network.

When you want the agent to run something on abbey, there is a command for it:

```sh
on make test
```

`on` runs its argument on the machine you are connected to, in the directory
the workspace is open on. It exists because the paths differ — without it, an
agent that tries `ssh abbey "cd $(pwd) && make"` names a directory that does
not exist over there.

`on` is written into bothy's own directory on your machine when you connect.
Nothing is written on the other machine.

## Naming a machine

The first time you connect, bothy asks which directory to open and, if that
host needs a particular key, where it is. It remembers both, so afterwards
`bothy connect abbey` is enough. `bothy connect edit abbey` changes them.

**The default is `/`, the whole machine.** That is deliberate: a home
directory on a server usually holds nothing but dotfiles, so opening there
looks like a connection that did not work. The root always has something in
it, and anywhere below is a few keystrokes away in the browser. Type a path if
you know where you are going — `~` works too, and is expanded by asking that
machine rather than guessing with yours.

You can also write them down. Anything in `config.toml` wins over what bothy
learned, and the file is the one you already keep in git:

```toml
[remotes.prod]
host     = "10.0.0.5"
dir      = "/srv/api"
identity = "~/.ssh/id_prod"
```

Then `bothy connect prod` needs no prompt and no memory.

For a host that is already in your `~/.ssh/config`, bothy asks ssh what it
knows rather than asking you — aliases, users, ports and jump hosts all work
because ssh handles them, not bothy.

## Keys and trust

bothy chooses which key ssh should offer, when you tell it to, and does
nothing else to your SSH setup. It never disables host key checking, never
changes your known-hosts file, and never turns on password authentication.
Choosing a key is not the same as weakening verification, and bothy only does
the first.

Nothing is copied. The key stays where it is; bothy records the path.

## Sessions

The session lives on your machine, so it behaves like any other:

```sh
bothy ls                 # shows it, named for the host
bothy attach bothy-abbey-api
bothy kill bothy-abbey-api
```

The name carries the host so a project called `api` over there and one called
`api` here are two workspaces rather than one.

If the network drops, the mount goes stale and the session survives. Reconnect
with `bothy connect abbey` again.

## What to expect

**Browsing is fine. Searching from your side is not.** `rg` run locally walks
the whole tree over the network. Run it in the shell pane, or with `on`, and it
runs on the machine that has the files.

**Confine and connect do not combine.** `bothy confine` opens its own
workspace with the agent walled off, and `bothy connect` opens one against
another machine. They are two different launches, and there is no way to ask
for both at once today.

**macOS is untested.** sshfs there needs macFUSE, a kernel extension and a
reboot. Nothing about bothy prevents it; nobody has run it.

[All commands](Commands) · [Where it runs](Where-it-runs)
