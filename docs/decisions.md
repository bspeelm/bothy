# Decisions

Architecture decision records. One paragraph each. Add an entry whenever a
principle in `PLAN.md` §0 is bent, and say why.

---

## ADR-001 — The core is Go

Single static binary, trivial cross-compilation to linux/darwin/windows ×
amd64/arm64, no runtime to install first, fast startup, boring. Rust would fit
the Zellij/Yazi ecosystem better culturally, but this codebase is glue, not a
systems tool, and Go's cross-compile story is simpler. **Shell is explicitly
rejected for the core** — it is unmaintainable across three operating systems
and two shell languages. The only shell in the repo is `bootstrap/install.sh`,
which is deliberately small enough to read in one screen.

## ADR-002 — User-space install is the default

Pinned release binaries land in `~/.local/bin`; configs land in XDG paths. No
root, no host modification, works unchanged on immutable distros (Silverblue,
Kinoite, Bazzite) and inside Toolbx/Distrobox containers. `--system` is an
opt-in that hands the work to the native package manager, and it is the only
mode that can hit the known-bad-repository problems the doctor checks for.

## ADR-003 — Zellij is the only multiplexer

tmux support would roughly double the layout renderer and its test matrix for a
second dialect of the same idea. Zellij's KDL layouts, plugin panes, and
built-in status bar are what make a declarative profile possible at all. tmux is
a documented non-goal for v1, not a rejection forever.

## ADR-004 — Windows means WSL2

Zellij and Yazi's plugin ecosystem are Unix-native. Native Windows without WSL is
a documented non-goal that will not change. The Windows story is: install
Windows Terminal, install WSL2, hand off to the Linux installer inside it.

## ADR-005 — No plugin system

bothy has slots and providers, and a provider is a declarative TOML file plus
templates. That is the entire extension surface. No runtime plugin loading, no
marketplace, no extension API. If adding a provider requires touching Go code,
the slot model is wrong and that is the bug to fix.

## ADR-006 — Dracula PRO is supported by reference, never redistributed

Dracula PRO is a paid product. bothy ships the **open** Dracula palette (MIT) as
its default and baked-in data. Selecting `theme.variant = "pro"` requires the
user to point `theme.pro_pack` at their own licensed copy of the pack; bothy
then parses `design/palette.md` out of that pack for the colours it needs, and
copies the pack's own ready-made Ghostty theme and vim colorschemes into place
verbatim. No PRO hex value, colorscheme, or font is stored in this repository,
and a test asserts that (`git grep` for the PRO background must return nothing
outside test fixtures). The same rule applies to any future paid theme: bothy
can *use* what a user already owns, and ships none of it.

## ADR-007 — Version-gated workarounds, not permanent ones

The origin cheat sheet disables Yazi image previews inside Zellij, because
Zellij 0.42 could not pass the Kitty graphics protocol and its mangled query
replies were parsed as keystrokes (a phantom "Find next" on every preview).
Zellij 0.45.0 implemented Kitty graphics, and 0.45.1 fixed image sizing and
stopped advertising Sixel support the terminal does not have. So the workaround
is now gated on a detected version plus a runtime probe rather than applied
unconditionally: new installs get real image previews, older Zellij still gets
the `enter-hint` placeholder, and `bothy doctor` reports which path is active
and why. Deleting the workaround outright would have thrown away the knowledge;
keeping it unconditionally would have shipped a permanent regression. This
pattern — gate, probe, explain — is how every workaround inherited from the
cheat sheet should age.
