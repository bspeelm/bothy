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

## ADR-009 — bothy is isolated: it brings its own config tree

This reverses the largest assumption in PLAN.md revision 1, that bothy installs
tools and writes configs onto your machine. It now writes only into
`~/.local/share/bothy/`, and launches each tool pointed there.

The original design was defensible and is implemented: every generated file
carried a header, every pre-existing file was backed up and recorded, and
`uninstall` restored them. It passed a round-trip test asserting an empty
filesystem diff. The problem was never that it did not work.

The problem was that "let me rewrite your dotfiles, I promise I backed them up"
is a large ask of someone evaluating a tool, and that several hundred lines of
manifest, hashing and restore logic existed for no reason other than that bothy
touched files it did not own. Isolation replaces a *promise* about your files
with a *property*: bothy cannot damage a config it never opens.

It also removes the writes that were hardest to justify — `~/.vimrc` and six
`git config --global` keys. A workspace tool replacing your editor config is
overreach. Nobody has a pre-existing opinion about the Zellij layout bothy
launches; plenty of people have one about their editor.

Revision 1 assumed the terminal made full isolation impossible, because Ghostty
is launched by the desktop rather than by bothy. Two things resolved that.
Ghostty honours `--config-file=`, so bothy can hand it a config of its own; and
because a named `theme =` is looked up in paths that are *not* relocatable, the
palette is written into that file directly rather than referenced. Verified:
`ghostty +validate-config --config-file=<inlined>` exits 0. Once bothy launches
the terminal it owns the whole process tree, so `PATH`, `EDITOR`,
`ZELLIJ_CONFIG_DIR` and `YAZI_CONFIG_HOME` are set for that session alone, and
the `~/.bashrc.d` fragment — the last write outside bothy's tree — goes too.

Binaries are treated as a separate axis, deliberately. Isolating configs costs
nothing; duplicating binaries costs disk. So bothy fills gaps rather than
duplicating: a tool already on `PATH` that meets the minimum version is used
as-is, and only a missing or too-old one is fetched, into bothy's own `bin/`
which is prepended to `PATH` for its session only. A filled-in tool never
shadows your everyday one. Filling in the entire toolset costs about 124 MB of
static binaries, measured on 2026-08-31 from the pins in `bothy.lock`. An
earlier version of this paragraph said 48 MB, which was true of a smaller
toolset and quietly stopped being true; the number is dated now so the next
reader knows what they are looking at.

The cost is that bothy does less. It no longer carries your `.vimrc` or your
delta wiring to a new machine — but revision 1's own rule already said those
belong to the underlying tool, not to bothy. What it does carry is
`~/.config/bothy/`: slot choices, a palette, and overrides. One directory to put
in git, which is better portability than a manifest of files scattered across a
home directory.

## ADR-010 — The source budget counts code, not comments

PLAN.md §3 caps the core source at "~5k LOC". That was enforced by counting
every line in every non-test `.go` file, and the one-command install pushed it
to 5010.

The two obvious responses were both wrong. Raising the number to 5500 keeps a
measure that was never measuring the right thing. Cutting 10 lines to pass
would, in practice, have meant cutting comments — and this project's comments
are the part worth keeping. They are where the reasons live: why Zellij 0.42
breaks image previews, why `/proc/PID/exe` fails inside a rootless container
where `cmdline` does not, why Ghostty's palette is inlined rather than named.
Deleting those to satisfy a line count would destroy the most valuable thing in
the repository in order to protect a proxy for it.

So the budget now counts what the principle says: **code**. Of 5010 lines, 900
are comments and 470 are blank; 3640 are code. That is the number capped at
5000, and it still bites — a change that adds real bulk fails, while one that
explains itself does not.

A second cap on total lines, at 7000, keeps the prose honest too. Comments
should be worth their room; an unbounded allowance would eventually mean
narration rather than reasoning.

The general point, since it applies beyond this budget: when a measure and the
thing it measures disagree, fix the measure. Do not quietly move its threshold,
and do not damage the thing to satisfy it.

## ADR-011 — Toolbx is detected by its own marker, not by having a name

`/run/.containerenv` is written by podman, not by Toolbx — it opens with the
engine that wrote it — and podman puts a `name=` line in it for every container
it runs, generating a name where none was given. bothy read that name as
evidence of Toolbx, on a comment that said "a bare podman container usually
does not [have a name]". It always does.

The cost was not theoretical. `ContainerName` feeds `ContainerFor`, so `bothy`
on the host answered a plain `podman run --name foo` with `toolbox run
--container foo`, a hop that cannot succeed. Toolbx writes `/run/.toolboxenv`;
that mark is now the only evidence read, and a named podman container is
`Generic`.

Two things about how this was found are worth keeping. A test asserted the bug
was correct — it skipped unless `/run/.containerenv` existed, which is true in
every podman container, and then asserted the container was a Toolbx. And
detection could not be tested at all without running inside the thing being
detected, which is why nobody had. `detectContainerIn` now takes a root, and a
table test covers the marker files directly. When a thing can only be tested by
being the thing, it will not be tested.

## ADR-012 — Portability is a CI claim, not a README claim

The README said bothy runs on Linux. It had only ever run on Fedora: on
Silverblue, in a Fedora toolbox, and in `fedora-toolbox:44` — three
environments and one distribution. `PLAN.md` §10 was honest about this, which
is the only reason it was fixable.

So Ubuntu support arrives as a CI job rather than as an edit to a sentence. The
job installs bothy into `fedora:44` and `ubuntu:24.04`, runs the doctor,
compares the whole report against a table of expected severities, and
uninstalls, asserting the tree is empty afterwards.

It is a Go test behind a build tag rather than a shell script, because ADR-001
keeps exactly one shell file in this repository and because the assertions are
a set comparison and a directory walk — pleasant in Go, miserable in shell. It
drives Docker rather than podman, and the container's `$HOME` is a bind-mounted
host directory, so every filesystem assertion happens on the outside where the
test can see it.

The job earned its place immediately, in two ways. It found that
`install.SessionEnv` set no XDG directories, so `yazi --clear-cache` and
`zellij setup --check` — both run by the doctor, through that same environment
— wrote outside bothy's tree. ADR-009 held for `install` and leaked at every
other command. `plugins.go` had already described that gap and concluded it
"has to be closed one subprocess at a time"; it does not, because the tools
agree on where to look, so three environment variables close it for all of them
at once, including the ones added later.

And on its first run it was green while its test was red: `go test | tee`
reports `tee`'s exit code, and pipefail is not on by default in a GitHub
Actions step. A job written to cite the "green and testing nothing" bug
committed it on its first attempt. The lesson is not that pipefail is easy to
forget. It is that a passing job is evidence of nothing until you have watched
it fail.

## ADR-013 — The Debian package is a file, not a repository

Fedora gets a Copr repository and Debian gets a `.deb` attached to the GitHub
release. The asymmetry is deliberate, and what was rejected to reach it is the
part worth writing down.

A Launchpad PPA is the obvious answer and it is the wrong shape. It builds from
a Debian source package, which means carrying `debian/control`, `debian/rules`
and a `debian/changelog` in a second packaging syntax alongside the rpm spec — a
second place for the version to live, when `make release-tag` and `make copr` go
to some trouble to have only one. It wants a series per Ubuntu release, so the
matrix grows every six months whether or not anything changed. And it is Ubuntu
only, so Debian and Mint would still be downloading a file.

Hosting an apt repository directly is worse. It needs a GPG key that has to be
kept, kept secret and kept alive, and a signed `Release` file that carries an
expiry date. The failure mode is therefore not "no new version" but "every
existing user's `apt update` begins erroring on a date I have forgotten about",
which is principle 7 exactly: an automated path whose failure is silent until it
is someone else's problem. Copr is tolerable because Fedora runs it. The Debian
equivalent is a thing I would be running.

So the `.deb` comes out of goreleaser's `nfpms:` block, built from the binary of
the same tag, and lands on the release page beside the tarballs. It costs one
block of configuration and no ongoing anything.

The cost is real and belongs in the open rather than in a footnote: **`apt
upgrade` will never bring you a new bothy.** dpkg tracks it and `apt remove`
removes it, but there is no source for apt to learn a newer one exists, so
upgrading means fetching the next file by hand. Anyone who wants updates to
arrive on their own should use the install script, which resolves
`/releases/latest/` on every run. The `.deb` is for people who would rather their
package manager knew the binary was there — the same wish the Copr serves — and
it delivers that much and no more.

Two smaller decisions follow from the same place. The `.deb` is built from the
release binary rather than from source, which is the opposite of what
`packaging/bothy.spec` does and is defensible only because this one is not
claiming to be a distribution's package: it is the tarball wearing a `.deb`, so
that dpkg has a record of the file it put in `/usr/bin`. And `nfpms:` emits `deb`
and nothing else. It would have taken one word to emit an rpm too, and the result
would have been two packages named `bothy`, built from different inputs by
different systems, with nothing to say which one a machine ought to have.

## ADR-014 — bothy installs no editor, and helix could not be installed anyway

The swap table offers vim, nano and helix, and bothy supplies none of them.
That is deliberate, and it is worth separating from a second fact that looks
like the same thing.

**The deliberate part.** An editor is the most personal tool in the workspace
and the one a person is likeliest to already have, configured the way they
like. ADR-009 already removed bothy's `~/.vimrc` write for that reason —
"a workspace tool replacing your editor config is overreach". Installing the
editor itself is the same overreach one step earlier. So the editor slot names
a command and sets `EDITOR` for bothy's session, and nothing else.

**The accidental part.** helix publishes only `tar.xz`. The standard library
has no xz decompressor and PLAN.md §13 caps dependencies at go-toml, so
`Extract` refuses that format by name. If the decision above were ever
reversed, helix would still be the one editor that could not be supplied.

Taking `ulikunitz/xz` — small, pure Go — was considered and rejected. Not on
its merits, which are fine, but because the ceiling has held precisely because
every exception was refused, and a dependency admitted for one tool's archive
format is admitted forever. This ADR is where the evidence accumulates if
helix, or a second tar.xz tool, turns out to matter to real users.

What actually needed fixing was neither of those. There was no editor check at
all: a configured editor that was not installed produced a pane running a
missing command, an `EDITOR` pointing at nothing, and a doctor reporting
sixteen passes and no failures. The slot was entirely a claim about the
machine, which is the one kind of thing this project's doctor exists to check.
It is checked now, with per-distribution advice in `slots/advice/`, the same
shape Ghostty and the agent already use.
