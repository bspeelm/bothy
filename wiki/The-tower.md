# The tower

Several sessions open across several windows, each with an agent working, and no
way to see them at once. `bothy tower` puts every running agent in one window.

```
$ bothy tower
watching 3 agent(s), side by side; 2 pane(s) expanded to be worth reading
```

Three things are possible from there: read what each agent is doing, type a
reply to one, or take a session over and work in it directly.

## Reading the mirrors

Each row mirrors one session's agent pane, refreshed every two seconds. Move
between rows with `Alt+h/j/k/l`, the multiplexer's own pane navigation.

A session is shown when it is running and has a live agent pane. A session whose
agent has exited is skipped, because the pane outlives the agent and would show
a frozen screen.

An agent walled off with `bothy confine` is watched like any other. Its pane runs
podman rather than the agent, so it is found by the pane's name instead.

## Answering an agent

Type into a mirror and press Enter, and the line goes to the agent that mirror
is watching, as if you had typed it in that agent's own window. Move between
mirrors with `Alt+h/j/k/l` first, so the reply goes where you are looking.

The bottom two rows of a mirror are reserved for your reply. Nothing the mirror
draws reaches them, so a reply stays on screen while the agent above continues
working, and a reply can be composed while the agent is mid-task.

**A message can span lines.** End a line with a backslash and the next line
joins it; the message goes when a line does not end in one:

```
> the failing test is in tower_test.go \
> and the fixture it reads is list-panes.json \
> can you check the field names
```

A multi-line message arrives as one message rather than one per line, so the
agent sees it whole. To send a message that really ends in a backslash, type
two.

Shift+Enter cannot do this. The tower reads input the way a shell prompt does,
where the terminal ends the line on Enter, so no key means "insert a newline and
keep reading". The backslash is the convention a shell uses for the same
problem.

If a reply cannot be delivered — the session went away, or its agent pane did —
the mirror says so where you typed it, rather than letting the line vanish.

## Taking over a session

A mirror shows a pane's contents and can relay a line of text, which is not
enough for a prompt that needs a keystroke. When an agent asks for a numbered
choice, a y/n, or anything answered with the arrow keys, take the session over:

```
> /take
```

The pane stops mirroring and becomes a client of that session: the session
itself, with your keyboard attached. Arrows, Esc, Tab, Ctrl-C, the agent's own
bindings and its scrollback all work, because you are in it rather
than looking at it. The other mirrors carry on behind you.

`Ctrl-o d` — the way you leave any session — puts the pane back to mirroring.

| | |
|---|---|
| `/take` | hand this pane to the session it is watching |
| `Ctrl-o d` | give it back and return to mirroring |
| `//take` | send the word `/take` to the agent instead |

**`Ctrl-q` ends the session you are in**, exactly as it would in that session's
own window. While you are taken over the keys are the session's, so bothy cannot
catch that one for you. `Ctrl-o d` is the way out.

**On Windows the window is handed over whole.** Zellij there will not descend
into a session running inside a pane, so a take-over that nested would draw
two tab bars and `Ctrl-q` would end the tower. Instead the tower closes and
the session takes the window; leaving that session brings the tower back,
rebuilt ([ADR-061](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-061--the-take-over-hands-the-window-over-where-the-multiplexer-will-not-descend)).

### What it does to the windows

The pane is made full size before the session arrives and put back when you
leave, because a session running inside a pane takes its size once, on the way
in, and never asks again.

While you are in, that session has two windows — its own and this one — and it
is sized to fit both. If its own window is open elsewhere it may shrink there
for as long as you stay, and grow back when you leave. That is the cost of being
in one session from two places, and the reason the tower mirrors rather than
attaching the rest of the time.

## What it does not do

**bothy relays; it does not compose.** Everything that reaches an agent came
from your keyboard. bothy writes no message, answers nothing on your behalf and
sends nothing on a timer. During a take-over it is not relaying at all: the
session has your keyboard directly.

Beyond the line you type, the tower reads panes and writes nothing to them. It
starts and stops nothing, and creates no session but its own, so an agent cannot
be disturbed by being watched. It attaches to nothing unless you ask it to with
`/take`, because a second window on a session resizes it.

A relayed reply is text, and only text — not arrows, not tab, not Ctrl-C. A
question that has to be answered by moving a selection is what `/take` is for.

**It cannot bring a session's window to the front.** Selecting a row shows
which session needs attention; switching to it is a manual step. No Wayland
compositor allows one application to raise another's window, and bothy does not
install a shell extension to work around that.

## Why it expands the panes it watches

A mirror can only show what its pane displays, and a cockpit gives its agent
about a quarter of the window — 57 columns by 23 rows, against 191 by 46 for the
same pane filling its tab. So the tower makes each watched agent pane fullscreen
in its own window, which is what makes a mirror worth reading.

It reads the state before changing it, so a pane you had already expanded is
left alone, and it puts back only the panes it expanded — checking again on the
way out, because you may have collapsed one yourself in the meantime. Fullscreen
is `Ctrl+P` then `f`, and that keeps working normally while the tower runs.

`--no-expand` leaves your panes exactly as they are, at the cost of thin
mirrors. `--restore` collapses expanded agent panes, for when a terminal was
closed before the tower could put them back.

## Side by side, or stacked

Mirrors sit **side by side** when the window is wide enough to hold them all at
their own width, and stack when it is not; `bothy tower` says which it chose.
Stacked, each shows the bottom of its pane — enough to see which agent wants
you — and the multiplexer's fullscreen binding on a tower pane then shows the
whole thing.

## Flags

| | |
|---|---|
| `--mirror <session>` | watch one session and nothing else |
| `--every <duration>` | the refresh interval, two seconds by default |
| `--no-expand` | leave your panes as they are, at the cost of thin mirrors |
| `--restore` | collapse agent panes a closed terminal left expanded |

[All commands](Commands) · [Walling off the agent](Walling-off-the-agent)
