# Contributing

Most of what you need is written down already; this file is the signpost.

**Adding a provider** — a terminal, multiplexer, file browser, editor or agent —
is [`docs/adding-a-provider.md`](docs/adding-a-provider.md). It is usually one
TOML file in `slots/` and no Go. If it seems to need Go, that is worth saying in
an issue: it means the slot model is wrong somewhere, which is more interesting
than the provider.

**Why things are the way they are** is [`docs/decisions.md`](docs/decisions.md).
It is longer than the code. If a change contradicts a decision in there, that is
fine and the record is not sacred — but say which one, so the next person can
follow the argument rather than rediscover it.

**The rules the code is written to** are in [`CLAUDE.md`](CLAUDE.md). Three
carry most of the weight:

- **`make check` before every commit.** Lint, tests, cross-compilation for four
  targets, and the size budgets. The budgets are failing checks: code is capped
  at 7,000 lines and comments at 25% of it. They bite regularly and that is
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

## If you are evaluating rather than contributing

The whole codebase is under six thousand lines by design, which means it can be
read in an afternoon. That is the intended way to decide whether to trust it.
