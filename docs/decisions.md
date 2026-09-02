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

**Superseded by ADR-018.** The cost stands; the refusal does not.

tmux support would roughly double the layout renderer and its test matrix for a
second dialect of the same idea. Zellij's KDL layouts, plugin panes, and
built-in status bar are what make a declarative profile possible at all. tmux is
a documented non-goal for v1, not a rejection forever.

## ADR-004 — Windows means WSL2

**Superseded by ADR-018.** The cost stands; the refusal does not.

Zellij and Yazi's plugin ecosystem are Unix-native. Native Windows without WSL is
a documented non-goal that will not change. The Windows story is: install
Windows Terminal, install WSL2, hand off to the Linux installer inside it.

## ADR-005 — No plugin system

**Amended by ADR-019**, which names the three tiers this rule has always had.

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
`bothy dev` is retained as an alias for anyone who has it in muscle memory.
(**The alias is removed by ADR-024**; the verb's replacement by bare `bothy`
stands.) The
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
static binaries on disk, from roughly 131 MB of downloaded archives — measured
2026-08-31 against the pins in `bothy.lock`. Both numbers are dated because an
undated measurement is one nobody knows to re-take; an earlier version of this
paragraph said 48 MB long after that had stopped being true.

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
narration rather than reasoning. (**That second cap is replaced by ADR-021**,
which caps comments as a share of code. The reason above is unchanged; the
measure was wrong.)

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

## ADR-015 — Comments record intent; history lives in git and here

ADR-010 argued that this project's comments are "where the reasons live" and
must not be trimmed to satisfy a line count. That was right, and it was applied
too broadly. Reasons are worth their room. Narrative is not, and about a
quarter of the source had become narrative: what a function used to do, how a
bug was found, how many times some mistake had recurred.

The distinction that matters is not length. It is whether a comment would still
make sense to somebody who had never seen the previous version of the code.

> Zellij below 0.45.0 cannot pass the Kitty graphics protocol through, and its
> mangled reply to Yazi's capability query is parsed as keystrokes.

reads the same to a newcomer as to the person who debugged it, and it is the
only thing standing between a future reader and deleting the version gate.
Whereas

> An earlier version returned early when offline and never recorded where the
> install happened … and this is the third short-circuit in this project to
> swallow a step someone added after it.

says one useful thing — the manifest is written on every path because the
launcher needs it — wrapped in three sentences of autobiography. The useful
sentence survives; the rest belongs to `git log`.

Two forces produced the drift, and both are worth naming. Writing a comment at
the moment of fixing a bug makes the bug feel like the context, when the
context is actually the constraint that made the bug possible. And ADR-010's
separate line budget for prose, introduced to protect reasons, equally
protected narration — a ceiling that permits growth is not a check on it.

So: the rule is written down in `CLAUDE.md` where it will be read before code
is written rather than after, and the total-line budget is lowered so that it
binds. A bug still ships with the test or doctor check that catches it, and the
test's name is where "this broke once" belongs.

This is not a licence to write terse code. Every comment ADR-010 was defending
is still here.

## ADR-016 — The agent is the point

The README described bothy as "a turn-key terminal workspace" and listed the
agent among its components as **Optional**. The default profile's own first
line said something else:

    # cockpit — the default: supervising an agent working on a repo.

Both could not be the intent, and the disagreement was not cosmetic. It decided
questions that were sitting open: whether delta earns its place or the side pane
should become lazygit so a diff is one keypress away (#45), and whether agent
definitions are worth moving out of a Go switch (#56). If the agent is one
component among five, those are arbitrary. If it is the point, they are obvious.

So: **the agent is the point.** bothy is a cockpit for working with a coding
agent on a repository — the file browser is there to show what changed, the
shell to check it, and the whole tree is disposable so the thing can be tried
on a machine nobody wants to keep it on.

This is a claim about *shape*, not about requirements. `slots.agent none` still
produces a working workspace, bothy still installs no agent, and PLAN.md §11's
non-goal — bothy does not manage the agent's config, keys or hooks — is
untouched. What changes is which questions have easy answers.

**And it constrains how the isolation property may be described.** The first
draft of this reframing put "leaves no trace" in the tagline, which is true of
bothy — one directory, removed by one command — and reads, beside the word
*agent*, as a claim about blast radius. It is not one. The agent runs as the
user, in the user's repository, with the user's permissions; its edits and
commits are real, and its own state lives in its own directory that bothy does
not touch by policy. Borrowing the connotations of a sandbox while declining to
be one is the kind of overclaim this project exists to avoid, so "not a
sandbox" is on the What-bothy-is-not list in as many words.

The tagline that survived does both jobs in two sentences:

> A room you can trust because it wants nothing.
> The agent, of course, is another matter.

The first line is §11 in three words — no telemetry, no account, no
auto-updater, no marketplace, no claim on your dotfiles or your agent's
credentials. The second names the agent, which is what this ADR asked the
stated intent to do, and declines responsibility for it in the same breath.

That second sentence is the whole not-a-sandbox point, delivered as a wink
rather than a disclaimer, and it is better for being funny: a caveat someone
enjoys reading is a caveat they finish. The plain statement stays on the
What-bothy-is-not list, because a joke is not a substitute for saying it once
without one.

The alternative was to accept "terminal workspace" and rename `cockpit`, which
is the honest version of the other choice. It was not taken because the
narrower claim is the one nobody else makes. "One command opens yazi, zellij and
an agent" is a layout file anyone can write; a disposable, verified box for
watching an agent work is not, and every component already present was chosen
for that job.

The cost is a smaller audience by description. Someone wanting a terminal
workspace with no agent will read the first line and leave, and that is the
correct outcome — they were going to write the layout file themselves.

## ADR-017 — The invariant is three panes; everything else is a provider

bothy produces one arrangement: files above, agent below left, shell below
right. Which terminal draws it, which multiplexer splits it, which browser fills
the top pane and which agent runs in the middle are providers of slots, and the
product is that the arrangement survives whichever ones you already have.

So the invariant splits into capabilities — **panes**, **sessions**, **images**,
**theme**, **isolation** — and every stack is described by which of them it can
supply. `panes` is mandatory; a stack that cannot give you three panes is not a
bothy stack. The other four are reported, per stack, before anything is
installed. This is ADR-007's *gate, probe, explain* raised from one workaround to
the whole product: a stack that cannot draw images gets told so in the plan
rather than at launch.

The alternative was to keep one blessed stack and treat everything else as
unsupported. It was not taken because the interesting claim is not that Ghostty
plus Zellij plus Yazi works — that is a layout file — but that the same three
panes can be assembled from what is already on the machine, with an honest
account of what is lost.

This does not reverse ADR-016. That record narrowed what bothy is *for*, at a
deliberate cost in audience; this one widens what it runs *on*. Narrow about the
purpose, broad about the substrate. The purpose is what makes the arrangement
worth reproducing anywhere.

`docs/north-star.md` carries the long form.

## ADR-018 — Supersedes ADR-003 and ADR-004: scope is a CI table, not a refusal

ADR-003 made Zellij the only multiplexer and ADR-004 made native Windows a
non-goal "that will not change". Both stated a scope by refusing something, and
the README's "What bothy is not" list carried the same two lines.

ADR-012 already decided where scope belongs: *portability is a CI claim, not a
README claim*. A refusal and a tested-stack table answer the same question, and
only one of them can be checked. Where they disagree — and they would, the first
time a second backend became cheap — the refusal is the half that goes stale
silently.

So the two lines come off the not-list, ADR-003's and ADR-004's refusals are
withdrawn, and what bothy runs on is stated by the stacks CI tests. tmux and
native Windows are then neither promised nor refused: they are absent from the
table, which is exactly what is true about them.

The reasons those two records gave were sound and are kept as costs rather than
as prohibitions. tmux is a second renderer of the same profile, and the estimate
that it "would roughly double the layout renderer" turns out to be low: the
multiplexer is seven decisions spread across the tree, not one package. Native
Windows needs a second bootstrap in a second language, which ADR-001 permits
exactly once and spends on `bootstrap/install.sh`. Both are worth doing when
someone wants them and neither is worth doing on speculation.

What replaces the refusal is a promise that is harder to keep and possible to
verify: for any stack, bothy says what the invariant can and cannot deliver on
it, whether or not that stack is one CI has seen.

## ADR-019 — Amends ADR-005: three provider tiers, stated

ADR-005 says the extension surface is slots and providers, and that a provider
needing Go code means the slot model is wrong. The rule is right about direction
and has never been literally true. `docs/adding-a-provider.md` states it and then
shows the Go snippet a configuration provider needs; `EditorBinary` and
`agentBinary` are hardcoded maps duplicating an `advice.binary` field that is
parsed and never read, with a third copy inlined in a doctor check.

Naming the tiers is more useful than a rule with undocumented exceptions:

- **Data.** One TOML file. Every tool bothy fetches.
- **Data and a branch.** A file, templates, and one arm in `install.plan()`.
- **Data and a renderer.** Go that *interprets* the profile rather than being
  configured by it. The multiplexer only.

The third tier exists because Zellij takes a KDL layout file, tmux takes
`split-window` commands and Windows Terminal takes `wt` arguments. Those are
renderers of one profile, not templates of one thing, and no amount of data
makes them one.

The rule ADR-005 was reaching for survives as a direction: the second tier
should shrink as the first grows, and a provider that lands in the third tier
without being a multiplexer is still the bug ADR-005 described.

## ADR-020 — delta is dropped, and the side pane stays a shell

delta was downloaded on every install because the setup bothy ports wired it in
as git's pager. ADR-009 stopped bothy writing to `~/.gitconfig`, so the wiring
went and the tool stayed: 7 MB fetched, pinned and version-checked for a
feature that no longer exists. Nothing bothy generates has referred to it since.

The alternative was to give it a job. ADR-016 raised it as one of the questions
"the agent is the point" settles — make the side pane optionally lazygit, which
reads delta as its pager, and a diff is one keypress from the agent that caused
it. That is a good argument and it is not the one taken, for two reasons that
both postdate it.

ADR-017 names the third pane: *a shell below right, because you will want to
run something without leaving*. A side pane that is lazygit is a cockpit
without a shell, which is a weaker arrangement than the one the invariant
describes, not a richer one.

And `PLAN.md`'s non-goal is specifically that lazygit is one keypress away *in
the shell pane*. That is the review mode. Replacing the shell with lazygit
removes the thing that argument rests on, so bothy would have to choose between
a shell and a diff where today it has a shell that can produce either.

So delta leaves entirely — `DefaultExtras`, `slots/tools/`, and `bothy.lock` —
rather than becoming an optional download nobody would find. It is one `dnf
install git-delta` away, and it only ever did anything once its owner had wired
it into a file bothy does not write. The cockpit is unchanged: three panes, and
the third is a shell.

## ADR-021 — The prose budget is a ratio, not a total

ADR-010 capped code at 5,000 lines and added a second cap on *total* lines to
keep the prose honest. The reason for the second cap was right and the measure
was wrong, in the way that record itself describes.

Total lines are code plus comments plus blanks, and code already has its own
cap. So the total cap never bounded prose: it bounded prose *plus* code, and
every line of legitimate functionality shrank the prose allowance by one.
Between ADR-010 and this one, code grew by about 1,250 lines and consumed the
entire margin, while comments stayed at roughly the same share of the codebase
they had always been — 21% then, 22% now. Nothing about the prose had changed,
and the prose budget failed.

The measure that matches the principle is proportion. Comments are capped at
**25% of code**, which is the question actually worth asking: is the
explanation proportionate to the thing being explained? A file that doubles in
size may honestly need twice the reasoning; one that adds a paragraph of
narration to an unchanged function may not, and the ratio notices.

This does not loosen anything. At the moment of the change the ratio stood at
22% against a 25% cap, roughly the same headroom the total cap had when it was
set — and unlike the total, it does not quietly tighten every time a feature
lands.

ADR-010's general point is the reason this record exists rather than a raised
threshold: when a measure and the thing it measures disagree, fix the measure.
It is a good rule and it caught its own second cap.

## ADR-022 — bothy redirects caches, and names what the tools keep

0.1.4 pointed `XDG_CACHE_HOME`, `XDG_STATE_HOME` and `XDG_DATA_HOME` into
bothy's tree for every subprocess. It was written to close a real leak: the
doctor runs `yazi --clear-cache` and `zellij setup --check`, both of which
wrote outside the tree, so ADR-009 held for install and quietly failed at every
other command.

It closed that leak and opened a larger one. Neovim keeps its plugins in
`$XDG_DATA_HOME/nvim`, zoxide keeps the directory database it has learned from
you, lazygit keeps its state — and inside bothy every one of those tools found
an empty directory instead. The workspace exists to run your tools, and it was
running them with their memory removed. "Your editor is yours" cannot survive
bothy pointing your editor somewhere your editor has never been.

The distinction the original change missed is between a cache and everything
else. **A cache is the tool's own scratch space**: losing it costs a rebuild
and nothing more, so keeping it inside bothy's tree makes uninstall complete
without taking anything from anyone. It is also what keeps `ya pkg`'s clone of
the plugin repository somewhere uninstall can reach. **Data and state are
yours**, and moving them is not isolation but amnesia.

So `XDG_CACHE_HOME` stays and the other two go, and `bothy doctor` reports
which tool directories live outside the tree and that uninstall will not remove
them. The property is weaker and true, which beats stronger and false: bothy
writes nothing outside its own tree, and names what the programs it starts
write outside theirs.

The container job's end-to-end assertion changes shape with it. It cannot
assert that nothing exists outside bothy's tree any more, so it asserts that
anything which does is named after a tool bothy runs — matching the tool's own
name rather than allowing `~/.local/share` wholesale, because a rule that
permits a directory permits everything that lands in it.

## ADR-023 — Panes navigate independently, and the room does not move

Every pane starts where `bothy` was launched, because the rendered layout sets
no `cwd` on any of them and Zellij inherits the directory it was started in.
After that they are independent: the browser can wander off, the side shell can
`cd` elsewhere, and the agent stays where it began. That independence is the
point — the side pane exists so you can look at something the agent is not
looking at.

A `bothy cd` was proposed to re-home all three at once, for the afternoon when
you switch projects. It is not being built, and the reason is that the problem
it was filed against no longer exists.

The motivation was that changing project meant "tearing down a session that had
state in it — agent context, a lazygit view, a shell history". Since sessions
were named after their directory, it does not. `cd ~/other && bothy` opens a
second room; the first keeps running, detached, with its conversation intact,
and `bothy attach` returns to it. Nothing is torn down, so nothing needs
re-homing.

Two things would also have been wrong with it. **The agent cannot move.** Claude
Code and most others pin their working directory at startup, so "move the room"
is really "move the two panes that matter least and restart the one that
matters most" — and ADR-016 says the agent is the point. **And the session name
would start lying.** A room named `bothy-thisproject` sitting in another
directory makes `bothy ls` wrong and makes a later `bothy` in that directory
open a third session rather than find the second.

What was missing was not a command but a sentence, so the README now says how
to keep several rooms and move between them.

The line that stays: implicit navigation moves one pane, and nothing moves all
of them. A hook that pushed every browser `cd` to the other panes would make
the browser useless for browsing, which is the design this record exists to
rule out.

## ADR-024 — The cuts 1.0 planned, taken early

`docs/plan-1.0.md` listed four things to remove before 1.0, on the grounds that
each is something a newcomer has to read past. They are taken now instead,
because the code budget wanted the room and because a deletion decided a month
ago is not improved by waiting.

**The watermark.** A Ghostty background-image trick, off by default, needing
per-layout measuring to look right. It cost a config key, three fields on the
template data, a branch in `plan`, a doctor check, an embedded PNG and a page
of documentation — 34 lines of code for a thing whose own config comment
conceded it was a nice touch rather than a feature.

(**Reversed in part by ADR-025.** The feature comes back; the shipped picture
does not, which is where the weight was.)

**`bothy lock` leaves the public help.** It stays a command and stops being
advertised: it downloads half a gigabyte to recompute checksums, which is a
maintainer's business and a surprising thing to offer everyone who types
`bothy help`.

**The plan documents move to `docs/history/`.** Three of the eight files in
`docs/` were the plans for releases that already shipped. `plan-1.0.md` stays
where it is, because it is the road right up until it has been travelled.

**And `bothy dev` goes, which reverses ADR-008.** That record kept the verb as
an alias "for anyone who has it in muscle memory", and the muscle memory it
was protecting belonged to people who had the old shell function — a group
that has not grown since.

Removing it turned out not to save anything, and that is the interesting part.
The verb was the only way to reach the launcher's flags: `bothy --in-place`
answered "unknown command", so the alias was load-bearing for anyone who
wanted anything other than the default. main now routes a leading flag to the
launcher, which costs slightly more than the alias did and fixes a gap nobody
had filed. The cut paid for itself in the wrong currency.

## ADR-025 — The watermark stays; the picture bothy shipped does not

ADR-024 cut the watermark among the things 1.0 planned to remove. That was
wrong about the feature and right about its weight, and the two can be
separated.

What made it expensive was not the idea but the asset. bothy shipped a
24 KB PNG of Tux, pre-composited for a 1920×1080 screen at a position measured
from one person's monitor, behind a boolean that turned it on. That picture is
wrong on most screens, wrong for anyone whose layout differs, and — a thing
nobody had noticed — it is Larry Ewing's, redistributed with no credit anywhere
in `NOTICE`.

So `workspace.watermark` becomes a path to art of your own, and bothy ships
none. The template gains the image when the key is set and says nothing when it
is not. That is fewer moving parts than the boolean was: no embedded asset, no
copy step in `plan`, no backup of a file bothy put there.

The opacity is deliberately not a second key. Every other Ghostty setting is
tuned by writing it into `~/.config/bothy/overrides/ghostty/`, which is
appended after bothy's config and wins; a setting that already has a mechanism
does not need a key of its own.

The doctor check survives unchanged in purpose, and the purpose is worth
restating because it is not obvious: Ghostty says nothing at all about a
`background-image` it cannot find. It draws nothing, which looks exactly like
an opacity set too low, and sends you tuning a setting that was never the
problem.

`docs/watermark.md` now carries the compositing recipe rather than describing
a file bothy installs — including the arithmetic, because the useful part was
never the picture but the trick of making a window-shaped canvas put art where
a pane will be.

## ADR-027 — A config bothy cannot read is a warning, never a refusal

0.1.5 decided that an unknown key in `config.toml` is a warning: a config that
refuses to load is worse than a typo, and one written by a newer bothy must not
brick an older one. `Load`'s comment says exactly that.

It handled unknown *names* and not unknown *types*, and 0.3.0 found the gap the
expensive way. `workspace.watermark` changed from a boolean to a path; `Save`
writes the whole struct, so every config bothy had ever written carried
`watermark = false`; and go-toml refuses a boolean for a string field with a
hard error. `Load` returned it. **Every command failed for every existing
user, including the `config` that would have repaired it and the `doctor` that
would have explained it.** It shipped, and was found by running the next
feature against an ordinary machine.

Two things change. The key is renamed to `background_image`, which it should
have been anyway — named for what it is rather than what it is for — and the
rename is what makes an old config meet the unknown-key path it already
survives.

And the general case is closed: a value bothy cannot read is dropped, named,
and the file loads without it. The decoder stops at the first such key, so it
is removed and the decode retried; otherwise every key after it would silently
keep its default, which is a worse failure than the error it replaced. go-toml
reports a row for a type error and leaves the key name empty, so the line is
what there is to go on, and only a cut that leaves valid TOML is accepted — a
half-removed multi-line value would turn one bad key into a broken file.

The two are reported differently, because "did you mean `background_image`?"
about `background_image` helps nobody.

What this does not tolerate is a file that is not TOML. Reading a key bothy
does not understand is a compatibility question; a syntax error is a mistake
the author needs to hear about.

The wider lesson is about the shape of the escape hatch rather than about
TOML. A tolerance written for one failure mode will be read later as covering
the category, by someone who checks the comment and not the code — and the
comment here was right about the principle and silent about the case.

## ADR-026 — The code budget rises once, to 6,000

ADR-010 capped code at 5,000 lines when there were 3,640 of them, and said the
general thing: when a measure and the thing it measures disagree, fix the
measure; do not quietly move its threshold.

They do not disagree here. The code cap counts code, code is what has grown,
and it has grown for reasons anyone can inspect: three platforms where there
was one, named sessions, tool upgrades against the lockfile, pinned plugins, a
provider format that reads from data, an end-to-end job for macOS. Nothing in
that list is bulk. So this is a threshold move, which is the case ADR-010 asks
to be argued rather than performed — and the argument is not that 5,000 was
wrong. It was right, and it has been spent.

**Why 6,000 and not 5,500.** The road has three things that will each need
room and are already decided: platform-specific code organised deliberately
rather than by accident (#76), the provider format that lets a slot be added
without Go (#69), and the multiplexer backend seam (#64). A cap that has to be
raised again in two milestones is not a cap, it is a recurring negotiation.
6,000 is chosen to be argued once.

**What it costs.** The budget bites less often, and that is the point of a
budget. Two things keep it honest anyway. The comment ratio from ADR-021 bounds
prose independently, so the code cap no longer has to do both jobs at once —
which is what made the old total tighten every time a feature landed. And the
binary cap is unmoved at 10 MB, which is the limit a user can actually feel.

**What was tried first.** Everything cheap. #72 read a provider's command from
its own file and deleted three copies of the same map; the cuts 1.0 had already
planned were taken early (ADR-024); a dead `ShareDir` went. Together they
returned about thirty lines. A survey for more found one duplicated three-line
helper, deliberately left duplicated because unifying it across a package
boundary is worse Go, and no function longer than eighty lines including its
comments. There is no fat. That finding is what makes this a threshold move
rather than an excuse for one.

## ADR-028 — A provider says what it is, and only a declared name can be wrong

A provider file said how to get a program and nothing about what the program
was. `slots/tools/yazi.toml` gave a repository, a minimum version and a table of
release assets; nothing in it said yazi was the file browser. Slot membership
lived in Go instead, so the data and the code could not check each other, and
this was accepted in silence:

```
$ bothy config set slots.mux yazi
$ bothy config set slots.browser zellij
$ bothy config set slots.agent ghostty
```

Three commands and a workspace that cannot open. `config.allowed` closes the
value set of `workspace.pane_frames` and `workspace.launch` and of nothing else,
because for a slot **there was nothing to be closed against**.

**Every provider now carries a header** — `slot`, `what`, `platforms`,
`provides` — in both dialects, joined by `internal/slots`.

**Only a name bothy has a file for can be wrong.** The rule is not "the value
must be a known provider"; it is "a known provider must agree". `slots.agent`
takes any command you care to name and `slots.terminal` names emulators bothy
ships no file for, and both keep working. This is the difference between a check
that catches a contradiction and a whitelist that decides for you.

**Every field has a reader, and two of those readers are tests.** A field
nothing reads is a field nobody notices is wrong, which is why `redirect` was
left out of this format when it was proposed. `slot` is read by the check above
and by `tools.Required`, which stops naming mux and browser positionally.
`provides` is read by `bothy doctor`: a capability nothing in the stack
contributes to is reported as unavailable rather than as unverified — asked
*before* the check results, because the graphics check reads the emulator bothy
is running in rather than the one the config names, and so passes on a stack
that has nothing to do the work. `what` is read by `bothy tools`. `platforms`
has no behavioural reader yet and is held to `[assets]` by a test; that test is
what makes the restatement safe to carry until the planner (#74) reads it.

**What this does not do.** `install.plan()` still learns each provider from an
`if`. Turning those four branches into a loop needs per-provider file lists, and
the four providers it writes configs for split two-and-two across the dialects:
zellij and yazi are tools, vim and ghostty are advice, because bothy installs no
editor (ADR-014) and Ghostty publishes no binaries. There is no dialect that can
hold file lists for all four until the layout move, so that is #115 and it is
0.5.0. Declaring the slot and then having `plan()` ask the declaration instead of
the config restates the branch without removing it, and buys nothing.

## ADR-029 — The comment ratio tightens to 22%, once

ADR-021 set the ratio at 25% when comments stood at 22%, matching the headroom
the total cap had when it was set. That headroom was then spent, and spent
badly: the comments that filled it were not reasoning, they were retelling.
A seventeen-line paragraph above a single `env.set`. A sixteen-line one above
an `if`. Package docs recounting policy the ADRs already hold, in full, again.

Cutting them took no code with it — 204 lines out, 25% to 22%, every operative
constraint kept along with the ADR or issue carrying its argument. That is the
finding that justifies the move: the budget was not tight, the prose was loose,
and a cap nothing has pressed against is not enforcing a norm.

**Why 22 and not 20.** 22 is where the codebase sits after an honest pass, so
it is a measurement rather than a target. 20 would demand a second pass that
starts cutting reasoning rather than narration, which is the failure the ratio
exists to prevent in the other direction.

**What it costs.** About forty lines of headroom at today's size, growing with
code as a ratio does. That is deliberate: ADR-021's point was that prose should
stay proportionate to what it explains, and a cap the author never meets does
not ask the question. The next comment that does not fit is a prompt to check
whether it is reasoning or a story, which is exactly CLAUDE.md's own test.

**This is the tightening, not a habit of them.** ADR-010's rule cuts both ways:
a threshold moved whenever it is inconvenient is not a threshold. It moved up
once for code, argued in ADR-026, and down once for prose, argued here.

## ADR-030 — Releases are signed by the workflow that built them

`install.sh` checked a SHA-256 from `checksums.txt` and said plainly what that
proved: the bytes match what the release page published. It could not prove who
published them, because whoever could swap the archive could swap the checksum
beside it. A compromised account defeated the check completely, and every
install still said "checksum verified".

Every release artifact is now signed by `actions/attest-build-provenance` in the
workflow that built it — a Sigstore certificate bound to this repository and
workflow, which the repository itself cannot mint.

**Everything the release page offers, not just the archives.** The tarballs, the
`.deb`, and `checksums.txt`. Signing the archives alone would leave the file the
*default* install trusts swappable by exactly the person who could swap the
artifact it vouches for, which is the hole this record exists to close.

**Keyless, over a personal key.** minisign or signify would be smaller, but they
make key custody a person's problem and rebuild the bootstrap question one layer
up: the public key has to live somewhere other than this repository, or it is
the checksum situation again. CI attestation has no private key to store,
rotate, or leak.

**Verification is opt-in, and that is forced rather than chosen.** `gh
attestation verify` asks the GitHub API for the attestation, and asking needs a
login — measured, not assumed: unauthenticated it answers *"To get started with
GitHub CLI, please run: gh auth login"*. An installer that fails on a machine
merely because `gh` is present would be worse than one that says what it did not
check. So the default path is unchanged and names `--verify`, and `--verify`
fails loudly rather than degrading: no verifier, no bundle, or a bad signature
are each an error, never a quieter level of success.

**The bundle is a release asset**, which is what makes the opt-in path usable
without a GitHub account: `--bundle` reads the attestation from disk instead of
from the API.

**No doctor check.** A binary on disk may have arrived by rpm, deb, `go
install`, or `make install-binary`, so a check answering "cannot verify" on most
install paths is noise wearing a security badge, and it would make `gh` a
dependency the doctor otherwise never has. Provenance is an install-time
question and is answered there.

**All five install paths, since signing one of them is not the question.**
Three mechanisms, verified rather than assumed:

| path | signed by | checked by |
|---|---|---|
| script | this attestation | you, `--verify` |
| `.deb` | this attestation | you, `gh attestation verify --bundle` |
| dnf | Copr's per-project key | dnf, `gpgcheck=1` in the repo file |
| `go install` | the module checksum database | go, against `sum.golang.org` |
| source | nothing | — |

dnf and Go were already covered, and by better mechanisms than a README claim:
both check without being asked. The two paths this record adds are the two that
had a checksum or nothing.

**What it does not close, stated so it is not found later.** `install.sh` is
itself fetched unsigned over `curl`, and a source build trusts the clone: no
signature on an artifact can fix verifying the verifier, which is a property of
`curl | sh` and of `git clone` rather than an oversight. And the eight tools
bothy fetches are other projects' releases, pinned by checksum in `bothy.lock`
and trust-on-first-use by construction — bothy cannot sign what it did not
build. Those are different problems, and neither is this one.

## ADR-031 — Platform differences are injected, and build tags need the compiler's permission

Three platforms are on the road and nothing said whether bothy stays one source
tree with runtime branching or splits into `//go:build` files. Deciding after
the divergence exists means converting it, and the mux seam (#64) is where a
platform-specific implementation first has somewhere to plug in.

**The rule: inject what differs, and reach for a build tag only when the code
cannot compile elsewhere.** The test is not "does this make sense on Windows"
but "does the compiler reject it there". A tmux backend compiles everywhere and
is simply not selected; `.desktop` entries compile on macOS and are merely
meaningless, which is why ADR-018's guard is a runtime check. Neither is a build
tag. An `ioctl` naming `TIOCGWINSZ` is, because no runtime branch saves a file
the compiler has already refused.

**When forced, the tagged file is a shim.** `termsize_unix.go` is twelve lines
of syscall with no branching; the decision that uses it lives behind
`install.go`'s `terminalSize` seam, which tests replace. What CI never compiles
is then also what decides nothing. `TestPlatformSplitsStayShims` holds this: it
lists every build-tagged shipping file with a line budget, fails on one that is
not listed, and fails on one that grows.

**Why not tags as the normal mechanism.** ADR-011 already answered this in
miniature -- container detection could not be tested without being a container,
so nobody had, and a test asserted the bug was correct. A `//go:build windows`
file is the same shape: the Linux runner never compiles it, never vets it, never
tests it. ADR-001 also chose Go for trivial cross-compilation, and tags spend
it. And `Makefile`'s `SOURCES` has no tag awareness, so three platforms of
source would count against a cap meant to bound one binary's complexity, while
`MAX_BINARY_BYTES` correctly measures only the host's.

**What this costs.** Removing the two termsize files entirely would be the
purest reading and is not free: `syscall.TIOCGWINSZ` is undefined on Windows, so
a single-file version needs `golang.org/x/sys` -- a second dependency, which
PLAN.md §13 rules out. The exception is narrower than the dependency it avoids.

**Where single-source genuinely cannot hold**, named rather than assumed:
`bootstrap/install.sh` cannot serve native Windows, and a PowerShell twin is a
second shell file where ADR-001 permits one. That is an ADR question rather than
a build-tag question, and it is the real reason native Windows is post-1.0.

## ADR-032 — One provider file, and a config provider stops needing Go

`slots/` held three unrelated TOML dialects parsed by three structs in three
packages: `slots/tools/` for what bothy fetches, `slots/advice/` for what it
only recommends, `slots/plugins/` for what a generated config depends on. A
provider that was both — yazi, which bothy fetches *and* generates config for
*and* installs plugins for — was three files that never mentioned each other.

Worse, generating config was not data at all. It was an arm in
`install.plan()`, which `docs/adding-a-provider.md` documented with a Go
snippet directly under the sentence "if adding one needs new Go code, stop".

**One file per provider, flat, at `slots/<name>.toml`.** A common header, then
at most one way of being obtained — `[fetch]` or `[advise]` — then `[[file]]`
for config to generate and `[[plugin]]` for what that config depends on.

**Flat rather than `slots/<slot>/<name>.toml`.** Six of fifteen providers fill
no slot, so per-slot directories need an `extras` directory that is not a slot,
plus a test holding each directory name equal to the `slot` field inside its
files. The field is authoritative either way; the directory would only be a
second copy of it that can disagree.

**`[fetch]` beside `[advise]`, not merged.** How a program is obtained is
exactly what differs between them, and one flattened block would hide that. A
provider may have both; none does yet, and vim could.

**`dest` is data, not a convention.** `<ConfigRoot>/<provider>/<file>` fits
seven of the ten generated files and breaks for three — zellij's theme, vim's
colourscheme, and ghostty's config, which is `ghostty.conf` rather than
`ghostty/config`. A convention with three exceptions is not a convention, so
the destination is spelled out with `{theme}` interpolated.

**`when` is a closed vocabulary.** Three files are conditional. An expression
language would want a parser, and PLAN.md §13 allows one dependency, already
spent on TOML. `install.conditions` is a map from name to predicate, and a test
asserts every `when` in `slots/` is a key in it. An unrecognised condition
writes the file rather than skipping it: a typo that shows is easier to notice
than one that silently drops a config.

**What stayed in Go, deliberately.** The `xdg-open` shim fills no slot and
writes to `bin/` rather than the config root — bending it into a provider would
make the format worse to fit one thing that is not one. And
`checks_yazi.go` is yazi-specific knowledge, not configuration.

**The projection.** `internal/slots` is the only thing that parses the format;
`tools.Tool` and `advice.Advice` remain as field copies of it. Fifteen files
downstream are untouched, and "one parser" is true without a refactor reaching
the whole tree.

**Verified by producing the same bytes.** Three configurations — default,
`provide_config = true`, and images off — each write a tree byte-identical to
the one the four hardcoded branches wrote, manifest timestamp aside. The format
is a different way of saying the same thing, which is the only claim worth
making about a refactor. Net −10 lines of code.

## ADR-033 — The multiplexer is a backend, and the interface was measured

ADR-019 made the multiplexer tier three: a renderer, not a template. It stayed
one renderer wearing a general name. `internal/layout` emitted KDL, and six
more decisions about zellij sat outside it — three defaults, the session
lifecycle, the graphics gate and two doctor checks.

`internal/mux` holds a `Backend` interface with two implementations: `Zellij`,
and `None` for the `slots.mux = "none"` bothy already accepted.

**The shape came from a throwaway tmux backend, not from zellij.** A seam with
one implementation is designed against one example. Five guesses were made
about where the boundary would fall and tmux corrected four:

- **`Render` returning a string is wrong.** Zellij is handed a layout file at
  launch; tmux splits a session that already exists. `Open` renders and
  launches together, and `Preview` — for `bothy layout` and one doctor check —
  is the separate thing that only shows.
- **Layout verification is a query, not a file read.** `layoutcheck.go` globbed
  `$XDG_CACHE_HOME/zellij/*/session_info/<session>/session-layout.kdl`, a
  private path with a version directory in it. tmux answers with `list-panes`,
  and zellij with `action dump-layout`, which is documented. The KDL parser
  stays; the glob does not.
- **`discardDeadSession` is not an interface method.** It exists because zellij
  resurrects EXITED sessions. tmux has no dead state, so this lives inside
  zellij's `Open`.
- **Session naming differs in kind, not charset.** tmux *accepts* "." and ":"
  and then cannot address the session, because they are its own target
  separators. A shared sanitiser would produce sessions bothy cannot attach to.

The fifth guess held: `TabBar` and `StatusBar` are zellij plugin panes sitting
in the generic profile type, and are still there.

**The spike does not ship.** ADR-012 says supported means CI-tested, and a
half-tmux in the tree is a promise bothy has not made. What survives is the
interface it corrected and this record.

**What it cost: +178 code lines**, against an estimate of +50 to +100 made
before the survey. The estimate was wrong by about double, which is the answer
ADR-003 had been guessing at since 0.1 when it said tmux would "roughly double
the layout renderer". The comment ratio held at 22% (ADR-029) without raising
it, once flourish was cut rather than budget.

**Two zellij references remain outside the package, deliberately.**
`config.Default()` names `zellij` as the shipped multiplexer, which is what a
default is. And `bothy keys` prints a table of zellij's own bindings, because
bothy sets none of its own; a tenth interface method to carry help text would
fit the interface to one command rather than to the boundary.

## ADR-034 — The agent can be walled off, on request, and never by default

The agent slot runs an arbitrary command with the user's full permissions: every
repository, `~/.ssh`, the shell history. bothy does not make that worse — it is
the access the agent has when started by hand — but bothy owns the launch, which
is a position to offer something better.

`bothy confine` runs the agent pane in a rootless podman container. Nothing else
about the workspace changes.

**A command, not a setting.** There is no `confine = true`. A default that
changes how the agent runs would break for people who never asked for it and
could not tell why. Typing the command is the opt-in, and not typing it leaves
bothy exactly as it was.

**bothy writes the recipe and does not build it.** The container needs the agent
inside it, and installing an agent is what PLAN.md §11 rules out: install
methods change, credentials are not bothy's business, and one arriving unasked
is the overstep the rule exists to prevent. So the first `bothy confine` writes
a Containerfile into bothy's tree, prints the one command that builds it, and
stops. It teaches rather than fails, because a `--print` flag nobody knows about
is not a way in. Once written the file is the user's, and bothy does not
overwrite it.

**The wall is the filesystem, and the README says so.** Mounted: the project
directory, writable, because editing it is the job; and the agent's own
credentials, without which it cannot log in and the wall protects nothing
anyone wanted. Not walled: the network, because the agent calls its API. #116
proposed an allowlist, which needs pasta or slirp configuration and is a second
feature. A wall people misunderstand is worse than none, so the limits are
stated where the feature is.

**`label=disable`, not a `:z` mount.** SELinux enforcing means a bind mount is
unreadable without one of the two. `:z` relabels the user's project directory
to `container_file_t`, which persists after `bothy uninstall` and is a change
outside bothy's tree — measured here, on this repository, and undone. The wall
is the mount set and the user namespace; SELinux confinement of the container
would be a second one, and not at that price.

**The toolbox hop.** bothy often runs inside a toolbox, where there is no
podman, and the host's is reachable through `flatpak-spawn` — the hop the
`xdg-open` shim already takes. Without either, confinement is unavailable and
says so; it never silently runs unconfined.

**Linux is what CI tests.** On macOS podman runs a Linux VM: the wall is real
and its edges differ. ADR-018's pattern applies — an untested platform is
labelled, not refused. Someone who wants it supported adds a CI job and the
label comes off.
