# Plan — the road to 1.0

Written after 0.1.5. Three milestones and a contract.

## What bothy is

A minimal cockpit for one person and one agent: file browser, agent, shell,
vim navigation throughout, in one terminal window. It brings its own configs,
verifies every download, never touches your dotfiles, and removes itself
completely.

**The restraint is the product.** bothy is not an IDE, not a plugin platform,
not a multi-agent orchestrator, and is not going to become any of those. The
"What bothy is not" list in the README is the most important section in it,
and 1.0 protects that list rather than shortens it.

So nothing below is a feature. Each item closes a gap between what the README
already says and what the code already does, makes an existing claim honest,
or writes down a promise somebody can depend on.

The rule for every item: **if it adds a pane, a daemon, or a dependency, it is
out. If the README says it and the code does not do it, it is in. Supported
means tested in CI; everything else is labelled untested.**

That is a narrower claim than "no new features", which would not survive
contact with this document — `bothy ls` and `bothy keys` are new commands.
What is being kept is the shape: no new panes, no new dependencies, no new
runtime behaviour.

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

**Remove delta from `DefaultExtras`.** Nothing bothy writes has referenced it
since the git-pager wiring was removed; 7 MB nobody uses. Anyone who wants it
has it in their own gitconfig. (#45)

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

Until it ships, macOS comes out of the README install table. The gate: the
macOS job is required for merge.

---

## 0.4.0 — the agent slot

The agent is the reason bothy exists and the slot people change most. It
should not be a switch in Go.

Agent definitions move to `slots/agents/<name>.toml`: binary name, the
environment variable it sets when running (for the nested-session guard), and
the install command the doctor prints when it is missing. Ship definitions for
claude-code, codex, gemini-cli, aider and opencode, each verified against the
current release rather than from memory.

This is the one item that could be mistaken for a feature. It is not: the
README already promises "any command you name" for the agent slot. This makes
the five likeliest names work without anyone looking up a binary name.

The gate: adding a sixth agent touches no Go code, and
`docs/adding-a-provider.md` already covers how.

---

## 1.0.0 — the contract

1.0 is a promise about stability, not a feature level.

**Declared stable:** the keys in `config.toml`, the profile and palette TOML
schemas, the two directories, and the `doctor --json` shape. `schema = 1` goes
into `config.toml` so any of them can evolve later.

**Supported means tested in CI:** Fedora, Ubuntu, one Debian derivative via
`ID_LIKE`, Arch, macOS arm64. Everything else is labelled untested, never
"should work".

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
- **tmux, Windows without WSL, a plugin API, a background service.** Already
  on the "not" list, unchanged.
