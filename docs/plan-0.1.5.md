# Plan — 0.1.5, and what makes 0.2.0

The release where the claims and the code agree.

An external review of the whole tree produced twelve findings. Ten of them
shipped in 0.1.4, along with apt packaging, `bothy outdated` and checksum
verification of bothy's own download. What is left is smaller than it looked
when the review landed, and it divides cleanly along a line worth drawing in
advance.

**0.1.5 makes the existing claims true.** Two review findings, a slot that is
honestly half-implemented, and a config file that accepts typos in silence.
None of it is a feature. A release spent on making what is already claimed
actually true is a release worth cutting on its own.

**0.2.0 is macOS, verified.** The README's install table, the bootstrap script
and the lockfile have all carried darwin since v0.1.0, and CI has never proved
a line of it. That is the one item here that expands what bothy can honestly
say it supports, which is what earns the minor bump. It is deliberately not in
0.1.5: it is the largest piece, the one most likely to disappear into
runner-environment archaeology, and the only one whose absence would leave a
0.2.0 that was really a 0.1.5 wearing a bigger number.

### Where 0.1.4 left things

| Review finding | |
|---|---|
| `attach` resolved through the ambient PATH (#13) | shipped |
| `attach` ran the client without `SessionEnv` (#14) | shipped |
| Quoted agent arguments broke, and an empty one panicked (#15) | shipped |
| `VersionFromTag` mangled hyphenated tags (#16) | shipped |
| The bootstrap script verified nothing (#17) | shipped |
| Unquoted args in the attach container hop (#18) | shipped |
| `Extract` treated any unknown suffix as a bare binary (#19) | shipped |
| Deterministic temp names raced (#21) | shipped |
| `MaxFile` < `MaxAsset` read as a mistake (#23) | shipped |
| `bothy dev attach` bypassed flag parsing (#24) | shipped |
| **`Relock` is trust-on-first-use (#20)** | **0.1.5** |
| **`LoadProfile` round-trips through a temp file (#22)** | **0.1.5** |

Also carried over: `container` has been green on every run since its pull
retry landed, and is still only advisory (#32).

Everything below is filed and milestoned: 0.1.5 holds #20, #22, #30, #31 and
#32; 0.2.0 holds #33 alone.

---

## 0.1.5

### 1. Upstream provenance for the pins (#20)

`bothy lock` computes checksums from whatever GitHub served at lock time. A
release tampered *after* locking is caught; one tampered *before* is pinned
faithfully. The doc comment is honest about this, and pinning what you
downloaded is a legitimate model — but several of the pinned projects publish
their own checksums, and not reading them is leaving provenance on the table.

Measured, rather than assumed: five of the nine publish, in two shapes.

| | publishes | shape |
|---|---|---|
| zellij | yes | `<asset>.sha256sum` beside each asset |
| ripgrep | yes | `<asset>.sha256` beside each asset |
| jq | yes | `sha256sum.txt`, one manifest |
| lazygit | yes | `checksums.txt`, one manifest |
| fzf | yes | `fzf_<version>_checksums.txt`, one manifest |
| fd, yazi, delta, zoxide | no | — |

So this is one optional field in the tool TOML naming either a sibling suffix
or a manifest filename, and the code to fetch and compare. `Relock` prints
`upstream checksum matched` or `no upstream checksum published` per tool, and
a mismatch is a hard failure — the whole point is that it is the one place a
substituted binary could still get through.

No new failure mode for the four that publish nothing. The lockfile records
which entries were cross-checked, so the answer survives the run.

### 2. helix, resolved rather than footnoted (#30)

The README's swap table offers helix as an editor. helix ships as tar.xz,
which `Extract` refuses because the standard library cannot unpack it and
PLAN.md §13 caps dependencies at go-toml. So helix works if you already have
it and cannot be supplied if you do not — an asymmetry no other slot has, and
nothing tells the user about it.

**Resolution: keep the ceiling, fix the story.** helix stays use-only. The
swap table says so, `bothy doctor` reports "helix is configured and not
installed; bothy cannot supply it (tar.xz) — install it with <per-distro
command>" exactly as it already does for Ghostty and the agent, and
`docs/decisions.md` gets the ADR so the question stops being re-litigated.

The alternative — take `ulikunitz/xz`, a small pure-Go dependency — is
rejected for now because it is irreversible in a way this is not. A dependency
admitted for one tool's archive format is admitted forever, and §13 has held
precisely because every exception was refused. The ADR is where the evidence
accumulates if helix, or a second tar.xz tool, turns out to matter to real
users.

### 3. The config file stops accepting typos (#31)

`config.Load` unmarshals over the defaults, so `slots.borwser = "yazi"` loads
cleanly, does nothing, and keeps doing nothing on every machine forever. The
README tells people to put `~/.config/bothy` in git and carry it between
machines, which multiplies a typo rather than surfacing it.

**Warn, never error.** A config that refuses to load is worse than a typo, and
a key written by a *newer* bothy must not brick an older one, or the
carry-it-in-git story breaks in the other direction.

- `config.Load` collects unrecognised keys.
- Commands that load the config print one line per unknown key, naming the
  nearest valid key when the edit distance is small.
- A doctor check: `unknown key "slots.borwser" — did you mean
  "slots.browser"?`, with the corrected `bothy config set` line as the fix,
  per the rule that every failure carries one.
- When an unknown key resembles nothing, the message says "written by a newer
  bothy?" — because that is the other reason it happens, and the confused
  issue it would otherwise generate is predictable.

While attention is on this file: `config.Validate` checks one field. It grows
to cover the cross-field rules install already enforces implicitly — a slot
naming a provider that does not exist, a passthrough entry naming something
that is not a slot — so a `bothy config set` mistake surfaces at the next
command rather than as a broken workspace.

**What this does not do, deliberately.** An earlier draft of this plan framed
the release as "the config schema is settling". It is not, and saying so would
be the one claim the release could not back. Validating a schema is not
freezing it. If a stability promise is wanted, it is its own decision with its
own ADR, naming which keys are committed to and what a breaking change to them
would require. Until that is written, the schema is what the code accepts, and
this work only makes that set knowable.

### 4. Two smaller things

**`LoadProfile` writes an embedded profile to a temp file to read it back
(#22).** A `layout.ParseProfile([]byte)` that the file loader wraps removes
the round-trip, the cleanup and two error paths.

**Make `container` a required check (#32).** It has been green on every run since
its pull retry landed. Adding it to `.github/rulesets/main.json` is what stops
Ubuntu support regressing silently, and the retry is what makes that safe to
do without handing the merge queue to Docker Hub's availability.

### The 0.1.5 gate

1. #20 and #22 closed.
2. helix: the ADR written, the swap table honest, the doctor saying so.
3. Unknown config keys warn; the doctor checks for them; `Validate` covers the
   cross-field rules.
4. `container` is in the ruleset and has blocked at least one real PR, because
   a required check that is not really required is a failure this project has
   already had once.

---

## 0.2.0 — macOS, verified rather than claimed (#33)

The install table says "anyone on Linux or macOS". The bootstrap script maps
`Darwin`. The lockfile carries `darwin_x86_64` and `darwin_aarch64` for eight
of nine tools. CI proves none of it: the container job is Linux by
construction, and PLAN.md §10 has listed macOS as deferred since v0.1.0. The
claim has been ahead of the verification for the whole life of the project.

A `macos-latest` leg running the same contract the container job asserts on
Linux: build, install, the full doctor report checked against a table of
expected severities, uninstall, and an empty tree afterwards. What it exercises
for the first time:

- **Terminal and graphics detection off Linux.** No Ghostty and no display
  worth speaking of on a runner, so the severities table is where the correct
  macOS-CI answer gets pinned down rather than discovered by accident.
- **The darwin fetch paths.** delta publishes no Intel-macOS build, so the
  lockfile carries no checksum and install refuses. That refusal is currently
  implemented by code nobody has ever run on a Mac, and recording it as
  expected is the first regression guard on the rule that an absent checksum is
  a hard refusal and never an unverified download.
- **`shasum -a 256`** in the bootstrap script, which 0.1.4 added and only
  Linux's `sha256sum` has been exercised.

What it cannot do: the spawn-Ghostty path or image previews, for want of a
display and a capable terminal. Those stay hand-verified and dated in this
document, the way the Copr webhook result was.

**This is what makes it 0.2.0.** It is the only item in either release that
widens what bothy can honestly claim to support. If the leg cannot be made
green in reasonable time, the honest resolution is the README — macOS moves to
a "should work, unverified" note — and then there is no 0.2.0, only a 0.1.6.
The current state, claimed and untested, is the only unacceptable one.

---

## Deferred, unchanged

macOS *packaging* (brew), WSL2 beyond detection, tmux, `bothy update`,
alternate multiplexers, parallel agents. The 0.1.x non-goals survive intact.
