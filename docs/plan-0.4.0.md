# Plan — the provider format, across 0.4.0 and 0.5.0

#69 says "three dialects become one". That is the wrong description of the
problem, and following it would produce a tidier `slots/` that fixes nothing.

This proposes what to build instead, and why. It is a proposal: the open
questions at the end are open.

---

## The bug that makes the case

`passthrough` is a documented feature and it does not work in either form.

| you write | `bothy config set` | `bothy install` |
|---|---|---|
| `["yazi"]` — the README's form | **rejects it**: *"not a slot (terminal, mux, browser, editor, agent)"* | **works** |
| `["browser"]` — `Validate`'s form | accepts | **silently ignores it** |

`Validate` checks against slot names. Every consumer asks by provider name —
`PassesThrough("yazi")`, `PassesThrough("zellij")`, `PassesThrough("vim")`.
Both halves are internally consistent and they disagree with each other.

Nobody wrote a bug. Two people wrote the obvious thing, at different times,
and there was nothing to be wrong against — because **nothing in the data says
that `yazi` fills `browser`**.

That is the defect. The three dialects are a symptom.

Put plainly: **a provider file says how to get the program and nothing about
what it is.** `slots/tools/yazi.toml` carries a repository, a version minimum
and a set of checksums, and never says that yazi is the file browser or that
it is what makes image previews possible. Both of those are things it
provides, in the two senses the format needs — `slot` for the role it fills,
`provides` for the capabilities it delivers. The doctor currently knows the
second only because it is typed by hand into `Checks()`.

## What is actually wrong

Slot membership lives in Go, as literals, thirteen times in `install.go`
alone:

```go
if cfg.Slots.Mux == "zellij" && !cfg.PassesThrough("zellij") { … }
if cfg.Slots.Browser == "yazi" && !cfg.PassesThrough("yazi") { … }
if cfg.Slots.Editor == "vim" && cfg.Editor.ProvideConfig { … }
if cfg.Slots.Terminal == "ghostty" && !cfg.PassesThrough("ghostty") { … }
```

Each pairs a slot with a provider and a set of templates. `slots/tools/`,
`slots/advice/` and `slots/plugins/` describe those same providers and none of
them mentions a slot. ADR-005 promises adding a provider touches no Go;
`docs/adding-a-provider.md` then shows the Go snippet it needs, which is where
ADR-019's second tier came from.

So: **the format's job is not to merge three TOML shapes. It is to make the
provider-to-slot relationship exist somewhere a program can read.**

## What the format is

One file per provider, at `slots/<slot>/<name>.toml`. The per-slot directories
already exist on disk, empty and untracked, which is how long this has been
the intended shape.

```toml
name   = "yazi"
slot   = "browser"
binary = "yazi"
what   = "the file browser"

platforms = ["linux", "darwin"]
provides  = ["images"]
redirect  = "env"          # env | flag | none — how bothy points it at its config

# How bothy gets it, if it can. Absent for a provider it only names.
[fetch]
repo        = "sxyazi/yazi"
min_version = "26.0.0"
reason      = "the plugin ecosystem and bothy's config both require 26.x keys"
extra       = ["ya"]
[fetch.assets]
linux_x86_64 = "yazi-x86_64-unknown-linux-musl.zip"

# What to tell someone when bothy cannot.
[advise]
fedora = "sudo dnf install -y yazi"
darwin = "brew install yazi"

# What bothy writes for it.
[[file]]
template = "yazi.toml.tmpl"
dest     = "yazi/yazi.toml"

[[file]]
template = "theme.toml.tmpl"
dest     = "yazi/theme.toml"
```

`platforms`, `provides` and `redirect` are the north star's §4 fields, and are
the point of doing this now rather than later: the planner (#74) is a walk over
them, and the capability model (ADR-017) has nowhere else to read `provides`
from.

**This is a common header plus one acquisition block, not a merge.** The three
dialects do genuinely different jobs — fetch a release, name a command, pin a
git revision. What they share is *identity*: name, binary, slot, platforms,
what it provides. Pretending `[fetch]` and `[advise]` are the same shape would
be the tidiness that fixes nothing.

## Three things that are harder than they look

**Destinations are not a convention.** `templates/<slot>/<provider>/x.tmpl` →
`<ConfigRoot>/<provider>/x` holds for four of the seven generated files and
breaks for three: zellij's theme goes to `zellij/themes/{theme}.kdl`, vim's
colourscheme to `vim/colors/{theme}.vim`, and ghostty's config to
`ghostty.conf` rather than `ghostty/config`. So `dest` is per-file data with
`{theme}` interpolated, not a rule.

**Some files are conditional.** yazi's `enter-hint.lua` is written only when
image previews are off; vim's two only when `editor.provide_config` is set. A
`when` field naming a condition bothy knows keeps this in data without
inventing an expression language, which the one-dependency budget rules out:

```toml
[[file]]
template = "enter-hint.lua.tmpl"
dest     = "yazi/plugins/enter-hint.yazi/main.lua"
when     = "no-image-previews"
```

The vocabulary is small and closed. That is honest tier two — data plus a
named condition — rather than a claim to have eliminated Go.

**Plugins belong to a provider, not to bothy.** `slots/plugins/yazi.toml` is a
list of things yazi needs. It folds into yazi's own file as `[[plugin]]`,
which is where it always belonged, and the third dialect disappears without
being merged into anything.

## What stays in Go, deliberately

`internal/doctor/checks_yazi.go` is 182 lines of yazi-specific knowledge —
that `yazi --clear-cache` parses the config without executing `init.lua`, that
a fetcher entry without a setup call silently does nothing. That is not
configuration and no format should try to hold it.

ADR-019's tiers stand. This moves providers from tier two to tier one and
leaves the multiplexer's renderer in tier three, where 0.5.0 finds it.

## Order

Split across two milestones. 0.4.0 makes providers *declare* what they are;
0.5.0 changes where they live and how they are parsed, alongside the mux seam
(#64) — which is the same problem seen from the other end, a provider that
still needs Go.

Each step ends green.

1. **Fix `passthrough`.** A live bug in a documented feature, independent of
   the format. Slot names win — they are what `Validate` already demands and
   what the doctor's newest check already uses — and `PassesThrough` resolves
   the configured provider for a slot. `passthrough = ["yazi"]` becomes a
   retired spelling (ADR-027's machinery, which this is the first real use of),
   and the README changes.

2. **`slot` in the tool files, and `plan()` reads it.** The four `if` branches
   become a loop over providers for the configured slots. Highest structural
   value, no new file format yet.

3. **The common header.** `platforms`, `provides`, `redirect`, `what` — added
   to the files that exist, consumed by the capability report and by nothing
   else yet.

4. **`[fetch]` and `[advise]` under one parser**, `slots/<slot>/` layout, the
   old paths retired. One migration, one deletion.

5. **Plugins fold into yazi's file.** Third dialect gone.

6. **#56, the agent slot**, as the first provider added under the finished
   format — which is also the test of whether it worked.

**Steps 1–3 are 0.4.0. Steps 4–6 are 0.5.0**, where the mux seam already is.

## Open questions

1. **Does `extras` become slots?** `fzf`, `ripgrep`, `fd`, `jq`, `zoxide`,
   `lazygit` fill no slot; they are a list in `config.toml`. `slots/tool/` for
   things that are not slot providers is a wart. The alternative — an `extras`
   pseudo-slot — is a different wart.

2. **Is `redirect` real yet?** Nothing consumes it until the planner. Adding a
   field with no reader invites it to be wrong for a milestone. It could wait
   for 0.6.0 without holding anything up.

3. ~~How much migration is owed?~~ **None.** `slots/` is `//go:embed`-ed, so
   no one can add a provider without forking and rebuilding: it is source code
   that happens to be TOML, and contributing a provider means opening a pull
   request. The format can change in a single commit. (`docs/adding-a-provider.md`
   is a contributor's guide, not a promise to anyone's local files.)

4. ~~Does this fit in 0.4.0?~~ **No, and it is split above.**
