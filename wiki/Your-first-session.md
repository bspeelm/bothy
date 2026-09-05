# Your first session

You have installed bothy. This is what happens next and what to do in the room.

## Open it

```sh
cd ~/some/project
bothy
```

The first run is slower than the rest: bothy looks at the machine, lists what
is missing, asks before downloading anything, writes its configs, then opens
the window. Afterwards it is just the window.

If your terminal cannot draw images, bothy opens a **new Ghostty window**
rather than running where you are — image previews are the reason. Force it
either way with `--in-place` or `--window`.

## The room

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

**The agent pane** is where your AI tool runs — `claude` unless you configured
otherwise. It starts focused, because it is usually what you came for. If you
have no agent installed the pane sits empty and the doctor says so.

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

**`Ctrl-q` is the other thing.** It ends the session rather than leaving it.
There is no undo.

## When something looks wrong

```sh
bothy doctor
```

Twenty-nine checks, each with the command that fixes it. Start there before
anything else — see [The doctor](The-doctor) for how to read it, and
[Troubleshooting](Troubleshooting) for the failures people actually hit.

## Making it yours

- A different layout: `bothy config set profile editor` — see [Profiles](Profiles).
- A different tool in a slot, or your own config for one:
  [Swapping parts](Swapping-parts-and-theming).
- Walling the agent off from the rest of `$HOME`:
  [Walling off the agent](Walling-off-the-agent).

Settings live in `~/.config/bothy/config.toml`; `bothy config edit` opens it.
After changing anything, `bothy install` writes the configs again and checks
the result.
