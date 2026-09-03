# Working on bothy

## Comments record intent, never history

A comment answers "why is this code shaped this way", so that changing it
badly is harder. It does not record what the code used to be. Git and
`docs/decisions.md` hold history; a source file that also holds it is a
changelog with a compiler.

**Keep** — the reason a choice is non-obvious:

```go
// Zellij below 0.45.1 cannot pass the Kitty graphics protocol, and its
// mangled query replies are parsed as keystrokes. Hence the version gate.
```

```go
// Passthrough must unset, not merely decline to set: the session inherits
// this environment, so leaving an inherited value in place points the tool
// at bothy's config anyway.
```

**Delete** — the same fact told as a story:

```go
// An earlier version returned early when offline and never recorded where
// the install happened, so `bothy` launched from the host could not find
// its way back. Skipping downloads is not a reason to skip bookkeeping --
// and this is the third short-circuit in this project to swallow a step
// someone added after it.
```

That paragraph says one useful thing: the manifest is written on every path,
offline included, because the launcher needs it. Say that.

The test: **would this comment still make sense to someone who had never
seen the previous version?** If it only lands as "we got this wrong once",
it belongs in a commit message.

Corollaries:

- A bug fix ships with a test or a doctor check, and the test name carries
  the story. That is where "this broke once" belongs.
- A 30-line function does not need 40 lines of preamble. If the reasoning is
  that long, it is an ADR.
- No self-narration. Comments describe the code, not the process that
  produced it, and not the person producing it.
- Prefer deleting a comment to writing one that restates the line below it.

## Read first, every session

`docs/north-star.md`, the ADR titles in `docs/decisions.md`, and the current
plan. The recorded refusals ride along for free — a decision already declined
is settled, and a re-proposal is answered with its ADR number rather than a
fresh argument.

## Everything else

- Dependencies: stdlib plus `github.com/pelletier/go-toml/v2`. See PLAN.md §13.
- `make check` before every commit: lint, tests, and the size budgets. Do not
  argue with a budget; write an ADR to change it.
- Conventional commits. `main` requires a PR.
- Every user-facing claim names the test or CI job that proves it, or the
  claim goes.
- Load-bearing surfaces are listed in `docs/reviewed.md`. Flag every change to
  one explicitly; `make ledger` says what is unread.
- A new result in a closed-world set goes in the expectation table. The test
  failure will say so.

## Instinct fences

Mine to self-check. Each is a failure that has actually shipped in agent-led
work, so the instinct is named before it fires.

- **Answer the ask.** Extra scope is proposed in prose, never shipped in the
  diff.
- **Fix in place.** No `_v2`/`_new`/`_old` parallel files; a replacement and
  the deletion it replaces travel in one commit.
- **Never discard an error** without a comment saying why ignoring it is
  correct.
- **Never weaken or delete a test to go green.** A failing test is
  information; report what it told you.
- **Never claim something ran** without the command and its output. Anything
  unverified is labelled unverified — including in reports about my own work.
- **No placeholders** without a named tracking issue. The TODO count may not
  grow.
- **One diff, one intent.** Refactors and fixes ship separately.
- **Give the strongest counter-argument first** on any proposal, and build
  only what survives it. Direction is settled by ADR, not by enthusiasm.
- **Polish is not proof.** The existence of docs, tests or ADRs is never
  offered as evidence of quality. Only executed checks count.
- **Before a release**, prepare the review packet: plain language, executable
  checks ("run X, expect Y"), unverified items labelled. The release is
  blocked until it is answered — do not work around that block.
- **Open the pull request when the work is done** and the checks are green,
  not before. Do not keep pushing to a PR under review, and do not arm
  auto-merge on a branch still being added to.
