# Troubleshooting

**Run `bothy doctor` first.** Twenty-eight checks, each carrying the command
that fixes it — a test fails if any check reports a failure without one. This
page is the handful people actually hit, by symptom.

## `bothy: command not found`

`~/.local/bin` is not on your `PATH`. The install script says so and prints the
line; if you missed it:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Put it in your shell's startup file. bothy will not do that for you — it does
not edit your dotfiles.

## Previews are block art, not pictures

Your terminal cannot draw inline images. That is a fact about the terminal, not
a misconfiguration, which is why the doctor reports it as a capability you do
not have rather than as a failure.

Ghostty, Kitty and WezTerm can. In anything else bothy falls back to an
approximation made of characters. If you have Ghostty installed, bothy opens a
new window in it by default; `--in-place` overrules that.

## The agent pane is empty

Nothing to run. bothy launches `claude` unless told otherwise and never
installs an agent — your AI tools and their credentials are your business
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

Name the slot, not the program. To adjust bothy's rather than replace it, drop
a file in `~/.config/bothy/overrides/<tool>/<file>` — it is appended, so yours
wins. See [Swapping parts](Swapping-parts-and-theming).

## Yazi behaves oddly, or keys do nothing

Usually an old Yazi from a distro package — they are often years behind, and
bothy's config uses current key names. `bothy doctor` checks the version and
the key names separately, and `bothy install` fetches a pinned one into bothy's
own tree without touching the system copy.

## `bothy confine` fails

It needs rootless podman, and an image you build once. `bothy confine` with no
image prints the two commands. Inside a Toolbx, podman is on the *host* —
bothy handles that itself, but your own `podman build` needs
`flatpak-spawn --host`. See [Walling off the agent](Walling-off-the-agent).

With no podman at all it fails and says so; it never silently runs unconfined.

## macOS refuses to run it

Gatekeeper, because bothy is unsigned. `xattr -dr com.apple.quarantine "$(which
bothy)"` clears it, and the doctor prints that with your path filled in. The
Homebrew cask does it during install. See [Installing](Installing#macos-gatekeeper).

## Something else

`bothy doctor --json` is the thing to paste into an issue — it is what the bug
template asks for, and it carries every check, its severity and its fix.
Report at [github.com/bspeelm/bothy/issues](https://github.com/bspeelm/bothy/issues);
for anything security-shaped, [SECURITY.md](https://github.com/bspeelm/bothy/blob/main/SECURITY.md) instead.
