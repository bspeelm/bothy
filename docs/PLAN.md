# PLAN.md — bothy

> **bothy** *(n., Scottish)* — a small unlocked mountain shelter, free for anyone
> to use, kept by two customs: leave it as you found it, and leave fuel for the
> next visitor.

A turn-key terminal workspace built from tools you already trust. `bothy`
launches a persistent layout — file browser, agent, shell — and `bothy doctor`
tells you what is broken and how to fix it.

This is revision 2. Revision 1 is in git history; §1 says what changed and why.

---

## 1. What changed in revision 2, and why

Revision 1 made bothy **a thing that sets up your machine**: it wrote
`~/.config/yazi/yazi.toml`, `~/.vimrc`, six `git config --global` keys and a
`~/.bashrc.d` fragment, backing up whatever was there. That worked — it is
implemented and tested — and it was the wrong shape.

Three problems, in increasing order of seriousness.

**It made bothy unsafe to try.** "Let me rewrite your dotfiles, I promise I
backed them up" is a large ask of a stranger evaluating a tool.

**It made the machinery load-bearing for the wrong reason.** Manifests, content
hashes, backup-and-restore, skip-if-you-edited-it — several hundred lines exist
purely because bothy touched files it did not own.

**It took over things that were never its business.** A workspace tool
replacing your `.vimrc` and wiring your global git pager is overreach. Nobody
has a pre-existing opinion about the Zellij layout `bothy` launches; plenty of
people have one about their editor.

Revision 2 inverts it. **bothy is a workspace you enter, not a setup it performs
on your machine.** It brings its own config tree and launches the tools pointed
at it. Your dotfiles are neither read nor written.

Three facts made this possible, all verified rather than assumed:

| Tool | Isolation mechanism | Verified by |
|---|---|---|
| Zellij | `ZELLIJ_CONFIG_DIR` | `[CONFIG DIR]` in `zellij setup --check` follows it |
| Yazi | `YAZI_CONFIG_HOME` | `yazi --debug` reports every config path under it |
| Ghostty | `--config-file=`, palette **inlined** | `+validate-config` exits 0 |

Ghostty was the case revision 1 assumed impossible. It is possible, with one
wrinkle: theme *lookup* paths are not relocatable, so a named `theme = dracula`
still hunts in `~/.config/ghostty/themes/`. Writing the palette directly into
bothy's own config file sidesteps that entirely.

And because bothy launches the terminal, it owns the whole process tree — it can
set `PATH`, `EDITOR` and the config variables for that session alone. The
`~/.bashrc.d` fragment, the last write outside bothy's own directory,
disappears.

---

## 2. The footprint

This is the load-bearing section. Everything else serves it.

```
~/.local/bin/bothy                  the only binary on your PATH
~/.local/share/bothy/
├── config/                         bothy's configs — never yours
│   ├── zellij/{config.kdl,themes/,layouts/}
│   ├── yazi/{yazi.toml,theme.toml,keymap.toml,init.lua,plugins/}
│   ├── ghostty.conf                palette inlined, no theme reference
│   └── vim/vimrc
├── bin/                            only tools bothy had to fill in
└── state/manifest.json             what was installed, for uninstall

~/.config/bothy/                    YOURS — small, and worth putting in git
├── config.toml                     slots, profile, passthrough
├── palette.toml                    your colours, if not open Dracula
└── overrides/<tool>/<file>         appended after each template
```

**Nothing else.** No `~/.vimrc`, no `~/.config/yazi`, no `~/.config/ghostty`, no
`~/.bashrc.d`, no `git config --global`. `bothy uninstall` removes two
directories.

That second directory is also the answer to portability: clone it onto a new
machine, run `bothy install`, and you have your workspace. One folder, not a
manifest of files scattered across your home.

---

## 3. Principles

1. **Isolated.** bothy reads and writes its own tree. A file you own is a file
   bothy does not touch. This replaces revision 1's backup-and-restore contract,
   which was a promise; this is a property.
2. **Thin.** An installer, a config writer, a launcher, and a doctor. No daemon,
   no GUI, no plugin marketplace, no telemetry, no bundled copies of the tools.
3. **Fill gaps, never replace.** Applies to binaries and to the terminal: if
   what you have is good enough, use it; if it is missing or too old to work,
   supply one — scoped to bothy's session, not to your PATH.
4. **Slots, not features.** Every component is a slot with interchangeable
   providers described by data. Adding a provider must not require core changes.
5. **Every bug becomes a doctor check.** When a setup failure is fixed, the fix
   ships with a check that detects it. The doctor is the moat.
6. **Budgets are real.** Binary ≤ 10 MB, code ≤ 6k lines, comments ≤ 25% of
   code, workspace idle RSS ≤ 200 MB excluding the agent. Asserted in CI.
   The code figure began at 5k and rose once, deliberately (ADR-026); the
   comment figure became a ratio when a total proved to be measuring code and
   prose together (ADR-021).

   The line budget counts code and comments separately, because counting them
   together was measuring the wrong thing: this codebase comments densely on
   purpose — most of them record why a thing is the way it is, which is the
   part that would otherwise be lost — and a single number punished that
   exactly as hard as it punished sprawl. Under the combined count the project
   appeared to be at 5,010 of 5,000 and I proposed raising the limit; split
   out, it was 3,697 lines of code against the same 5,000. The budget had not
   been breached, the metric had been wrong.

7. **Simplicity beats cleverness.** Prefer a documented manual step to an
   automated one that fails silently.
8. **One palette ships.** Open Dracula, MIT. Every other palette is a file on
   your machine. Enforced by a test (ADR-006).

---

## 4. Binaries: fill gaps, scoped

Configs and binaries are **separate axes**. Isolating configs costs nothing;
duplicating binaries costs disk. So they get different policies.

At install, for each required tool:

- On `PATH` and meets the minimum version → **use it**. Record where it came
  from. Never upgrade, remove, or ask a package manager for anything.
- Missing, or below the minimum → **fetch a pinned release** into
  `~/.local/share/bothy/bin/`, verified against `bothy.lock`.

bothy prepends its own `bin/` to `PATH` **for the session it launches**, so a
filled-in tool never shadows your everyday one. Installing zellij 0.45.1 does
not change what `zellij` means in your shell.

Worked example, from the machine bothy was built on: Fedora's COPR ships zellij
**0.42.2**, which cannot pass the Kitty graphics protocol — the cause of
block-art previews and the phantom "Find next" keypress. Upstream **0.45.1**
fixes it. The minimum is therefore 0.45.1, bothy fetches it, image previews
work, and dnf is never involved. On a machine with a current zellij, bothy
downloads nothing.

Full download cost if *every* tool has to be filled in — measured, not
estimated:

| zellij | yazi | lazygit | jq | ripgrep | fzf | fd | zoxide | total |
|---|---|---|---|---|---|---|---|---|
| 17.2 | 13.1 | 6.6 | 2.2 | 1.9 | 1.9 | 1.4 | 0.5 | **44.8 MB** |

Static binaries, no dependency trees, shared across every project.

`--system` mode is **dropped**. It existed to install tools system-wide via the
native package manager, which is precisely what principle 3 says not to do.
Anyone who wants their distro's tools already has them, and bothy will use them.

---

## 5. Configs: isolated, with passthrough

bothy renders its configs into `~/.local/share/bothy/config/` and launches each
tool pointed there. **Passthrough** opts a slot out, per tool:

```toml
# ~/.config/bothy/config.toml
passthrough = ["yazi"]     # use ~/.config/yazi instead of bothy's
```

This is one environment variable per slot, not a second code path — the launcher
either points `YAZI_CONFIG_HOME` at bothy's directory or at yours. For someone
with a Yazi setup they like, it means bothy's layout and their file browser,
which is a reasonable thing to want.

The doctor reports which slots are passed through and what is lost by it — the
image-preview handling and the container-aware opener live in bothy's
`yazi.toml`, so passing Yazi through means those do not apply.

`~/.config/bothy/overrides/<tool>/<file>` still appends after the template, for
adjusting bothy's config rather than replacing it. That mechanism is built and
tested and does not change.

**Editor and git are not configured at all.** bothy sets `EDITOR` for its own
session and otherwise leaves your editor alone. A generated vimrc remains
available for someone who has none, opt-in via `editor.provide_config = true`.
The six `git config --global` delta keys are removed outright; documenting them
in the README is the right amount of help.

---

## 6. The terminal

Same gap-filling rule, applied to where bothy runs:

- Current terminal speaks the Kitty graphics protocol (Ghostty, Kitty, WezTerm)
  → **run in place**, in the terminal you are already in.
- It does not, or there is no terminal (a desktop launcher, a `.desktop` entry)
  → **spawn Ghostty** with `--config-file=<bothy's>`, falling back to running in
  place with a doctor warning if Ghostty is not installed.

The reason is not aesthetics: image previews need a terminal that can draw them,
and degrading silently to block art is the exact class of failure bothy exists
to catch. Ghostty is never installed by bothy — it is a GUI application that
belongs to the host, and on an immutable distro it needs `rpm-ostree` and a
reboot. bothy advises; the doctor checks.

Because the config file is bothy's and the palette is inlined,
`~/.config/ghostty` is never touched. The watermark extra points at art inside
bothy's tree.

---

## 7. Slots, profiles, theming

Unchanged from revision 1 and already built.

**Slots**: terminal (advise-only), mux (zellij), browser (yazi, none), editor
(vim, nano, helix), agent (claude-code, any command), theme, extras.

**Profiles**: `cockpit` (default — browser on top, agent + shell below),
`editor` (three columns), `minimal` (agent + shell). Small TOML describing rows
and columns, rendered to Zellij KDL by a renderer that owns the two KDL traps:
`split_direction="vertical"` means columns, and plugin nodes are always emitted
multi-line. `cockpit` renders byte-identical to the origin `dev.kdl`.

**Theming**: one eleven-token palette drives zellij, yazi, ghostty and a
generated vim colorscheme. Open Dracula ships; any other palette is a file you
point `theme.palette` at. `bothy theme example` prints the blank form. See
ADR-006 — settled, and enforced by a test.

---

## 8. Doctor

The check list is largely unchanged, but **what it audits shifts**: bothy's own
tree, plus the parts of your machine that can still break the workspace.

Still checks, unchanged in value:

- **Yazi silently discarding its whole config** — the highest-value check there is
- Yazi ≥ 26 when plugins are installed; `[mgr]` not `[manager]`; `url` not `name`
- Image previews: which side of the version gate this machine is on, and why
- `zellij setup --check`; generated KDL parses
- Terminfo for `$TERM` resolvable, especially inside a container
- Agent on PATH, and not nesting inside an existing agent session

Revised or new:

- **Terminal capability** — can the terminal bothy is about to use draw images?
  Reports whether it will run in place or spawn Ghostty.
- **Passthrough** — which slots use your configs, and what that turns off.
- **Binary provenance** — for each tool, system or bothy-supplied, and its
  version. Replaces the PATH-shadowing check, which mattered only because
  revision 1 installed into `~/.local/bin`.
- **Layout actually built** — compares the profile's pane count against
  Zellij's resolved `session-layout.kdl`. Guards the far side of the renderer:
  that Zellij still *interprets* the KDL the way it did when the renderer was
  written.
- **Yazi plugins** — bothy's config references four; a missing one costs the
  feature it names.
- **Profile renders** — a hand-written profile is the likeliest thing here to
  be broken, and better caught before launch than at it.

Dropped: `xdg-open` shim guard, `EDITOR` override, vim colorscheme location,
Ghostty near-miss filename. All four existed because bothy wrote into a shared
home. The opener still matters inside a container, but bothy now sets it in its
own session rather than installing a shim on a shared PATH.

Output stays `✓ / ! / ✗`, a one-line fix under every failure, `--json` for CI,
non-zero exit on any failure. Every failing check carrying a fix is enforced by
a test.

---

## 9. What this costs

Stated plainly, because a plan that only lists benefits is a sales document.

**Several hundred lines of tested code get deleted.** The backup directory,
content-hash drift detection, restore-on-uninstall and the git-settings revert
exist to protect files bothy will no longer touch. `state.Manifest` shrinks to a
list of installed binaries.

**bothy does less.** It no longer carries your `.vimrc` or your delta wiring to
a new machine. Those are dotfiles, and revision 1's own rule — when unsure
whether something belongs in bothy or in the underlying tool, it belongs in the
underlying tool — says they were never bothy's to carry.

**Two configs for the same tool, on purpose.** Your Yazi keybindings do not
apply inside bothy unless you pass Yazi through. That is the price of isolation
and it cuts both ways.

**A bigger behaviour on non-Ghostty terminals.** Spawning a window is more than
revision 1 ever did, and on a headless box there is nothing to spawn — so the
run-in-place path has to stay correct, not become an afterthought.

---

## 10. Phases

Each ends green in CI. Phase A is the turn; B is the piece never built.

**Phase A — isolation. Done.** Introduce a config root and thread it through
`install.plan()` (already parameterised by directory, so this is one variable,
not a rewrite). Launch with scoped `ZELLIJ_CONFIG_DIR`, `YAZI_CONFIG_HOME`,
`EDITOR`, `PATH`. Inline the palette into `ghostty.conf`. Delete the `~/.vimrc`,
`~/.bashrc.d` and `git config --global` writes, and the backup machinery they
required. A test proves nothing is written outside bothy's tree.

**Phase B — the fetcher. Done.** `bothy.lock` pins version + sha256 for nine
tools across four platforms, generated by `bothy lock` from the real bytes.
Downloads are verified before anything is written; extraction handles the four
archive layouts upstream actually uses. `bothy tools` shows the decisions.
Not covered: tar.xz, which the standard library cannot unpack and which is why
helix is not yet a tool definition.

**Phase C — terminal launch and passthrough. Done.** Capability detection with
an explicit reason for every decision; Ghostty spawned with `--config-file`,
via `flatpak-spawn --host` from inside a container; run-in-place whenever a
window cannot work (no display, no ghostty, already inside a spawned one).
Per-slot passthrough skips both the config write and the environment variable.
The `.desktop` entry is a separate opt-in command, because it is the one file
that must live outside bothy's tree to work at all.

**Phase D — doctor revision. Done.** The §8 changes, plus the pane-count check
revision 1 specified and never built — keyed on `ZELLIJ_SESSION_NAME` so it
reads the session you are in, and tested against real `session-layout.kdl`
files rather than an invented fixture. Also fixed the bug the work exposed:
bothy's `init.lua` required Yazi plugins it never installed, and the config
check could not see it because `yazi --clear-cache` does not execute
`init.lua`. Plugins are now installed with `ya pkg` into bothy's tree, and the
generated config is written to match what is actually present.

**Phase E — docs and v0.1.0. Done bar the tag.** README rewritten around the
isolation model; `docs/adding-a-provider.md` covers the three kinds of thing
you can now add; `NOTICE` credits Dracula and states plainly that no tool is
vendored. goreleaser builds linux/darwin x amd64/arm64 under names
`bootstrap/install.sh` already expects, and a release workflow re-runs the
gates at the tag rather than trusting the branch.

Two CI problems fixed here rather than shipped: the round-trip job filtered on
a test name Phase A had renamed, and `go test -run` exits 0 when its filter
matches nothing — green, and testing nothing. It now asserts how many tests
ran. And the README claimed the doctor detected traps whose checks Phase A had
deliberately removed; prose has no compiler, so a test now checks the command
table against `main.go`.

`bothy` now sets the workspace up on its first run rather than refusing and
naming another command, so getting from nothing to a working workspace is two
commands — fetch the binary, then run it. That is the same shape as
`flatpak install` followed by running the app, and it is the shape it should
always have had.

**v0.1.0 is tagged and released.** The release workflow ran for the first time
and passed: `make check`, then goreleaser building linux and darwin for amd64
and arm64 under the names `bootstrap/install.sh` already expected. The
one-liner in the README was then run against the real release and installs a
working binary.

**Phase F — Ubuntu, and the code worth porting. Done.** The sentence that used
to sit here said the remaining work before v0.2.0 was to run bothy on a distro
that was not Fedora. It now does, and CI keeps it that way: a container job
installs into `fedora:44` and `ubuntu:24.04`, checks the whole doctor report
against a table of expected severities, and uninstalls, asserting the tree is
empty afterwards. See `docs/history/plan-0.1.3.md` for the milestone and ADR-011/012
for the two decisions it forced.

Four things were wrong and only ever visible on Fedora: every named podman
container was detected as a Toolbx, `bothy attach` could not hop into a
distrobox, the `xdg-open` shim was written into containers with no host to
forward to (and then passed its own check), and `checkTerminfo` named a package
that does not exist. The cleanup that preceded the port removed five
declarations with no callers and five comments describing code that had been
deleted, and split the two files that had outgrown themselves.

The container job also found what the unit tests structurally could not:
`SessionEnv` set no XDG directories, so the tools bothy runs wrote outside its
tree at every command except `install`.

`bothy upgrade` shipped, and is worth saying what it is not: it prints the
command for however this copy was installed and never writes to bothy's own
binary. §11's auto-updater non-goal is untouched -- what was deferred here was
a command you type, which is a different thing from a process that replaces
you while you are not looking. It ships alongside a doctor check for the
matching bug: a launch never re-renders, so a newer binary ran against an
older binary's configs indefinitely and nothing said so.

Deferred: macOS, Windows, tmux — see ADR-018, which withdrew the refusals
and left the costs.

Next, in 0.1.4: an apt path via goreleaser's `nfpms:`, and a weekly job that
opens a tracking issue when a pinned tool has a newer release.

---

## 11. Non-goals

- A plugin marketplace, extension API, or runtime plugins
- Bundling or vendoring the tools themselves
- Installing anything system-wide, or asking a package manager for anything
- Managing your dotfiles, your editor config, or your global git config
- LSP/debugger management, background services, auto-updaters, telemetry, accounts
- Managing the agent's config, keys, MCP servers, or hooks
- Parallel-agent orchestration — one agent pane per profile is the scope

Which platforms and which multiplexer are not on this list. That scope is the
table of stacks CI tests, not a refusal — ADR-012 and ADR-018, with the table
itself in [`north-star.md`](north-star.md) §6.

---

## 12. Prior art

- **Yazelix** — closest prior art: Yazi + Zellij + Helix, reproducible via Nix.
  bothy differs by needing no Nix, seating an agent in the main pane, and
  shipping a doctor and an uninstall. Its Zellij/Helix keybinding-conflict work
  is worth learning from for the `editor` profile.
- **Agent-manager apps** (wmux, Pane, cmux, AgentsRoom) — parallel-agent cockpit
  GUIs. They solve *many agents at once* by shipping a new app, which is the
  weight-gain trajectory this project rejects.
- **Omakub / Omarchy** — the same taste one level up, at the OS.
- **devcontainers, DevPod** — project *dependency* setup. Orthogonal; compose,
  don't compete.
- **Hand-rolled setups** — this layout is widespread folk practice in dotfiles
  and blog posts. That is the demand signal: everyone builds it by hand.

**In one line:** the boring native version — Yazelix without Nix, plus an agent
seat; the agent cockpit without the app.

---

## 13. Working agreement

- Work phase by phase; a PR per phase, green CI before the next.
- Before adding a Go dependency, ask. The ceiling is stdlib + `go-toml`.
- Every bug fix ships with a doctor check and a test.
- A new provider is a data file and templates. If it needs Go changes, stop and
  explain why.
- Budgets stay asserted in CI.
- Conventional commits. When a principle is bent, record it in
  `docs/decisions.md` — that is how this revision itself is documented
  (ADR-006 theming, ADR-007 version-gated workarounds, ADR-008 the launch verb,
  ADR-009 isolation).
