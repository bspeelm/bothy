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
| `Makefile` | BS | f75b503 |
| `.github/workflows` | BS | c9d545f |
| `scripts` | BS | cb4b840 |

## How to do the read

Signing a row does not mean the code is good. It means: **if this file does
something bad, you knew what it did.** Someone has to be answerable, and the
agent cannot be — that is the whole of it.

Read for five things. They are the five the release packet asks, so the read
is also the preparation for answering it:

1. Where does it write?
2. How do bytes get in, and what checks them?
3. What does it execute, and with what environment?
4. What does it delete, and what happens when that fails?
5. What does it claim, and which check proves the claim?

A file is read when you can answer those five from memory, citing the lines. If
you cannot, the read is not finished, whatever the ledger says.

Then fill in the row above: replace the two dashes with your name and the
short commit you read the surface at. `scripts/ledger.sh` compares that commit
against the file's last change and reports `ok`, `STALE` or `UNREAD`. No
example row is written out here, because the ledger reads its own table and
would count it.

**Do not sign what did not hold up.** A question you could not answer is a
finding. So is a file too tangled to hold in your head — and the answer to
that one is to shrink the surface, not to sign harder. Leave the `-` in place
and say what stopped you.

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
