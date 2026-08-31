# Plan — 0.1.4

The release that finishes the distribution story and stops the pins going stale
quietly.

0.1.3 made bothy portable and proved it: a container job installs it into
`fedora:44` and `ubuntu:24.04` on every pull request. What it did not do is
give an Ubuntu user a way to *get* bothy that looks like the way they get
anything else. The install table offers a curl script, a Copr repository, `go
install`, and a git clone — one of which is Fedora-only and three of which are
not what a Debian-family user reaches for first.

The other thing 0.1.3 left open is quieter and worse. `bothy.lock` pins nine
tools by version and checksum, and nothing anywhere notices when one of them
moves. The pins are only correct on the day they are written; after that they
age silently, and the only way to find out is to run `bothy lock`, which
downloads roughly half a gigabyte to tell you.

So: **apt, and a job that watches the pins.** Plus four small things carried
over from 0.1.3, and one defect found while planning this that is more urgent
than any of it.

---

## Before anything else — 0.1.3 has no rpm

**There is no Copr build for 0.1.3.** The newest is 0.1.2, built during the
0.1.2 session. Anyone on `sudo dnf install bothy` gets a bothy with none of the
Ubuntu work in it, and the release notes say otherwise.

The package itself is configured correctly — `source_type: scm`,
`auto_rebuild: True` — so this is the webhook not firing, not the package being
wrong. `packaging/README.md` already documents every way that hook fails, by
symptom, and ends with "Verified working: a tag push produced a Copr build nine
seconds later." Both cannot be true now.

Two things to do, in order:

1. **Publish 0.1.3 by hand.** `make copr` hands Copr the tag directly and does
   not touch the webhook. (A submission was attempted while writing this plan;
   Copr became unreachable before it could be confirmed, so this needs checking
   at the builds page rather than assumed.)
2. **Diagnose the hook.** GitHub → Settings → Webhooks → the Copr hook →
   *Recent Deliveries*, looking for the `v0.1.3` tag. The table in
   `packaging/README.md` reads the answer off the response code: no delivery at
   all means it is subscribed to Push rather than *Branch or tag creation*; 415
   means the content type reverted to form-encoded; 404 means the URL lost its
   `/bothy/` suffix. That tab needs a logged-in browser session, so it is yours
   to read, not mine.

While in that file: line 80 says the webhook "is not set up" and line 128 says
it was verified working. One of those has been stale since the day it was
written, and the contradiction is why nobody noticed the hook had stopped.

**`make copr` also blocks.** `copr-cli buildscm` watches the build to
completion with no `--nowait`, so the command sits for several minutes with no
output. Worth a `--nowait` and a printed build URL, or at minimum a line in the
help text saying it will hang.

---

## Part 1 — apt

A `.deb` on each GitHub release, built by goreleaser's `nfpms:` block from the
binary of that same tag. No hosted repository, no PPA, no signing key.

The asymmetry with Fedora is deliberate and is the part worth arguing. A PPA
means carrying `debian/control`, `debian/rules` and a `debian/changelog` — a
second packaging syntax and a second place for the version to live, when
`make release-tag` and `make copr` go to some trouble to have only one. It also
wants a series per Ubuntu release, so the matrix grows every six months whether
or not anything changed, and it is Ubuntu-only, so Debian and Mint would still
be downloading a file.

Hosting an apt repository is worse. It needs a GPG key kept, kept secret and
kept alive, and a signed `Release` file that *expires*. The failure mode is not
"no new version" but "every existing user's `apt update` starts erroring on a
date I have forgotten about" — principle 7 exactly, an automated path whose
failure is silent until it is someone else's problem. Copr is tolerable because
Fedora runs it; the Debian equivalent is a thing I would be running.

The cost belongs in the open rather than in a footnote: **`apt upgrade` will
never bring you a new bothy.** dpkg tracks it and `apt remove` removes it, but
nothing on the machine knows where a newer one would come from. Anyone who
wants updates to arrive on their own should use the install script, which
resolves `/releases/latest/` on every run.

### The block

`nfpms:` goes between `archives:` and `checksum:` — the things built, then the
things computed over them. `checksums.txt` picks the deb up with no change,
because the checksum pipe hashes every uploadable artifact.

Details that are not guessable and were checked rather than assumed:

- **`ids: [bothy]`, not `builds:`.** `nfpms.builds` was renamed in goreleaser
  v2.5; the old spelling still parses but deprecates. The file already uses
  `archives.formats`, which is v2.6, so the floor is high enough.
- **`file_name_template: "{{ .ConventionalFileName }}"`.** The default puts the
  OS in the middle (`bothy_0.1.4_linux_amd64.deb`), which is not the Debian
  convention. The helper already carries the `.deb` suffix and goreleaser does
  not double it.
- **`formats: [deb]` and nothing else.** One more word would emit an rpm, and
  the result would be two packages named `bothy`, built from different inputs
  by different systems, with nothing to say which one a machine ought to have.
- **`bindir: /usr/bin`**, matching the spec's `%{_bindir}`, or a machine that
  had both would end up with two bothys.
- **`maintainer:` must be set** — goreleaser only warns, and nfpm then ships a
  deb with a blank `Maintainer:`, which lintian flags and some tooling rejects.

The two naming conventions coexist because they are separate pipes with
separate keys: `archives[].name_template` stays versionless so
`/releases/latest/download/` works for `bootstrap/install.sh`, and the deb
carries its version because Debian expects that and nothing resolves it by URL.
The header comment in `.goreleaser.yaml` currently explains the first rule as
though it were the only one, and needs extending so the second is not read as a
mistake. `bootstrap/install.sh` needs no change and should get none.

### No `Depends:`

The spec's reason for declaring nothing is about the nine tools in
`bothy.lock`, and does not transfer — `git` and `ncurses-bin` are not tools
bothy manages. So this needs its own argument. Three, and they agree:

1. **Symmetry.** If the deb declares `git` and the rpm does not, "what does the
   bothy package require" has two answers depending on which distro you ask.
2. **It trades a good diagnostic for a bad error.** A missing `git` is not a
   broken install: `bothy doctor` names it and hands you the command, and both
   `checks_yazi.go` and `plugins.go` degrade rather than crash. PLAN.md §3 says
   every bug becomes a doctor check; the doctor *is* the mechanism here, and
   `Depends:` would pre-empt it with an apt failure.
3. **The size surprise.** `git` pulls perl and friends. Someone installing a
   package advertised as one static binary should not get a perl interpreter.

`Recommends:` is the Debian-idiomatic middle ground and was considered. Rejected
because rpm has weak dependencies too, so taking it honestly means editing the
spec as well — a decision about both packages, not about apt.

The useful consequence: with no dependencies at all, `sudo dpkg -i` works
standalone and the usual `apt -f install` follow-up is never needed.

### Verification, and where it does *not* go

**Not in `cmd/bothy/container_test.go`.** That harness asserts through
`box.snapshot()`, which walks the bind-mounted `$HOME` host-side; a deb installs
to `/usr/bin`, invisible to every assertion the file owns. `start()` also copies
the local binary to `$HOME/.local/bin` and puts it first on `PATH`, so a
deb-installed `/usr/bin/bothy` would be shadowed. And the artifact does not
exist at PR time.

Instead, a small `deb` job on `ubuntu-latest`, which already has dpkg and apt.
`goreleaser release --snapshot --clean` builds the whole artifact set and
publishes none of it, then the job installs the deb, asserts `command -v bothy`
is `/usr/bin/bothy`, runs `bothy version`, asserts `dpkg-deb -f Depends` is
empty — the no-dependencies claim checked rather than assumed — and removes it.
Under a minute, no tool downloads.

This matters more than it looks: without it, the first time anyone runs
`apt install` on a bothy deb is *after* the tag is pushed and irrevocable. What
actually goes wrong with a first `.deb` is metadata — a blank maintainer, a
version dpkg rejects, `Architecture: linux-amd64`, the binary in the wrong
prefix — and all four are visible in `dpkg-deb --info` in one second.

`fetch-depth: 0` is required; goreleaser needs tags to compute a snapshot
version. `--snapshot` also runs the `before:` hook `go mod tidy`, which is
harmless on a throwaway runner but will rewrite `go.mod` in a tree you care
about — worth knowing before running it locally.

One thing CI cannot check, to be done by hand once around the 0.1.4 tag:
that installing a second `.deb` over the first upgrades in place rather than
erroring. That is the claim `packaging/README.md` will make about upgrades, and
it should be seen working once and recorded, the way the Copr section records
its nine-second webhook result.

### A version hole this exposes

`make release-tag` cannot drift, but a hand-made `git tag v9.9.9` can:
goreleaser would build a deb stamped 9.9.9 while Copr, reading the spec at that
commit, publishes an rpm at 0.1.4. That is already true of the tarball; the deb
makes it durable, because a wrong version in a package manager's database
outlives a wrong filename. The guard is four lines in `release.yml`, using the
same `sed` expression `release-tag`, `copr` and `.copr/Makefile` already share —
a fourth reader of one source, not a fifth source:

```sh
v=$(sed -n 's/^Version:[[:space:]]*//p' packaging/bothy.spec)
test "v$v" = "${GITHUB_REF_NAME}" || exit 1
```

Worth taking. The workflow's own comment says the gates re-run at the tag
"rather than trusting the branch"; this is that argument applied to the version.

---

## Part 2 — `bothy outdated`, and the job that runs it

`bothy.lock` pins nine tools and nothing notices when one moves.

The obvious implementation does not work. `bothy lock` is the only thing that
compares pinned against upstream, and it does so as a side effect of `Relock`,
which downloads **every asset for all four platforms** to compute checksums from
real bytes — roughly 500 MB and several minutes. That is deliberate and correct
(`internal/fetch/lock.go`: *"a lockfile whose checksums were copied from a
metadata endpoint rather than computed from the bytes bothy will actually run is
a lockfile that verifies nothing"*), and it is entirely unsuited to a weekly
check. Worse, **`bothy lock` returns nil even when every tool fails** — `failed`
only decorates the trailer — so a workflow can neither use its exit code nor
trust its silence.

So: a new command that does the cheap half.

### `bothy outdated`

Nine calls to `LatestRelease`, compared against the embedded lockfile. Zero
asset bytes. Every piece already exists and is exported — `fetch.LatestRelease`,
`fetch.VersionFromTag`, `fetch.LoadLock`, `Lockfile.Get`, `tools.Load` — nothing
has ever composed them.

```
$ bothy outdated
  zellij   0.45.1   ->  0.46.0
  yazi     26.8.15  ->  26.9.2

2 of 9 tools have newer releases.
Run 'bothy lock' to take them, then review the diff.
```

`--json` for the workflow. Exit 0 whether or not anything is outdated — being
out of date is a fact, not a failure — and non-zero only when the check could
not be *made*, which is the distinction `bothy lock` fails to draw.

It is a real command for a person, not a CI shim: "is anything stale?" is a
question worth being able to ask locally without downloading half a gigabyte.

Three things to get right:

- **Send a `User-Agent` and a token when there is one.** `LatestRelease` sets no
  headers at all, so nine unauthenticated calls draw on GitHub's 60/hour limit
  from a shared runner IP. `fetch.Client` is an exported package var, so a
  token-injecting `Transport` can be swapped in without touching
  `LatestRelease`.
- **A tool whose API call fails is reported as unknown, not as current.** The
  failure mode to avoid is a job that says everything is fine because GitHub
  was rate-limiting it.
- **`min_version` is not the floor.** It means "the oldest that actually works"
  and gates `tools.Resolve`; it has nothing to do with what is pinned.

### The job

Weekly `schedule:` plus `workflow_dispatch:`, running `bothy outdated --json`
and opening or updating **one** tracking issue. `permissions: issues: write` —
`ci.yml` currently declares no permissions block at all.

An issue, never a pull request, and never an automatic bump. PLAN.md §11 lists
auto-updaters as a non-goal, and the lockfile is regenerated deliberately for a
stated reason. A bot that opens a PR is one merge away from being the installer
that quietly moves its own pins. The ruleset settles it independently anyway:
`bypass_actors` is empty, so nothing can push to main without a review.

One issue that is updated, not one per week — otherwise the tracker fills with
near-identical issues nobody closes.

---

## Part 3 — the four carried over

**Make `container` a required check.** It has passed on every pull request it
has run on. One entry in `.github/rulesets/main.json` beside `check`,
`isolation` and `no-paid-palette`.

But **not before adding a pull retry**, because it has already failed once on
main — run 33406915292, `docker: received unexpected HTTP status: 502 Bad
Gateway` pulling the image. Three of four subtests passed; the fourth never got
a container. As an advisory job that is noise; as a required check it is a
registry hiccup blocking every merge. Pull both images in their own step with a
retry before the test runs.

**ADR-009 says 48 MB; the README says 131 MB.** Measured on this machine: ten
binaries, **124 MB**. The README is right and ADR-009 is stale by a factor of
2.6 — it was written against a smaller toolset and never revisited. Fix the ADR,
and say the number was measured and when, so the next reader knows what it is.

**`make budgets` uses `stat -c%s`,** which is GNU-only. macOS is a stated future
target and this is the first thing that would break there. `wc -c < $(BINARY)`
is portable and needs no branch.

**fd is `fdfind` on Debian and Ubuntu,** so `tools.Resolve` looks for `fd`,
misses, and downloads bothy's own copy even where the system has one. The
decision to make is not which is better but which is *bothy's*: gap-filling says
use what is there, and pinning says use what was verified. Either add an
`alt_binaries` field to the tool schema and check it in `Resolve`, or write down
that a differently-named system copy is not the copy bothy pinned and fetching
is correct. Not both, and not neither.

---

## Part 4 — three things found while planning this

None were on the list. All three are small and all three are wrong today.

**`bothy help` makes a claim that has been false since ADR-009.** The usage text
ends: *"Nothing is written without first backing up what was there."* There are
zero backup functions in the codebase — ADR-009 deleted the manifest, hashing
and restore machinery, and the entire point of isolation is that bothy has
nothing to back up because it never opens a file it does not own. The sentence
survived the revision that made it false, and it is printed to every user who
types `bothy help`. It should say what is now true: bothy writes only inside its
own directory, and `bothy uninstall` removes it.

**`TestEveryCommandIsInTheUsage` passes by accident.** It checks
`strings.Contains(usage, cmd)`, and `bothy lock` is absent from the usage text —
but "lock" is a substring of "unlocked" in the first line, *"a small, unlocked
terminal workspace"*. So the one command that is genuinely undocumented is the
one the test cannot see. Match whole words against the usage lines. This matters
now rather than later, because `bothy outdated` is about to be added and the
test is the thing meant to keep it documented.

**`make copr` blocks with no output.** `copr-cli buildscm` watches the build to
completion and the target passes no `--nowait`, so it sits for minutes looking
hung. Add `--nowait` and print the build URL, or say in the help text that it
will wait.

---

## Critical files

| Path | Why |
|---|---|
| `.goreleaser.yaml` | The `nfpms:` block; the header comment that explains only one of the two naming rules |
| `cmd/bothy/outdated.go` | **New.** The cheap pinned-vs-latest check |
| `internal/fetch/lock.go` | `LatestRelease` sends no headers; `fetch.Client` is the seam for a token |
| `cmd/bothy/main.go` | Dispatch and usage for `outdated`; the false backup sentence at :61 |
| `cmd/bothy/docs_test.go` | The substring match that lets an undocumented command through |
| `.github/workflows/ci.yml` | The `deb` job, and the image-pull retry `container` needs first |
| `.github/workflows/outdated.yml` | **New.** Weekly schedule + `workflow_dispatch`, `issues: write` |
| `.github/workflows/release.yml` | Tag-matches-spec assertion |
| `.github/rulesets/main.json` | `container` becomes required — after the retry |
| `packaging/README.md` | The Debian section; the stale "not set up" line at :80 |
| `docs/decisions.md` | ADR-013 (a file, not a repository); ADR-009's 48 MB |
| `README.md` | The apt row, "all four" -> "all five", the upgrade note |

## Sequencing

0. **Publish 0.1.3 to Copr and diagnose the webhook.** Before anything else —
   the current release has no rpm.
1. **Part 4's three fixes**, which are small, independent and wrong today. The
   `docs_test.go` fix comes before `bothy outdated` so the test can actually
   hold the new command to account.
2. **apt** — the `nfpms:` block, the `deb` job, the docs, ADR-013, the
   tag/spec assertion.
3. **`bothy outdated`**, then the workflow that runs it.
4. **The container pull retry**, then `container` into the ruleset.
5. **The three small carried-over fixes** — the ADR number, `stat -c%s`, fd.
6. `docs/plan-0.1.4.md` updated with what actually happened, then the release.

## Verification

1. `make check` green throughout. No Go changes in Part 1, so the budgets only
   move in Part 2 — `bothy outdated` is roughly 60 lines against 1078 spare.
2. `goreleaser release --snapshot --clean` locally, then `dpkg-deb --info` and
   `--contents` on the result: architecture, version, maintainer, and the binary
   at `/usr/bin/bothy`.
3. The `deb` CI job installs it, runs it from `/usr/bin`, asserts `Depends` is
   empty, and removes it.
4. By hand, once: a second `.deb` installed over the first upgrades in place.
   Record the result the way the Copr section records its webhook timing.
5. `bothy outdated` against the real API, and with `fetch.Client` pointed at a
   test server for the offline test — including the case where the API fails,
   which must report unknown rather than current.
6. The `outdated` workflow run once via `workflow_dispatch` before trusting the
   schedule, and confirmed to update its existing issue rather than open a
   second.
7. `container` promoted to required only after the pull retry has survived a few
   runs — and confirmed by watching a PR actually blocked on it, since a
   required check that is not really required is the failure this project has
   already had once.
