# bothy

A terminal workspace assembled from tools you already have. One command opens a
file browser, an agent and a shell in one window, configured and checked.

The [README](https://github.com/bspeelm/bothy) is the front door. These pages
are the detail. **Start with the words** — bothy uses a handful of terms
precisely, and nothing below makes sense without them.

## The words

| term | what it means here |
|---|---|
| **workspace** | the thing `bothy` opens: three panes in one terminal window. Not a project, not a directory — the room you work in |
| **pane** | one region of the window. The file browser across the top, the agent and a shell below. Three panes is the invariant: a stack that cannot produce them is not a bothy stack |
| **session** | a running workspace you can walk away from. Detach with `Ctrl-o d`, come back with `bothy attach`, and it has carried on without you. One per project directory, named after it |
| **slot** | a job to be filled, not a program. There are five: **terminal**, **mux**, **browser**, **editor**, **agent**. You say which program fills each |
| **provider** | a program that can fill a slot, described by one TOML file — how to detect it, fetch it, configure it. `zellij` fills the mux slot; `yazi` fills the browser slot |
| **mux** | short for multiplexer: the thing that splits one terminal into panes and keeps them running after you disconnect. Zellij, today |
| **profile** | the layout: which panes, what size, which slot goes where. Three ship — `cockpit` (the default three-pane room), `editor`, `minimal` |
| **capability** | something a stack can or cannot give you: **panes**, **sessions**, **theme**, **isolation**, **images**. `bothy doctor` reports each as available or not, because a terminal that cannot draw images cannot be configured into drawing them |
| **passthrough** | using your own config for a tool instead of bothy's. Name the slot, not the program |
| **box** | a toolbox or distrobox container: your home, your files, a different set of installed packages. bothy does not make them — `toolbox` does — but it remembers which box each project belongs in |
| **confine** | running the agent in a container with your project mounted and the rest of `$HOME` not. Opt-in, never automatic |
| **the lock** | [`bothy.lock`](https://github.com/bspeelm/bothy/blob/main/bothy.lock) — the version and checksum of every tool bothy would fetch. Nothing is downloaded that is not pinned here |

Two directories, and it is worth knowing which is which:

```
~/.local/share/bothy/   bothy's things — configs it generates, tools it fetched
~/.config/bothy/        your things — settings, palette, overrides
```

## Start here

- **[Your first session](Your-first-session)** — opening the room, what the
  three panes are for, moving between them, and leaving without losing them.

## Using it

- **[Commands](Commands)** — all sixteen, with their flags.
- **[The doctor](The-doctor)** — how to read a report, and why a capability can
  come back unavailable rather than broken.
- **[Installing](Installing)** — every channel, what each one checks, and the
  two platforms with edges.
- **[Security](Security)** — what bothy verifies, what it deliberately does
  not, and where the wall around the agent ends.
- **[Troubleshooting](Troubleshooting)** — the failures people actually hit,
  by symptom.
- **[What happens when you type bothy](What-happens-when-you-type-bothy)** —
  the run in order, with the reason for the order.

## Fitting it to your machine

- **[Toolboxes](Toolboxes)** — which box a project opens in, how bothy decides,
  and the commands for the boxes you already have.
- **[Walling off the agent](Walling-off-the-agent)** — setting up `bothy
  confine`, the toolbox case, and removing it.
- **[Profiles](Profiles)** — the three layouts that ship, and writing your own.
- **[The watermark](The-watermark)** — an image behind the terminal, off unless
  you point at one.
- **[Swapping parts, and theming](Swapping-parts-and-theming)** — the five
  slots and the palette.
- **[Where it runs](Where-it-runs)** — which terminals, which stacks, and what
  is advised but untested.
- **[What you can depend on](What-you-can-depend-on)** — the stability contract.

## Why it is like this

Every decision is numbered in [`docs/decisions.md`](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md), with
what was given up and what was refused.

---

*Generated from `wiki/` in the main repository. Edits made here are
overwritten; a wrong answer is a pull request.*
