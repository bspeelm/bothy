# Vouching ledger

Which load-bearing surfaces a named human has read line by line, and at which
commit. `scripts/ledger.sh` compares each recorded commit against the file's
last change: differ and the entry is stale, and it says so. `make check`
reports; the release blocks.

**This ledger is allowed to show debt.** An unread surface is listed as unread.
That is the mechanism — a ledger with nothing outstanding is either finished or
dishonest, and only one of those is common.

## What counts as load-bearing

A bug here reaches outside bothy's own tree, or lets bytes and paths in from
somewhere else:

- receives bytes from a network
- deletes files
- touches credentials, tokens or keys
- writes outside `~/.local/share/bothy` and `~/.config/bothy`
- builds an argv for something that runs as you
- enforces the rules — a weakened check is the fence moved, not a fence

**Not everything that execs or writes.** Twelve files call `exec.Command` and
ten write files; taking all of them would mark about half the code load-bearing
and make the review an operating cost paid forever rather than a capital cost
paid once. Fixed-argv probes (`infocmp`, a version query) and writes confined
to bothy's own tree are excluded, and named below so the exclusion is a
decision rather than an oversight.

| surface | read by | at commit |
|---|---|---|
| `bootstrap/install.sh` | - | - |
| `internal/fetch` | - | - |
| `internal/install/uninstall.go` | - | - |
| `internal/install/plugins.go` | - | - |
| `internal/confine` | - | - |
| `cmd/bothy/desktop.go` | - | - |
| `Makefile` | - | - |
| `.github/workflows` | - | - |
| `scripts` | - | - |

Fill in a name and the short commit you read it at. `scripts/ledger.sh` does
the rest.

## Excluded, and why

| surface | why it is not here |
|---|---|
| `internal/doctor`, `internal/probe`, `internal/platform` | exec with a fixed argv to ask a question; they read, they do not write or fetch |
| `internal/config`, `internal/render`, `internal/state` | write only inside bothy's own two directories |
| `internal/mux`, `cmd/bothy/dev.go`, `cmd/bothy/terminal.go` | launch what the config names. **The weakest exclusion here** — #146 was a shell-splitting bug in `mux/zellij_render.go`, which is argv construction by another name. Promote it if a second defect lands there (§7.4). |
| `internal/tools`, `internal/slots`, `internal/theme`, `internal/layout` | data and decisions; the dangerous half is `internal/fetch`, which is included |

## Staleness

An entry is stale the moment its surface changes, and a surface stale beyond
**30 days** is a framework failure rather than a code one — the vouching has
stopped being real. Measured in time rather than releases: this project shipped
three times in two days, and a threshold that fires constantly is one that gets
ignored.

## Status

Run `make ledger`.
