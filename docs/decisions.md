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

## ADR-006 — One palette ships; every other one is a file you supply

bothy carries exactly one palette: the freely-licensed open Dracula one, in
`internal/theme/theme.go`. Any other palette — a commercial one you have
licensed, or Catppuccin, or your own — arrives as a small TOML file of eleven
colours that you point `theme.palette` at. bothy reads it at install time and
themes the whole workspace from it, including generating a vim colorscheme.

An earlier design went further: it parsed a particular vendor's pack, extracted
their palette, and copied their theme files into place. That worked, and it was
the wrong shape. It put knowledge of one commercial product into the core, it
generated config files full of that product's values, and — most tellingly — the
test written to forbid those values had to list them, so the guard against
copying the palette *was* a copy of the palette.

The rule now is simply: **the only colour values in this repository are open
Dracula's**, and `TestOnlyOpenDraculaColoursAreShipped` enforces it by inverting
the check. It scans every shipped file and fails on any colour that is not part
of the built-in palette, so it needs to name nothing it forbids and catches any
stray palette rather than one vendor's.

The side effect is the good kind: a palette you have licensed stays entirely on
your machine, and the way in for it is the same route every other theme uses.
`bothy theme example` prints the blank file to fill in.

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

## ADR-008 — The launch verb is `bothy`, not `dev`

This reverses PLAN.md's original position, which held that `dev` was the brand
moment and must never change. Three things argued against it.

`dev` is a very common name to claim in someone's shell. It is plausibly already
a script, an alias, or a function in the dotfiles of exactly the people most
likely to want this — and silently shadowing it is the kind of surprise bothy
exists to avoid.

It also required a shell function to exist, which meant the launcher behaved
differently depending on whether your shell had been reloaded, and had its own
host-versus-container branch that duplicated logic the binary already had. The
binary can hop into a container; a shell function that has to decide whether it
is inside one is a second implementation of the same decision, free to drift.

And a workspace launched by `dev` gives no clue what launched it. `bothy` names
itself, which matters when the thing you are debugging is your own setup.

So: bare `bothy` launches the workspace, `bothy attach` reattaches, and
`bothy dev` is retained as an alias for anyone who has it in muscle memory. The
shell fragment still sets `EDITOR` and trims the prompt, because those are
environment, not a launcher.
