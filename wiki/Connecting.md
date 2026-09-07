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

## What "it works" means

**If `ssh <host>` works from your terminal, `bothy connect <host>` works.** That
is the whole test. bothy does not manage keys, users, ports or jump hosts — ssh
does, and bothy hands your host string to it untouched.

So the two things that catch people out are the two that would catch `ssh` out:

**The account.** `bothy connect 192.168.30.93` logs in as *your local
username*, because that is what ssh does with a bare address. On someone else's
machine that is usually wrong. Name it instead:

```sh
bothy connect bryan@192.168.30.93
```

bothy prints the account before it connects, so you can see which one it is
about to use.

**Being reachable.** If the address is wrong, the machine is off, or a firewall
drops the connection, bothy gives up after ten seconds and says so rather than
sitting there.

The tidier fix for a machine you use often is an entry in `~/.ssh/config`:

```
Host abbey
    HostName 192.168.30.93
    User bryan
    IdentityFile ~/.ssh/id_abbey
```

Then `ssh abbey` works, and so does `bothy connect abbey` — no account, no
address, no key to remember.

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

## What the agent is told

The agent is handed a short note when the workspace opens, because otherwise it
has to work out where it is by experiment — and gets it wrong. It says which
machine is mounted, that the agent is **not** running on it, that paths under
the working directory belong to that machine while `/etc` and `/home` are
yours, and that `on` is how to run something over there.

Agents differ in how they take a note, so each one declares it in its own
provider file. An agent that declares nothing is started plainly rather than
with a flag bothy guessed at.

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

**Use a key rather than a password** — not because a password is unsafe, but
because there is nobody to type it. One workspace opens several SSH
connections: the mount, the shell pane, and a fresh one for every `on`. A host
that wants a password asks for it each time, and the agent running `on make
test` cannot answer, so the pane simply waits.

bothy does not refuse a password. ssh decides how you authenticate and bothy
does not overrule it — the same reason it never turns password authentication
on. It just cannot answer the prompt for you.

```sh
ssh-copy-id abbey
```

A key with a passphrase has the same problem for the same reason, and the same
answer ssh already gives: load it into your agent once with `ssh-add`, and
every connection after that is unattended.

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

**Confine and connect do not combine, and bothy says so.** Run `bothy confine`
in a connected workspace and it refuses, because the wall could not reach the
files. The container is started by podman, and an sshfs mount made where bothy
runs is invisible to it — measured: twenty-four entries inside the toolbox,
none from the host. The bind would succeed and mount an *empty* directory, so
the agent would start walled off from the very files you opened it for. A wall
that hides the project is worse than none.

**macOS is untested by CI**, though nothing about bothy prevents it. sshfs
there needs a FUSE layer, and the one to use is **FUSE-T** rather than macFUSE:
macFUSE needs a kernel extension, which on Apple Silicon means booting into
reduced security to allow it, and Homebrew core dropped `sshfs` after macFUSE's
licence changed. FUSE-T avoids all of that by serving FUSE over a local
loopback instead of a kext.

```sh
brew tap macos-fuse-t/homebrew-cask
brew install --cask fuse-t-sshfs
```

The sshfs cask depends on `fuse-t`, so that one command brings both, and it
installs the binary as `sshfs` — which is the name `bothy connect` looks for.

[All commands](Commands) · [Where it runs](Where-it-runs)
