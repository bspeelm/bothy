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

## Everything else

- Dependencies: stdlib plus `github.com/pelletier/go-toml/v2`. See PLAN.md §13.
- `make check` before every commit: lint, tests, and the size budgets.
- Conventional commits. `main` requires a PR.
