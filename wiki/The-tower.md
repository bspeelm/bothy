# The tower

Several sessions open across several windows, each with an agent working, and no
way to see them at once. `bothy tower` puts every running agent in one window.

```
$ bothy tower
watching 3 agent(s), side by side; 2 pane(s) expanded to be worth reading
```

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

The bottom two rows of a mirror are yours. Nothing the mirror draws reaches
them, so a reply stays on screen while the agent above it keeps working — you
can type at an agent mid-thought, which is when you most often want to.

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

Shift+Enter does not do this, and cannot. The tower reads your typing the way a
shell prompt does, where the terminal ends the line when you press Enter — so
there is no key that means "newline, but keep going". The backslash is the
shell's answer to the same problem.

If a reply cannot be delivered — the session went away, or its agent pane did —
the mirror says so where you typed it, rather than letting the line vanish.

## Taking over a session

A mirror is a picture, and a picture cannot answer a menu. When an agent asks
something that needs a keystroke rather than a sentence — a numbered choice, a
y/n, anything you would answer with the arrow keys — type `/take` and press
Enter.

That pane stops mirroring and becomes the session: a real client, attached, so
every key works and the scrollback is there. Arrows, Esc, Tab, Ctrl-C, the
agent's own bindings. You are in it rather than looking at it.

Leave the way you leave any session — `Ctrl-o d` — and the pane goes back to
mirroring. The tower was running underneath the whole time; the other mirrors
never stopped.

To send the word itself to an agent rather than taking over, double the slash:
`//take`.

**Expect a few blank rows at the bottom.** A session running inside a pane is
given a little less room than the pane holds. Measured during a take-over: a
47-row mirror pane handed the session 42 rows, so five were left over.

Where they go is the multiplexer's business and not something bothy can hand
back — dropping the mirror's own frame was tried and recovers nothing, because
the frame is already accounted for. It is about a tenth of the pane, and it is
at the bottom, below what you are reading.

The other cost is the one attaching always has: while you are in, the session
has two windows, and it is sized to fit both. If its own window is still open
somewhere it may shrink there too, and it grows back when you leave. That is why
the tower mirrors rather than attaching the rest of the time.

**`Ctrl-q` while taken over ends that session**, exactly as it would in the
session's own window. The keys are the session's while you are in it, so bothy
cannot intercept that one. `Ctrl-o d` is the way out.

## What it will not do

**bothy relays; it does not speak.** Everything that reaches an agent came from
your keyboard. bothy composes nothing, answers nothing on your behalf, and
sends nothing on a timer. During a take-over it is not even relaying: the
session has your keyboard directly, and bothy is not in between.

Beyond the line you type, the tower reads panes and writes nothing to them. It
starts and stops nothing, and creates no session but its own, so an agent cannot
be disturbed by being watched. It attaches to nothing unless you ask it to with
`/take`, because a second window on a session resizes it.

A relayed reply is text, and only text — not arrows, not tab, not Ctrl-C. A
question that has to be answered by moving a selection is what `/take` is for.

**It cannot bring a session's window to the front.** Selecting a row shows you
which session wants attention; switching to it is yours to do. No Wayland
compositor lets one application raise another's window, and bothy will not
install a shell extension to get around that.

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
