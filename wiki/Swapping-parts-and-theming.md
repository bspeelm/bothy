# Swapping parts, and theming

Every part can be changed, and most installations change nothing.

## The five slots

| part | default | alternatives |
|---|---|---|
| terminal | ghostty, or wezterm on Windows | kitty — none of which bothy installs |
| multiplexer | zellij | or turn it off, and run the agent in this terminal |
| file browser | yazi | or turn it off |
| editor | vim | nano, helix |
| agent | claude-code | any command you care to name |
| theme | dracula | any palette you write down |

```sh
bothy config set slots.editor helix
```

The editor is the slot most often changed. Changing any of the others changes
the whole workspace at once.

The layout is separate from the parts, and is called a profile. Three ship, and
`cockpit` is the default.

```sh
bothy config set profile minimal
```

[Profiles](Profiles) describes each one and how to write your own.

## Theming

bothy ships one palette, [Dracula](https://github.com/dracula/dracula-theme),
and colours every tool from the same eleven values — including a vim colour
scheme it generates, so that no tool has to be themed by hand. To use another,
write it:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how a licensed palette is used. bothy contains no colours except
its own, so a palette you licensed stays on your machine. A test enforces
that rather than leaving it to intent.

## Using your own tool config instead

If you already have a Yazi configuration you want to keep, keep it.

```toml
# ~/.config/bothy/config.toml
passthrough = ["browser"]
```

bothy then points Yazi at your config rather than its own, and does not read
yours on the way past. Name the slot rather than what is in it — `"yazi"` is
understood too, and stops meaning anything the day you put something else in
the browser slot. To adjust bothy's config rather than replace it, put a
file in `~/.config/bothy/overrides/<tool>/<file>`. bothy appends it to its own,
so where the two disagree, yours applies.

---

Why the slots are shaped this way, and why only the multiplexer needs Go:
[ADR-017](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-017--the-invariant-is-three-panes-everything-else-is-a-provider)
and [ADR-019](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-019--amends-adr-005-three-provider-tiers-stated).
Adding one is [`docs/adding-a-provider.md`](https://github.com/bspeelm/bothy/blob/main/docs/adding-a-provider.md).
