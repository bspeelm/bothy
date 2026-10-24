# Profiles

A profile is the layout: which panes, how big, and which slot goes in each.
Three ship. Switch with:

```sh
bothy config set profile editor
bothy                            # or --profile editor for one run
```

`bothy layout` prints the Zellij layout a profile would produce, without
opening anything.

## `cockpit` — the default

```
┌──────────────────────────────────────────────────┐
│            browser — 100% w, 50% h               │
├─────────────────────────────┬────────────────────┤
│        agent  60%           │     side  40%      │
└─────────────────────────────┴────────────────────┘
```

For supervising an agent working on a repository. The browser takes the top
half, for reading what changed, and the agent starts focused. The editor is
reached through `$EDITOR` from Yazi, git or the agent rather than occupying a
pane.

## `editor` — three columns

```
┌──────────────┬──────────────┬──────────────┐
│  editor 40%  │  agent  35%  │  side   25%  │
└──────────────┴──────────────┴──────────────┘
```

For working in the editor with an agent beside it. The editor slot takes the
largest pane and starts focused. There is no file browser pane, since the
editor has its own.

## `minimal` — agent and a shell

```
┌────────────────────────┬───────────────────┐
│       agent  65%       │     side  35%     │
└────────────────────────┴───────────────────┘
```

For small screens and SSH sessions, where a file browser costs more rows than
it is worth. The tab bar is off for the same reason; the status bar stays, as
the only on-screen reminder of Zellij's modal keys.

## Writing your own

Profiles are TOML. Copy one out of
[`profiles/`](https://github.com/bspeelm/bothy/blob/main/profiles) into
`~/.config/bothy/profiles/`, change it, and name it in your config.

```toml
name        = "wide"
description = "Browser down one side"

[[rows]]
panes = [
    { slot = "browser", size = "30%" },
    { slot = "agent", name = "agent", focus = true },
]
```

`rows` stack top to bottom; several `panes` in one row sit side by side. A pane
either names a `slot` — `browser`, `agent`, `editor`, `mux`, `terminal` — or
names nothing and gets a plain shell. Exactly one pane should have
`focus = true`.

**The three-pane invariant is about capability, not about your layout.** bothy
reports `panes` as available when the profile renders and the multiplexer
builds what it describes; a two-pane profile of your own is fine, and `bothy
doctor` will tell you what it got
([ADR-017](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-017--the-invariant-is-three-panes-everything-else-is-a-provider)).

[Swapping parts, and theming](Swapping-parts-and-theming) · [All commands](Commands)
