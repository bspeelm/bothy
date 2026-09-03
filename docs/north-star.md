# North star

> *A room you can trust because it wants nothing. The agent, of course, is
> another matter.*

What bothy is, so that every milestone can be judged by whether it gets closer.
The reasons things are the way they are are in [`decisions.md`](decisions.md).
The road as it was planned is [`history/plan-1.0.md`](history/plan-1.0.md), kept
as a record now rather than a route.

---

## 1. The invariant

bothy exists to produce one thing:

```
┌──────────────────────────────────────────────────┐
│                  files                           │
├─────────────────────────────┬────────────────────┤
│           agent             │       shell        │
└─────────────────────────────┴────────────────────┘
```

Three panes. Files above, so you can see what the agent is touching. The agent
below left, focused, because it is what you are there to watch. A shell below
right, because you will want to run something without leaving. Arranged for
you, on whatever you already have, and removed without a trace.

That is the whole product. Everything else — which terminal draws it, which
multiplexer splits it, which file browser fills the top pane, which agent runs
in the middle — is a *provider* of one *slot*.

ADR-016 settled what bothy is *for*: an agent on a repository, which is a
narrower claim than "terminal workspace" and deliberately costs audience. This
document settles what it runs *on*, and widens it. Narrow about the purpose,
broad about the substrate — the two do not pull against each other, because the
purpose is what makes the layout worth reproducing anywhere.

## 2. The analogy, and its limit

The model is Dracula: one outcome, a palette, and hundreds of ports, each a
small file, each maintained by whoever needed it. The outcome is recognisable on
every port even though no two are implemented alike.

bothy borrows the shape — one layout, a matrix of stacks, the matrix expressed
as data. What it does not borrow is silence about gaps. A Dracula port may
implement three of eleven colours and say nothing. bothy says, for every stack,
what the invariant can and cannot deliver on it. **The report is part of the
product.**

## 3. Capability tiers

Not every stack can give you the whole cockpit. Rather than pretend, the
invariant splits into capabilities, and every provider declares which it
supplies:

| capability | what it means | who supplies it |
|---|---|---|
| **panes** | the three-pane arrangement itself | the multiplexer, or a terminal that splits |
| **sessions** | `bothy attach` after you close the window | a multiplexer with detach |
| **images** | real image previews in the file pane | terminal *and* multiplexer must both pass a graphics protocol |
| **theme** | every pane in the same palette | each tool, where its config can be pointed at bothy's tree |
| **isolation** | nothing of yours written to | every tool that accepts a config path or environment variable |

**panes is mandatory**: a stack that cannot give you three panes is not a bothy
stack. The other four are reported. This is ADR-007's *gate, probe, explain*
applied to the whole product rather than to one workaround — "this stack cannot
give you images" belongs in the plan, not in a surprise at launch.

Where a tool cannot be redirected — a terminal whose settings live in a file
with no `--config-file` equivalent — bothy does not theme it and says why.
ADR-009 holds. The promise weakens on that stack; it is never broken.

## 4. Slots and providers

The slot model was built for this, and three things have to change before it
can carry it.

**There is one provider format, and today there are three.** `slots/` holds
three unrelated dialects: tools (`name`, `binary`, `repo`, `min_version`,
`reason`, `[assets]`), advice (`name`, `what`, `binary`, `[install]`,
`[[avoid]]`), and plugins (`name`, `use`, `gives`, `needs`), parsed by three
structs in three packages. The per-slot directories the model implies —
`slots/mux/`, `slots/browser/`, `slots/terminal/` — exist on disk and are empty.
Unifying them is the largest single item on the road, and everything below
assumes it.

**Providers declare where they run and what they give.** The unified file gains
three fields:

```toml
platforms = ["linux", "darwin"]
provides  = ["panes", "sessions", "images"]
redirect  = "env"          # env | flag | none — how bothy points it at its own config
```

The planner in §5 is a walk over these. A new terminal or multiplexer is a new
file, and the matrix updates itself.

**There are three provider tiers, and saying so is more honest than claiming
one.** ADR-005 says adding a provider must touch no Go. That has never been
strictly true and pretending otherwise hides where the real cost is:

| tier | what it takes | examples |
|---|---|---|
| **data** | one TOML file | every provider in `slots/` |
| **data + a branch** | a file, templates, one arm in `install.plan()` | yazi, ghostty, vim |
| **data + a renderer** | all of the above, plus Go that *interprets* the profile | the multiplexer |

The multiplexer is the third tier because Zellij takes a KDL layout file, tmux
takes a sequence of `split-window` commands, and Windows Terminal takes `wt`
arguments. Those are not templates of one thing; they are renderers of one
profile. The cost is a renderer, a doctor check and a graphics gate per backend,
and it is accepted knowingly rather than discovered.

The second tier should shrink as the first grows. `advice.binary` is parsed and
never read while three Go maps duplicate it; that is the first thing the unified
format buys back.

## 5. Sense, score, constrain, recommend, apply

The first run, and `bothy plan` at any time, does one pass:

1. **Sense.** Platform, terminal, display, container, what is on `PATH` and at
   what version.
2. **Score.** For each slot, which providers are present and meet their minimum.
   Present and good enough is free. Present but too old costs a fetch. Absent
   costs an install. `min_version` and `reason` already carry the "why".
3. **Constrain.** A choice in one slot narrows the others. Terminal.app rules
   out images. No display rules out opening a window. This is a walk over
   `platforms` and `provides` — no Go per provider.
4. **Recommend.** Fill each slot with the best provider the constraints allow,
   preferring what is already there. For each gap: what to install, why, what it
   weighs, and which capability it adds.
5. **Apply.** Show the plan. Ask. Do exactly that and nothing else.

```
bothy: sensing

  terminal   iTerm2 3.5        usable  (images: its own protocol)
  mux        zellij 0.45.1     usable  (sessions: yes)
  browser    yazi 25.5         too old — image previews need 26.0
  agent      claude            usable
  editor     nvim              usable
  extras     fzf, rg, fd       usable
             lazygit, zoxide   absent

  recommended:
    keep      iTerm2, zellij, claude, nvim, fzf, ripgrep, fd
    fetch     yazi 26.8.15  (own copy, yours untouched)   ~12 MB
    fetch     lazygit, zoxide                              ~18 MB

  this stack gives you: panes, sessions, isolation
  it cannot give you:   a themed terminal (iTerm2 keeps its own settings)
  unverified:           image previews — probed at launch

  proceed? [y/n]
```

Three rules keep it honest:

- **Prefer what is there over what is best.** The suggestion to switch is one
  line at the bottom, never the default. This is "fill gaps, never replace"
  applied to choices.
- **Never recommend what it will not install.** Terminals and agents stay in
  an `[advise]` block, listed separately from what bothy will fetch.
- **Say what the stack cannot do.** Every plan ends with the capability lines. A
  stack that gives you panes and nothing else is still a stack; the user decides
  whether it is enough.

`bothy` on first run does this. `bothy plan` re-runs it. `bothy doctor` stays as
the after-the-fact check. One engine, three entry points.

## 6. Reference stacks

"As broad as possible" is the matrix. "Supported" is narrower, and ADR-012
defines it: a stack is supported when CI installs it, runs the doctor, and
matches an expected capability table. Every supported stack is a job forever, so
few are named.

For each platform, two: the **ideal** — everything the invariant can offer — and
the **common**, what a typical user already has, so that the first run installs
little or nothing.

### Linux

| | terminal | mux | browser | capabilities |
|---|---|---|---|---|
| ideal | Ghostty | Zellij | Yazi | panes, sessions, images, theme, isolation |
| common | the one you have | Zellij | Yazi | panes, sessions, isolation; theme and images depend on the terminal |

Already the ideal. The common stack is a terminal bothy neither spawns nor
themes: Konsole draws images by sixel, GNOME Terminal's VTE ships no graphics
protocol at all and gets text previews, said out loud rather than substituted
in silence.

### macOS

| | terminal | mux | browser | capabilities |
|---|---|---|---|---|
| ideal | Ghostty | Zellij | Yazi | panes, sessions, images, theme, isolation |
| common | iTerm2 | Zellij | Yazi | panes, sessions, isolation; theme not offered; images unverified |

iTerm2 draws inline images by its own protocol rather than Kitty's, and whether
Zellij passes that through is exactly what a runtime probe is for — so it is
reported unverified until one answers.

Terminal.app is *reported*, not supported: it cannot split, cannot draw images,
and cannot be pointed at a config. It gives panes through the multiplexer and
nothing else. The plan says so.

Homebrew is the install advice for everything bothy will not fetch. bothy's tree
stays at `~/.local/share/bothy`, which Yazi and Zellij honour on macOS.

### After 1.0

Native Windows and a second multiplexer are direction, not content. Both become
additive once the mux backend interface exists and providers declare their
platforms, which is why those two land before 1.0 and these two do not.

- **tmux.** The second backend, and the first time the invariant is produced two
  ways. It buys the Linux and macOS common stacks a multiplexer their users
  already have.
- **Native Windows.** Windows Terminal's own panes are the "multiplexer": panes
  and isolation, no sessions, no theme. It needs windows entries across the tool
  matrix, and a second bootstrap in a second language — ADR-001 permits exactly
  one shell file, and it rejects Windows by name.

## 7. What this costs

Stated plainly, because a plan that only lists benefits is a sales document
(PLAN.md §9).

**The multiplexer is seven seams, not one.** `internal/layout` is a Zellij KDL
emitter under a generic name, and six more `== "zellij"` decisions live outside
it: the config directory, the template branch, `ZELLIJ_CONFIG_DIR`, `--layout`
at launch, the graphics gate's version check, and a doctor check that counts
panes by reading Zellij's own session-layout file. A backend interface has to
cover all seven or the second backend leaks through the gaps.

**Every supported stack is a job forever.** ADR-012 is what makes "supported"
mean anything, and it is also the bill. Each row in §6 is a CI job that can go
red on someone else's schedule.

**The container job's design does not port.** Its filesystem assertions work
because `$HOME` is a bind-mounted host directory the test inspects from outside.
A macOS runner has no equivalent, so it is a differently shaped job rather than
another row in the same table.

**Three terminal tables have to become one.** bothy recognises four terminals,
scores three for graphics, and can spawn exactly one. A capability model needs a
single table, and that is where iTerm2 and the terminals that draw nothing both
have to be written down.

**The planner overlaps the doctor and must not duplicate it.** One is
prospective and one retrospective; if they answer the same question twice they
will eventually disagree, and the disagreement will be silent.

## 8. What 1.0 claims

> The cockpit on Linux and macOS, tested in CI on both. A plan for any other
> stack that says what it can and cannot give you. A provider format you can add
> to without writing Go — except the multiplexer, which is the hard part and the
> part bothy owns.

And what it does not claim: that every terminal is themed, that image previews
work everywhere, or that bothy has an opinion about which of your tools you
should replace. It has a recommendation, in a line at the bottom, and it wants
nothing.
