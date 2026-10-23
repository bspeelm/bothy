# Troubleshooting

**Run `bothy doctor` first.** Every failing check carries the command that
fixes it; a test fails if any check reports a failure without one. This page
covers the common failures, by symptom.

## `bothy: command not found`

`~/.local/bin` is not on your `PATH`. The install script says so and prints the
line; if you missed it:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Put it in your shell's startup file. bothy will not do that for you — it does
not edit your dotfiles.

## There are no image previews

There are none on any terminal, and the doctor says so rather than reporting a
failure. The file pane shows `(press Enter to preview)` on an image, and Enter
opens it in whatever your system opens images with.

The cause is upstream and measurable: Yazi picks its image protocol from what
the terminal says it is, zellij answers that question in its own name, and the
protocol Yazi then sends is one zellij renders as garbage. Ghostty and Kitty
draw images perfectly with nothing in the way; through a multiplexer nothing
does. If Yazi or zellij changes that, bothy turns previews back on by itself —
it asks on every launch.

## The agent pane is empty

There is nothing to run. bothy launches `claude` unless configured otherwise
and never installs an agent, because AI tools and their credentials are yours
to manage
([ADR-014](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-014--bothy-installs-no-editor-and-helix-could-not-be-installed-anyway)).

```sh
bothy config set slots.agent none        # or the name of one you have
```

`bothy doctor` names the command to install the agent it expected.

## `bothy attach` cannot find the session

Sessions are named after the project directory. If you started the workspace
some other way — plain `zellij`, or a differently-named session — attach has
nothing to match. `bothy ls` shows what is actually running, and the doctor's
`session-named` check reports when the running session is not one attach could
find.

## My own Yazi config is being ignored

That is the default: bothy points Yazi at its own config for one session so
your setup is untouched. To use yours instead:

```toml
# ~/.config/bothy/config.toml
passthrough = ["browser"]
```

Name the slot, not the program. See
[Swapping parts](Swapping-parts-and-theming#using-your-own-tool-config-instead),
which also covers adjusting bothy's config rather than replacing it.

## Yazi behaves oddly, or keys do nothing

Usually an old Yazi from a distro package — they are often years behind, and
bothy's config uses current key names. `bothy doctor` checks the version and
the key names separately, and `bothy install` fetches a pinned one into bothy's
own tree without touching the system copy.

## My project opened in the wrong toolbox

`bothy box` says which box it used and why:

```
$ bothy box
~/code/api
  box       dev
  because   bothy installed its tools there
```

`bothy box use <name>` sets the right one for this project and remembers it.
The reason names which rule answered, and
[Toolboxes](Toolboxes#which-box-a-project-opens-in) says what each one means.

If a pane dies with "command not found" after moving, run `bothy doctor` inside
the new box: tools bothy reused from the system exist in some boxes and not
others.

## `bothy confine` fails

It needs rootless podman, and an image you build once. `bothy confine` with no
image prints the two commands. Inside a Toolbx your own `podman build` needs a
hop to the host, which
[Walling off the agent](Walling-off-the-agent#inside-a-toolbox) spells out.

With no podman at all it fails and says so; it never silently runs unconfined.

## Windows: the file pane is empty, or every file says it has no type

Two things Windows does not ship and Yazi needs. If the pane is blank the
moment it opens, the Microsoft C runtime is missing — Yazi's build links it,
and without it Yazi exits immediately with nothing on screen:

```
winget install Microsoft.VCRedist.2015+.x64
```

If the pane opens but every entry reads "Cannot find `file` to detect the
file's MIME type", install [Git for Windows](https://git-scm.com/download/win):
Yazi works out what a file is by shelling out to the `file.exe` it ships, and
has no fallback. `bothy doctor` names both before you hit either.

## macOS refuses to run it

Gatekeeper, because bothy is unsigned. `xattr -dr com.apple.quarantine "$(which
bothy)"` clears it, and the doctor prints that with your path filled in. See
[Installing](Installing#macos-gatekeeper).

## Something else

`bothy doctor --json` is what to attach to an issue: the bug template asks for
it, and it carries every check with its severity and its fix.
Report at [github.com/bspeelm/bothy/issues](https://github.com/bspeelm/bothy/issues);
for a suspected vulnerability, [SECURITY.md](https://github.com/bspeelm/bothy/blob/main/SECURITY.md) instead.
