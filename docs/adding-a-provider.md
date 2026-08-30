# Adding a provider

A provider is a data file and some templates. If adding one needs new Go code,
stop — that means the slot model is wrong, and the slot model is the bug to fix
(PLAN.md §8). This document should fit on one screen; if it stops fitting, the
same thing has gone wrong.

## 1. Write the templates

One file per config the tool needs, under `templates/<slot>/<provider>/`:

```
templates/editor/helix/config.toml.tmpl
```

Templates are Go `text/template` and see the `install.Data` struct — the palette
as `.Theme`, plus `.Container`, `.EditorBin`, `.ImagePreviews` and a few others.
Colours come from the eleven palette tokens:

```toml
[editor.statusline]
mode.normal = "{{ .Theme.Purple }}"
```

Keep conditionals to a handful. A template that needs more than about five
`{{if}}`s wants to be two providers.

## 2. Add it to the plan

In `internal/install/install.go`, `plan()` maps a configuration to files:

```go
if cfg.Slots.Editor == "helix" {
    out = append(out, file{
        Dest:     filepath.Join(p.ConfigRoot(), "helix", "config.toml"),
        Tool:     "helix",   // names the overrides/<tool>/ directory
        Template: "templates/editor/helix/config.toml.tmpl",
    })
}
```

That is the one place core code learns a provider exists. Everything after —
the generated-by header, the override merge, and uninstall — happens for free.

Destinations must be under `p.ConfigRoot()`; the writer refuses anything else,
which is ADR-009 enforced rather than merely intended. If your tool needs to be
told where its config lives, add the environment variable to
`install.SessionEnv` — that is what makes the isolation take effect at launch,
and the doctor uses the same environment so it checks the file the tool will
actually read.

## 3. Add its doctor checks

A provider that can fail silently needs a check. In `internal/doctor`:

```go
func checkHelixHealth(env Env) Result {
    if env.Config.Slots.Editor != "helix" {
        return skip("editor slot is not helix")
    }
    ...
}
```

Rules for a good check:

- **Only silent failures.** If the tool already prints a loud error, a check
  restating it earns nothing.
- **Test the effect, not the artefact.** "The colorscheme file exists" is not the
  same claim as "the colorscheme loaded", and only the second one is true when
  it matters.
- **Every failure carries a one-line fix.** A test enforces this. A diagnosis
  without a fix is just a nicer error message.
- **Skip when not applicable.** Never fail a check about a slot nobody selected.

## 4. Test it

Golden-file style: render the template, assert on the output. `internal/layout`
has the pattern. If your provider fixes a bug, the test is the bug.

## Themes specifically

A theme provider is a palette and nothing else — the per-tool templates are
shared. Fill the eleven tokens and every tool, including the generated vim
colorscheme, is themed.

There is exactly one palette in this repository, the open Dracula one in
`internal/theme/theme.go`, and `TestOnlyOpenDraculaColoursAreShipped` fails the
build if any other colour value appears in a shipped file. So a new palette does
not go here at all: it goes in a file on the user's machine, and they point
`theme.palette` at it. `bothy theme example` prints the blank form.

That applies to a palette of any provenance, licensed or free. If you want to
propose an additional *built-in* palette, it needs a licence that permits
redistribution, and the guard's allowlist has to be widened deliberately — which
is the review conversation, not an implementation detail.
