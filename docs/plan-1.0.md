# Plan — the road to 1.0

Five milestones and a contract.

What bothy is aiming at is [`north-star.md`](north-star.md); this is the order
it gets there in.

## What bothy is

A minimal cockpit for one person and one agent: file browser, agent, shell,
vim navigation throughout, in one terminal window. It brings its own configs,
verifies every download, never touches your dotfiles, and removes itself
completely. The three panes are the invariant (ADR-017); which terminal,
multiplexer, browser and agent produce them is a matter of what you already
have, and bothy says per stack what it can and cannot deliver.

**The agent is the point** (ADR-016), which is a claim about shape rather than
requirements — `slots.agent none` still works, and bothy still installs no
agent. It settles what the file browser and the side pane are *for*, which is
what #45 and #56 were waiting on.

**The restraint is the product.** bothy is not an IDE, not a plugin platform,
not a multi-agent orchestrator, and is not going to become any of those. The
"What bothy is not" list in the README is the most important section in it, and
every line on it that says what bothy will not *become* is protected.

Two lines that said what bothy will not *run on* have come off it — tmux and
native Windows — because a refusal and a CI table answer the same question and
only one can be checked (ADR-018). Scope now lives in the table of stacks CI
tests. That is a shortening of the list and it is not a loosening of it: the
README says less about platforms and proves more.

So nothing below is a feature. Each item closes a gap between what the README
already says and what the code already does, makes an existing claim honest,
or writes down a promise somebody can depend on.

The rule for every item: **if it adds a pane, a daemon, or a dependency, it is
out. If the README says it and the code does not do it, it is in. Supported
means tested in CI; everything else is labelled untested.**

That is a narrower claim than "no new features", which would not survive
contact with this document — `bothy ls`, `bothy keys` and `bothy plan` are new
commands. What is being kept is the shape: no new panes, no new dependencies,
and no new runtime behaviour. `bothy plan` is the last of those three because
it decides nothing new; it prints the decision `bothy install` already makes
silently.

---

## 0.2.0 — the promises the README already makes

Linux only. Nothing here is new behaviour.

**Named sessions.** `launch` runs `zellij --layout <file>` with no
`--session`, so every `bothy` creates an anonymous session and `bothy attach`
cannot choose between them. Name it `bothy-<project dirname>`, use
`zellij attach --create`, and have `bothy` in a project with a live session
attach rather than start a second. Add `bothy ls`. This is what makes "a
terminal window that stays put" and "reconnect to a running session" true.

**`workspace.launch = auto | here | window`.** Spawning a Ghostty window is
the most surprising thing a new user meets and today it can only be overridden
per invocation. Make it a setting; keep `--in-place` and `--window` as one-off
overrides. `decideLaunch`'s forced-mode switch is the seam.

**Pin the Yazi plugins in the repository.** `ya pkg add` records a rev and a
hash — but on the machine, at install time. Two machines installing on
different days get different plugin revisions, which is the reproducibility
gap `bothy.lock` closes for the nine tools. Commit the rev and hash into
`slots/plugins/yazi.toml` and install those revisions.

Not vendoring: `PLAN.md` §11 rules out bundling other people's code and
`NOTICE` states plainly that the plugins are not vendored here. Both stay
true. `git` remains a prerequisite, and the README should say so rather than
imply otherwise.

**Drop delta.** Nothing bothy writes has referenced it since ADR-009 removed
the git-pager wiring, so it was 7 MB downloaded for a feature that no longer
exists. Out of `DefaultExtras`, out of `slots/tools/`, out of `bothy.lock`.
Anyone who wants it installs it themselves and wires it into the gitconfig
bothy has promised not to touch. (#45, ADR-020)

**Fix the host-binary assumption in `spawnTerminal`.** Inside a container it
assumes the host's bothy is at `~/.local/bin/bothy`, which is wrong for a dnf
or deb install. Resolve it through `flatpak-spawn --host command -v bothy`.

**`bothy keys`.** Not a feature, a courtesy: someone who has never used Zellij
or Yazi is stranded inside the workspace with no idea how to move focus or
exit. Print the six bindings that matter, once, on first launch. No pane, no
plugin.

**Fix `config.Set`.** It is a hand-maintained switch that must mirror the
struct, and it has already drifted: `Keys()` is reflect-derived and lists
`extras`, `Set` has no arm for it, so `bothy config set extras …` reports an
unknown key that `Nearest` will suggest. Replace the switch, and add the test
that asserts `Set` covers every key `Keys()` returns.

**Drop the `XDG_DATA_HOME` and `XDG_STATE_HOME` redirect.** `SessionEnv`
points both into bothy's tree for every subprocess, which hides Neovim's
plugin directory, zoxide's database and lazygit's state from the tools running
inside the workspace — the opposite of "your editor is yours". Keep
`XDG_CACHE_HOME`, which is what keeps `ya pkg`'s clone inside the tree.

Replace them with a doctor check reporting what the tools wrote outside
bothy's tree, so the leak is visible rather than hidden.

**This one lands last and alone.** `make check` will stay green — no unit test
asserts any XDG variable — but `container_test.go` asserts that after the real
tools have run nothing exists outside bothy's tree, and `container` is a
required check. The expectations move in the same pull request, and the job is
watched going red and then green rather than assumed.

*Order: sessions before `workspace.launch` (both touch the launch path), XDG
last. The rest are independent.*

---

## 0.3.0 — macOS, verified rather than claimed

The README lists macOS, goreleaser builds darwin, and CI proves none of it:
`decideLaunch` keys on `DISPLAY`, and the Yazi opener is hard-coded to
`xdg-open`. Either finish it or un-claim it. (#33)

- The opener uses `open` on darwin.
- `decideLaunch` stops keying on `DISPLAY`/`WAYLAND_DISPLAY`; on darwin a
  graphical session is assumed.
- Ghostty detected via `/Applications/Ghostty.app` or `open -a`. Kitty and
  WezTerm accepted as spawn targets, since mac users are less likely to have
  Ghostty.
- A macOS runner that installs, runs `bothy install --offline`, and runs
  `bothy doctor` against a table of expected severities, as the container job
  does.

The README no longer needs the word removed. It carries a **Where it runs**
section instead, which says macOS is untested, names what is broken on it, and
points at the CI table for what "supported" means — strictly more use to
someone on a Mac than deleting the word, which would have implied bothy does
not install there at all. It does; it just never opens its own window.

The gate: the macOS job is required for merge, and the section is rewritten
the day it goes green.

---

## 0.4.0 — one provider format, and capabilities reported

Everything bothy knows is meant to be data. Today `slots/` holds three
unrelated dialects — tools, advice and plugins — parsed by three structs in
three packages, and the per-slot directories the model implies (`slots/mux/`,
`slots/browser/`, `slots/terminal/`) exist on disk and are empty. Nothing reads
them. §4 of the north star assumes one format; this is where it gets written.

**One provider format**, gaining the three fields the planner walks:
`platforms`, `provides`, `redirect`. The existing dialects migrate onto it.

**The agent slot moves with it** (#56). Agent definitions become
`slots/agent/<name>.toml`: binary name, the environment variable it sets when
running, and the install command the doctor prints when it is missing.
claude-code, codex, gemini-cli, aider and opencode, each verified against the
current release rather than from memory. The gate is unchanged: adding a sixth
agent touches no Go.

**The second tier shrinks as the first grows.** `advice.binary` is parsed and
never read, while `EditorBinary`, `agentBinary` and an inline map in a doctor
check each hardcode the same `claude-code → claude` mapping in Go. Three
copies of data that already exists in a file is the first thing the unified
format buys back, and deleting them is what keeps this milestone from adding
net lines.

**The doctor reports per capability.** `Check` and `Result` gain a
`Capability`, stamped in the same loop that already stamps the ID, and
`doctor --json` grows a grouping. Nothing new is measured; what is already
measured gets grouped under the five names in ADR-017 so that "this stack
cannot give you images" is a line rather than an inference.

**Three terminal tables become one.** bothy recognises four terminals
(`platform.detectTerminal`), scores three for graphics
(`probe.graphicsTerminals`), and can spawn exactly one (`ghosttyCommand`). A
capability model needs a single table, and it is where iTerm2's own image
protocol and the terminals that draw nothing both get written down.

**Guard `bothy desktop-entry`.** It writes an XDG `.desktop` file with no
platform check, which is meaningless anywhere but Linux.

---

## 0.5.0 — the multiplexer seam

#64 asked whether the multiplexer is a slot like any other. It is not, and
0.5.0 is where that stops being an assumption and becomes a measurement.

`internal/layout` is a Zellij KDL emitter under a generic name, and it is not
the boundary. Six more `== "zellij"` decisions live outside it: `ZellijDir`,
the template branch in `install.plan()`, `ZELLIJ_CONFIG_DIR` in `SessionEnv`,
`--layout` at launch, the graphics gate's version check, and the doctor check
that counts panes by globbing Zellij's own `session-layout.kdl`. A backend
interface has to cover all seven or a second one leaks through the gaps.

So: extract the interface with Zellij as its only implementation. Nothing new
works afterwards. What exists afterwards is a number — how much of the tree a
multiplexer actually owns — in place of ADR-003's estimate that tmux "would
roughly double the layout renderer", which this shows to be low.

The gate: `grep -i zellij` across `cmd/` and `internal/` returns hits only
inside the backend package and `slots/tools/zellij.toml`.

---

## 0.6.0 — `bothy plan`

Sense, score, constrain, recommend, apply — north star §5. One pass that says
what your stack can give you, what it cannot, and what bothy would fetch to
close the difference, before anything is downloaded.

Less of this is new than it looks. `decideLaunch` already returns a reason on
every branch — it is a scorer. `probe.CheckGraphics` already returns
`{Supported, Reason}`, which is a capability row. `advice.Command` already
recommends, keyed on distro then `ID_LIKE` then OS. What does not exist is the
constrain step, and that is a walk over the `platforms` and `provides` fields
0.4.0 introduces — no Go per provider.

`bothy` on first run does this. `bothy plan` re-runs it. `bothy doctor` stays
the after-the-fact check. One engine, three entry points, and the reason that
matters: a planner and a doctor answering the same question twice will
eventually disagree, and the disagreement would be silent.

The gate: `bothy plan` on a machine with no Ghostty and an old Yazi prints what
it would keep, what it would fetch, and the capability lines — and `bothy
install` does exactly that and nothing else.

---

## 1.0.0 — the contract

1.0 is a promise about stability, not a feature level.

**Declared stable:** the keys in `config.toml`, the profile and palette TOML
schemas, the two directories, and the `doctor --json` shape. `schema = 1` goes
into `config.toml` so any of them can evolve later.

**Supported means tested in CI** (ADR-012, ADR-018): Fedora, Ubuntu, one
Debian derivative via `ID_LIKE`, Arch, macOS arm64. Everything else is
labelled untested, never "should work" — and after 0.6.0, `bothy plan` says
per machine what untested actually costs there.

This table is now the only statement of scope. The README's not-list no longer
carries platform lines, so if a stack is missing here it is missing from the
claim, which is the point.

**Signed releases.** The bootstrap verifies a checksum from the same release
as the archive, which catches corruption but not a compromised release — the
script says so itself. Sign with minisign or cosign and verify the signature.

**Distribution.** Copr and `.deb` exist; add a Homebrew tap and an AUR
package. Both are things this project would then be running, which ADR-013
was careful about, so each needs to say how its version stays in step with
`packaging/bothy.spec` — or be labelled best-effort.

**Not self-update.** `bothy upgrade` prints the command for however this copy
was installed and does not replace the binary. There are five install channels
and one of them could be self-updated safely; a dnf- or dpkg-owned
`/usr/bin/bothy` belongs to its package manager. The README lists
"auto-updater" among the things bothy is not, and 1.0 keeps that line.

**Cut before shipping:** the watermark extra and `docs/watermark.md`; the
`bothy dev` alias; `bothy lock` from public help, since it is a maintainer
command; and `docs/plan-0.1.x.md` into `docs/history/`. Someone opening
`docs/` should find "what happens when I type bothy", not process artifacts.

**Docs:** a README rewritten around "this is all there is, on purpose",
leading with the one-command launch and the uninstall guarantee. One short
page describing what happens when you type `bothy`, in order — detection,
fill-gaps, config render, launch decision, session — which is what the
doctor's output should link to. And a ninety-second recording.

---

## Explicitly not on the roadmap

Recorded so they are decided rather than pending.

- **Telemetry, gauges, or an instrument bar.** The agent CLIs draw their own
  status lines. bothy does not duplicate them.
- **Multi-agent or worktree orchestration.** The agent CLIs do this
  internally. A second agent pane is a token sink, not a workflow.
- **A diff pane or review mode.** lazygit is one keypress away in the shell
  pane. That is the review mode.
- **Editing the agent's configuration, hooks, or credentials.** Already a
  documented non-goal, and it stays one.
- **A plugin API or a background service.** Still on the "not" list, unchanged.

## Deliberately after 1.0

Not refused, not promised, and not in any milestone above. Both become additive
once 0.4.0 gives providers a `platforms` field and 0.5.0 extracts the mux seam,
which is the whole reason those two land before the contract and these two do
not.

- **tmux.** The second backend, and the first time the invariant is produced
  two ways. It buys the Linux and macOS common stacks a multiplexer their users
  already have. The cost is a renderer, a doctor check and a graphics gate, and
  0.5.0 will have measured it.
- **Native Windows.** Windows Terminal's own panes as the multiplexer: panes
  and isolation, no sessions, no theme. It needs windows entries across the
  tool matrix, several of which upstream does not publish, and a second
  bootstrap in a second language — ADR-001 permits exactly one shell file and
  spends it on `bootstrap/install.sh`, which rejects Windows by name.

Neither is worth doing on speculation. Both are worth doing when someone asks,
and after 1.0 the answer to "does bothy work on my stack?" is a command that
tells them rather than a sentence that guesses.
