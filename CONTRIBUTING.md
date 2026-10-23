# Contributing

Most of what you need is already written down; this file says where.

**Adding a provider** — a terminal, multiplexer, file browser, editor or agent —
is [`docs/adding-a-provider.md`](docs/adding-a-provider.md). It is usually one
TOML file in `slots/` and no Go. If it appears to need Go, please open an
issue: that indicates a gap in the slot model rather than a problem with the
provider.

**Why things are the way they are** is [`docs/decisions.md`](docs/decisions.md).
It is longer than the code. A change may contradict a decision recorded there;
name which one, so the reasoning can be followed rather than rediscovered.

**The rules the code is written to** are in [`CLAUDE.md`](CLAUDE.md). Three
carry most of the weight:

- **`make check` before every commit.** Lint, tests, cross-compilation for four
  targets, and the size budgets. The budgets are failing checks: code is capped
  at 9,000 lines and comments at 30% of it. They bite regularly and that is
  their job.
- **Comments record intent, never history.** A comment says why the code is
  shaped the way it is, so that changing it badly is harder. Git holds what it
  used to be.
- **A bug fix ships with a test or a doctor check**, and the test name carries
  the story.

**Conventional commits**, and `main` requires a pull request.

## Reporting something

Bugs and provider requests have forms; both ask for `bothy doctor --json`
or its equivalent, because it answers most of what would otherwise be a round
trip. A suspected security hole goes through
[SECURITY.md](SECURITY.md) rather than the tracker.

## Evaluating rather than contributing

The codebase is under nine thousand lines of Go by design, so it can be read in
full. That is the intended way to decide whether to trust it.
