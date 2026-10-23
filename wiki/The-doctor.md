# The doctor

`bothy doctor` runs 32 checks against the workspace and says what is
wrong and what to type. It is the command the project is built around: every
setup failure that has ever been fixed here ships with a check that detects it,
so the doctor is where the project's accumulated experience lives.

```sh
bothy doctor          # for you
bothy doctor --json   # for something that parses it
```

## Reading the output

Each line is a check. A failing one carries a reason and a fix:

```
✓ yazi 26.8.15
! image previews are enabled but this machine cannot show them properly
    ghostty draws images with the Kitty graphics protocol, but inside a
    multiplexer Yazi sends an older Kitty graphics variant instead
    fix: run 'bothy install' to regenerate yazi.toml with the placeholder previewer
```

| mark | severity | means |
|---|---|---|
| `✓` | pass | working |
| `!` | warn | works, but not as intended |
| `✗` | fail | the workspace is broken — the command exits non-zero |
| | skip | not applicable to this machine, and not counted against you |

**Only a failure makes the command exit non-zero.** Warnings do not, which is
why the example above ends in exit 0: a config that promises previews this
stack cannot draw is worth knowing about and is not a broken workspace.

## The summary, and capabilities

The last two lines are the point:

```
20 passed, 2 warning(s), 0 failure(s), 6 not applicable

this stack gives you: panes, sessions, theme, isolation
it cannot give you:   images
```

Checks are grouped into five capabilities, and a capability is reported as
available only when everything it depends on passes. This is deliberately
honest about the machine you are on rather than about bothy: no terminal can
be given image previews through a multiplexer by any amount of configuration,
so bothy says so once, plainly, instead of failing repeatedly.

| capability | what it means | checks behind it |
|---|---|---|
| **panes** | the layout renders and the multiplexer builds it | `profile-renders`, `layout-built` |
| **sessions** | you can detach and reattach | `session-named` |
| **theme** | one palette reaches every tool | `theme-palette`, `theme-reached` |
| **isolation** | bothy's config is used, and yours is untouched | `yazi-config-discarded`, `passthrough`, `isolation`, `confine`, `tool-data`, `mux-config` |
| **images** | inline image previews, which no stack currently delivers | `image-previews`, `terminal-capability` |

The remaining checks are not tied to a capability and always run: tool versions
and reachability, config schema and key names, the terminfo entry, the file
opener and its recursion guard, the agent and editor being present, the
watermark image if you set one, and — on macOS — whether the binary is
quarantined.

## `--json`, and what it promises

`--json` exists so something other than a person can read the report. Its shape
is a stability surface, and what that obliges is set out in
[What you can depend on](What-you-can-depend-on) and
[ADR-036](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-036--what-stable-obliges-and-why-the-config-schema-warns).

The IDs are the ones listed in `Checks()` in
[`internal/doctor/doctor.go`](https://github.com/bspeelm/bothy/blob/main/internal/doctor/doctor.go).

## When a check is added

A check is added whenever a setup failure is fixed, which means the list grows.
The expectation tables in the container and macOS tests are exhaustive in both
directions: a new check that is not added to them fails CI, so nothing arrives
unnoticed and nothing goes silently unexercised.
