# Adding a provider

Almost everything bothy knows is data. A provider is a TOML file and some
templates; if adding one needs new Go code, stop — that means the slot model is
wrong, and the slot model is the bug to fix (PLAN.md §13).

That used to be aspirational for a provider bothy generates config for: it also
needed an arm in `install.plan()`. It no longer does (ADR-032), so tier two of
ADR-019 now holds only the multiplexer.

A provider is **one file at `slots/<name>.toml`**. It says what it is, at most
one way of being obtained, and any config bothy should generate for it.

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

## `[fetch]` — bothy downloads it

```toml
[fetch]
binary      = "hx"
repo        = "helix-editor/helix"
min_version = "25.01.0"
reason      = "why this minimum exists, shown when bothy replaces someone's copy"
[fetch.assets]
linux_x86_64  = "helix-{version}-x86_64-linux.tar.xz"
darwin_arm64  = "helix-{version}-aarch64-macos.tar.xz"
```

Then `bothy lock` downloads each asset and records its checksum, committed
alongside the definition. `platforms` restates the OS prefixes of
`[fetch.assets]`, and a test holds them together.

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

## `[advise]` — bothy tells you how

For what bothy will not install: it needs root, publishes no binaries, or is
personal (ADR-014).

```toml
[advise]
binary = "hx"
[advise.install]
fedora-ostree = "sudo rpm-ostree install helix && systemctl reboot"
fedora        = "sudo dnf install -y helix"
darwin        = "brew install helix"
[[advise.avoid]]
repo    = "some/repo"
why     = "what it breaks"
distros = ["fedora"]
```

The keys are tried most-specific first — `<distro>-ostree`, distro, distro
family, OS, `default` — so an image-based host gets the command that needs a
reboot and everyone else does not.

## `[[file]]` — bothy generates its config

```toml
[[file]]
template = "templates/editor/helix/config.toml.tmpl"
dest     = "helix/config.toml"

[[file]]
template = "templates/theme/helix.toml.tmpl"
dest     = "helix/themes/{theme}.toml"
when     = "provide-editor-config"
```

`dest` is relative to the config root, with `{theme}` interpolated. It is
spelled out rather than derived because three of bothy's ten generated files
break any convention that fits the other seven. The writer refuses a
destination outside the config root, which is ADR-009 enforced rather than
intended.

`when` names a condition from a **closed vocabulary** in `install.conditions` —
`no-images`, `provide-editor-config`. An expression language would want a
parser and PLAN.md §13 allows one dependency, already spent on TOML. A test
asserts every `when` in `slots/` is a key in that map; add the key and the
condition together, or not at all.

If the tool needs telling where its config lives, add the environment variable
to `install.SessionEnv` — that is what makes isolation take effect at launch.
The doctor uses the same environment, so a check always inspects the file the
tool will really read.

Templates see `install.Data`: the palette as `.Theme`, plus `.Container`,
`.ImagePreviews`, `.Plugins` and a few others. Keep conditionals to a handful;
a template needing more than about five `{{if}}`s wants to be two providers.

## `[[plugin]]` — what the generated config depends on

If a generated config *references* something, bothy must install it and the
config must be written to match what is present.

```toml
[[plugin]]
name = "git"
use  = "yazi-rs/plugins:git"
rev  = "c591a36e7263e95497715d525e9c46c2f0a880ac"
```

`rev` is the pin, and it is the point: resolving on the machine gave two people
installing a week apart different plugins. Gate the reference in the template:

```
{{- if index .Plugins "git" }}
require("git"):setup { order = 1500 }
{{- end }}
```

This rule exists because it was broken. bothy's `init.lua` required two plugins
it never installed, and the config check could not see it: the command it uses
parses the config without executing `init.lua`. The config looked correct,
passed, and would have failed only at launch.

## Platforms

Differences between machines are injected, not compiled out: pass the thing
that varies as a parameter, or put it behind a variable a test can replace, the
way `install.go`'s `terminalSize` does. A `//go:build` file is for code the
compiler rejects elsewhere -- a raw syscall -- and nothing else, because CI
runs one platform and never compiles the other side of a tag. ADR-031, and
`TestPlatformSplitsStayShims` enforces it.

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
