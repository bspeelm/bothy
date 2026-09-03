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

Revision 1 set up your machine: it wrote `~/.config/yazi/yazi.toml`, `~/.vimrc`,
six `git config --global` keys and a `~/.bashrc.d` fragment, backing up whatever
was there. It worked, it was tested, and it was the wrong shape — a tool that
rewrites your dotfiles is too large an ask for what it gives back.

The decision and what it cost are
[ADR-009](decisions.md#adr-009--bothy-is-isolated-it-brings-its-own-config-tree).
The full account of what changed is in [`history/`](history/), where the plans
that shipped live.

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
`~/.bashrc.d`, no `git config --global`. `bothy uninstall` removes the tree and
the binary, and names the three things it leaves — the settings, the container
image, and the desktop entry. See the README for the list it prints.

The settings directory surviving is the answer to portability: clone it onto a
new machine, run `bothy install`, and you have your workspace. One folder, not a
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
6. **Budgets are real.** Binary ≤ 10 MB, code ≤ 6k lines, comments ≤ 22% of
   code — all three asserted in CI, as failing checks in the `Makefile`.
   Workspace idle RSS ≤ 200 MB excluding the agent is a design target and is
   **not** measured; saying otherwise claimed a check that does not exist.
   The code figure began at 5k and rose once, deliberately (ADR-026); the
   comment figure became a ratio when a total proved to be measuring code and
   prose together (ADR-021), and tightened from 25% to 22% once the headroom
   turned out to have been spent on retelling rather than reasoning (ADR-029).

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
| 18.7 | 14.5 | 6.9 | 2.3 | 2.3 | 2.0 | 1.6 | 0.6 | **48.9 MB** |

Linux x86_64 release assets for the pins in `bothy.lock`, measured 2026-09-03
against the GitHub release API. Unpacked on disk is larger; see ADR-009.

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

Five slots, each filled by a provider that is one TOML file in `slots/`. Three
profiles ship. One eleven-token palette drives every tool that can be pointed
at a config.

The model and its three tiers are [`north-star.md`](north-star.md) §4; the file
format is [`adding-a-provider.md`](adding-a-provider.md); what a user does with
either is [Profiles](https://github.com/bspeelm/bothy/wiki/Profiles) and
[Swapping parts](https://github.com/bspeelm/bothy/wiki/Swapping-parts-and-theming).

One thing here that belongs to the code rather than to either: the renderer owns
the two KDL traps — `split_direction="vertical"` means columns, and plugin nodes
are always emitted multi-line — because Zellij's spelling should not leak into a
profile someone writes by hand.

---

## 8. Doctor

Every setup failure that gets fixed ships with a check that detects it, which is
why the list only grows and why it is the most valuable thing in the project.

What each check does, how to read a report, and the five capabilities it groups
into are [The doctor](https://github.com/bspeelm/bothy/wiki/The-doctor). The IDs themselves are `Checks()` in
`internal/doctor/doctor.go`, which is the only list that cannot go stale.

Two rules the code enforces rather than documents: a failure without a fix line
fails a test, and a check naming a capability outside the five fails a test.

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

Phases A through F are done, and the milestones after them shipped as 0.2.0
through 0.8.1. The record is [`history/`](history/) — the plans as they were
written — and the releases themselves.

Kept as a numbered section rather than deleted because twenty files cite
`PLAN.md` by section, and a roadmap that has been walked is not worth
renumbering the architecture over.

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
