# Your first session

You have installed bothy. This is what happens on the first run, and how to
work in the window it opens.

## Open it

```sh
cd ~/some/project
bothy
```

The first run is slower than the rest: bothy inspects the machine, lists what
is missing, asks before downloading anything, writes its configuration, and
then opens the window. Later runs open the window directly.

If your terminal does not carry the Kitty keyboard protocol, bothy opens a
**new Ghostty window** rather than running where you are, so that the keys an
agent needs are not flattened on the way in. Force it either way with
`--in-place` or `--window`.

## The panes

The default profile is `cockpit`:

```
┌──────────────────────────────────────────────────┐
│           file browser — full width, half height │
├─────────────────────────────┬────────────────────┤
│      agent (focused)        │       shell        │
└─────────────────────────────┴────────────────────┘
```

**The file browser** is Yazi. Move with `hjkl`, `l` enters a directory or opens
a file, and the preview appears beside it.

**The agent pane** runs your AI tool — `claude` unless configured otherwise. It
starts focused, since it is the pane most sessions begin in. With no agent
installed the pane is empty and the doctor reports it.

**The shell** is a plain shell in the same directory. Git, builds, logs.

## Moving around

These are Zellij's bindings, and bothy leaves them alone:

| | |
|---|---|
| `Alt-h` `Alt-j` `Alt-k` `Alt-l` | move between panes |
| `Alt-n` | another pane |
| `Ctrl-o d` | **detach** — everything keeps running |
| `Ctrl-q` | quit, and the session is gone |
| `Ctrl-s e` | open the scrollback in your editor |

`bothy keys` prints this list any time.

## Leaving and coming back

`Ctrl-o d` detaches. The agent keeps working, the shell keeps its history, and
your terminal is yours again.

```sh
bothy attach     # back into this project's session
bothy ls         # which sessions are running
```

One session per project directory, named after it — which is how `attach` finds
the right one without being told.

**Closing the window ends the session.** Detaching is how to leave something
running. The agent's own conversation survives either way, since it keeps its
own transcript, but a turn in progress does not.

`Ctrl-q` also ends the session rather than detaching from it, and cannot be
undone.

## When something looks wrong

```sh
bothy doctor
```

Every failing check carries the command that fixes it. See
[The doctor](The-doctor) for how to read a report, and
[Troubleshooting](Troubleshooting) for the common failures by symptom.

## Changing it

- A different layout: `bothy config set profile editor` — see [Profiles](Profiles).
- A different tool in a slot, or your own config for one:
  [Swapping parts](Swapping-parts-and-theming).
- Walling the agent off from the rest of `$HOME`:
  [Walling off the agent](Walling-off-the-agent).

Settings live in `~/.config/bothy/config.toml`; `bothy config edit` opens it.
After changing anything, `bothy install` writes the configs again and checks
the result.
