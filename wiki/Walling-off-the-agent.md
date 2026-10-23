# Confining the agent

`bothy confine` runs the agent pane in a rootless podman container. What it
prevents, and what it deliberately allows, is in
[Security](Security#confining-the-agent); this page covers the setup.

## Three commands, once

The first prints the other two:

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

podman build -t bothy-agent ~/.local/share/bothy/confine   # ~550 MB, a few minutes
bothy confine
```

**bothy writes the recipe and never builds it.** Building installs an agent,
and bothy does not install agents: their installation methods change, they
require credentials bothy should not handle, and one arriving unrequested is
more than a workspace tool should do ([ADR-034](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-034--the-agent-can-be-walled-off-on-request-and-never-by-default)).

The Containerfile is yours once written: change the agent, pin a version, add
tools. bothy does not overwrite it. `bothy confine --print` shows the recipe
and the exact `podman run`, so the boundary can be read before it is trusted.

## Inside a toolbox

The common case on Silverblue, with one complication: **podman is on the host,
not in the toolbox.** bothy handles that itself, reaching the host through
`flatpak-spawn` the same way it opens files, so `bothy confine` needs nothing
extra.

Your own podman commands are the part that does not:

```sh
podman build -t bothy-agent ~/.local/share/bothy/confine
# bash: podman: command not found

flatpak-spawn --host podman build -t bothy-agent ~/.local/share/bothy/confine
```

`bothy confine --print` prints the invocation with the hop already in it, so
you can see which case you are in.

## Configuration

One key, optional:

```toml
[agent]
image = "bothy-agent"    # the image the agent runs in; this is the default
```

There is deliberately no `confine = true`. A default that changed how the agent
runs would break for people who never asked for it and could not tell why.

## Removing it

The Containerfile lives inside bothy's tree, so `bothy uninstall` takes it. The
image is not bothy's to delete — it is ~550 MB bothy did not build:

```sh
podman rmi bothy-agent
```

Uninstall names the image rather than leaving you to find it.

## Where it works

Tested on Linux with podman. On macOS podman runs a Linux VM: it works, the
boundary is real, and its edges differ — bothy reports that rather than
implying equivalence. With no podman installed, `bothy confine` fails and says
so; it never runs unconfined without reporting it.
