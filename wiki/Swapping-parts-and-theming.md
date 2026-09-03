# Swapping parts, and theming

Every part can be changed. Most of it needn't be, and most of it won't be.

| part | default | alternatives |
|---|---|---|
| terminal | ghostty | kitty, wezterm — none of which bothy installs |
| multiplexer | zellij | or turn it off, and run the agent in this terminal |
| file browser | yazi | or turn it off |
| editor | vim | nano, helix |
| agent | claude-code | any command you care to name |
| theme | dracula | any palette you write down |

```sh
bothy config set slots.editor helix
```

Most people change the editor and nothing else. The others change everything,
once, and then also nothing else.

Three layouts come with bothy, and are called profiles because everything
has to be called something. `cockpit` is the default: files on top, agent and
shell beneath — the screenshot, and the reason any of this exists. `editor`
puts an editor, an agent and a shell side by side, for people who would
rather type than watch. `minimal` is an agent and a shell and nothing else,
for small screens and for being somewhere else over SSH.

```sh
bothy config set profile minimal
```

Profiles are short TOML files. Write your own and put it in
`~/.config/bothy/profiles/`.

## Theming

bothy ships one palette, [Dracula](https://github.com/dracula/dracula-theme),
which has outlasted most of the software it was first drawn for, and colours
every tool from the same eleven values, including a vim colour
scheme it writes for you so that you do not have to learn how. If you would
prefer another, write it:

```sh
bothy theme example > ~/.config/bothy/my-palette.toml
$EDITOR ~/.config/bothy/my-palette.toml
bothy config set theme.palette ~/.config/bothy/my-palette.toml
```

This is also how a palette you have paid for gets in. bothy contains no
colours except its own, so anything you licensed stays on your machine, and
bothy stays the colour it arrived in. A test checks this, because good
intentions do not.

---

Why the slots are shaped this way, and why only the multiplexer needs Go:
[ADR-017](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-017--the-invariant-is-three-panes-everything-else-is-a-provider)
and [ADR-019](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-019--amends-adr-005-three-provider-tiers-stated).
Adding one is [`docs/adding-a-provider.md`](https://github.com/bspeelm/bothy/blob/main/docs/adding-a-provider.md).
