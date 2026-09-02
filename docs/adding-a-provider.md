# Adding a provider

Almost everything bothy knows is data. A provider is a TOML file and some
templates; if adding one needs new Go code, stop — that means the slot model is
wrong, and the slot model is the bug to fix (PLAN.md §13). This document should
fit on one screen; if it stops fitting, the same thing has gone wrong.

There are three kinds of thing you might add.

## The header every provider carries

Whichever kind it is, the file opens by saying what it *is*, not just how to
get it:

```toml
name      = "helix"
what      = "the editor in the editor profile"
slot      = "editor"                 # omit it for an extra, which fills none
platforms = ["linux", "darwin"]
provides  = ["theme"]                # doctor capabilities it contributes to
```

`slot` is the one with teeth: `bothy config set slots.mux helix` is refused,
because helix says it fills `editor`. A name bothy has no file for is still
accepted — the agent slot takes any command you care to name — so the check
catches a contradiction and stays out of the way otherwise.

`platforms` restates the OS prefixes of `[assets]` (or the keys of `[install]`),
and a test holds them together. `provides` is how bothy tells a capability
nothing verified from one nothing in your stack was ever going to do.

## A tool bothy can supply

One file in `slots/tools/`. Nothing else.

```toml
name        = "helix"
what        = "the editor in the editor profile"
binary      = "hx"
repo        = "helix-editor/helix"
slot        = "editor"
platforms   = ["linux", "darwin"]
min_version = "25.01.0"
reason      = "why this minimum exists, shown when bothy replaces someone's copy"

[assets]
linux_x86_64   = "helix-{version}-x86_64-linux.tar.xz"
linux_aarch64  = "helix-{version}-aarch64-linux.tar.xz"
darwin_x86_64  = "helix-{version}-x86_64-macos.tar.xz"
darwin_aarch64 = "helix-{version}-aarch64-macos.tar.xz"
```

Then `bothy lock` downloads each asset and records its checksum, and the entry
is committed alongside the definition.

Two rules that matter more than they look:

**`min_version` is the oldest that actually works, not the newest available.**
bothy replacing a working binary is a real intrusion, so the bar is "this
version cannot do the job", not "there is something newer". Most tools should
have a low minimum and never be fetched at all.

**`reason` is not optional when a minimum is doing work.** It is the sentence
someone reads when bothy tells them their binary is not good enough. "zellij
0.42.2 is below 0.45.1" is an assertion; adding "image previews need the Kitty
graphics protocol, added in 0.45" is an argument. A test enforces that the
tools with real minimums carry one.

Extraction needs no configuration: bothy matches on the binary's basename
anywhere in the archive, which covers all four layouts upstream actually uses.
Bare binaries, `.tar.gz` and `.zip` work; `.tar.xz` does not, because the
standard library cannot unpack it and PLAN.md caps dependencies at `go-toml`.
That is the only reason helix is not already here.

## A configuration provider

Templates in `templates/<slot>/<provider>/`, and one entry in
`install.plan()`:

```go
if cfg.Slots.Editor == "helix" {
    out = append(out, file{
        Dest:     filepath.Join(p.ConfigRoot(), "helix", "config.toml"),
        Tool:     "helix", // names the overrides/<tool>/ directory
        Template: "templates/editor/helix/config.toml.tmpl",
    })
}
```

That is the one place core code learns a provider exists. The generated-by
header, the override merge and uninstall all follow for free.

Destinations must be under `p.ConfigRoot()`; the writer refuses anything else,
which is ADR-009 enforced rather than intended. If the tool needs telling where
its config lives, add the environment variable to `install.SessionEnv` — that
is what makes isolation take effect at launch. The doctor uses the same
environment, so a check always inspects the file the tool will really read.

Templates see `install.Data`: the palette as `.Theme`, plus `.Container`,
`.ImagePreviews`, `.Plugins` and a few others. Keep conditionals to a handful;
a template needing more than about five `{{if}}`s wants to be two providers.

## A plugin the config depends on

If a generated config *references* something, bothy must install it and the
config must be written to match what is present. Add it to
`slots/plugins/yazi.toml` and gate the reference:

```
{{- if index .Plugins "git" }}
require("git"):setup { order = 1500 }
{{- end }}
```

This rule exists because it was broken. bothy's `init.lua` required two plugins
it never installed, and the config check could not see it: `yazi --clear-cache`
does not execute `init.lua`. The config looked correct, passed, and would have
failed only at launch.

## Doctor checks

A provider that can fail silently needs a check.

- **Only silent failures.** If the tool already prints a loud error, restating
  it earns nothing.
- **Test the effect, not the artefact.** "The colorscheme file exists" is not
  the claim "the colorscheme loaded", and only the second is true when it
  matters.
- **Every failure carries a one-line fix.** A test enforces this.
- **Skip when not applicable** — never fail a check about a slot nobody chose,
  or about a config the user passed through.
- **Ask the tool the way bothy will run it.** Use `env.tool(...)`, not
  `exec.Command`. Checking a binary or config other than the one the launcher
  uses produces a confident report about the wrong thing; this went wrong twice
  during development.

## Themes

A theme provider is a palette and nothing else — the per-tool templates are
shared, and filling the eleven tokens themes everything, including a generated
vim colorscheme.

There is exactly one palette in this repository, and
`TestOnlyOpenDraculaColoursAreShipped` fails the build if any other colour
value appears in a shipped file. So a new palette does not go here at all: it
goes in a file on the user's machine, and they point `theme.palette` at it.
`bothy theme example` prints the blank form. That applies to a palette of any
provenance. Proposing an additional *built-in* palette needs a licence
permitting redistribution and a deliberate widening of the guard's allowlist —
which is a review conversation, not an implementation detail.
