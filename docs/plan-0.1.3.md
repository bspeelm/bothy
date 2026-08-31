# Plan — 0.1.3

The milestone that makes "portable" a claim with evidence behind it.

`PLAN.md` §10 ends with one sentence about what is left before v0.2.0:

> Remaining before v0.2.0: run it on a distro that is not Fedora. Everything is
> verified on Fedora Silverblue, a Fedora toolbox, and a bare
> `fedora-toolbox:44` — which is three environments and one distribution.

That is this release. bothy exists to make a setup portable, and it has never
been run anywhere but Fedora. Fixing that turns out to need three things, in an
order that matters:

1. **The code should be worth porting.** There is dead code and there are
   comments describing code that no longer exists. Discovering those halfway
   through a port is the expensive way to find them.
2. **Ubuntu has to actually work** — which means going and finding the
   breakage, not assuming there is none.
3. **CI has to keep it working.** Otherwise "runs on Ubuntu" is a claim resting
   on exactly the evidence the sentence above complains about: one person, one
   machine, once.

Two further items were considered for this release and moved to **0.1.4**: an
apt distribution path, and a scheduled job that watches for upstream tool
releases. Both are easier once a container job exists to test a `.deb` inside,
and neither blocks the portability claim. Holding them keeps 0.1.3 shippable.

## Decisions

| | |
|---|---|
| **Cleanup** | Cut restatement and stale prose. Keep every comment that explains *why*. ADR-010 stands. |
| **apt** | goreleaser `nfpms:` producing a `.deb` per release. No hosted repository, no PPA, no signing key. **0.1.4.** |
| **Version watching** | A weekly job that opens or updates one tracking issue. Not a PR, not an auto-updater. **0.1.4.** |

---

## Part 1 — Cleanup

The request that started this was "remove AI slop and over-commenting". The
survey found no AI slop: there is not one `// Step 1:`, `// loop over`, `TODO`
or `FIXME` in any first-party file. What it found instead was more specific and
more worth fixing.

It also ran straight into ADR-010, which is still right. When the source budget
was breached, cutting comments to pass was rejected because *"this project's
comments are the part worth keeping… they are where the reasons live."* Nothing
below contradicts that. Removing code with no callers is not removing reasons,
and a comment describing code that no longer exists is not a reason — it is a
wrong answer to a question a reader is about to ask.

The line, stated so it can be applied consistently: **delete comments that
restate the code or describe something gone; keep comments that explain why the
code is shaped the way it is.**

### Dead code

Each verified as having zero callers.

| Location | What |
|---|---|
| `internal/doctor/doctor.go:684-716` | `shadowable` and `whichAll` — remnants of the PATH-shadowing check that `checkToolProvenance` replaced. ~35 lines, fully documented, never called. |
| `cmd/bothy/main.go:379-382` | `newFlagSet`, a one-line wrapper used once while every other subcommand calls `flag.NewFlagSet` directly. |
| `internal/install/install.go:356-358` | `assetBytes`, a one-line alias for `bothy.Templates.ReadFile` used only by a test. |
| `internal/config/config.go:269-272` | `Incomplete()` is `return c.Validate()` beneath a doc comment asserting a distinction between the two. There is none. |
| `assets.go:38` | `Lock()` returns an error that is structurally always nil, which every caller must then handle. |

### Comments that are wrong

These are defects rather than clutter — a reader who trusts them is misled.

- **`internal/doctor/doctor.go:370`** — `checkTerminalCapability`'s doc comment
  is stapled on top of `checkYaziPlugins`'s. So `checkYaziPlugins` opens with a
  paragraph about a different function, and `checkTerminalCapability` at `:426`
  has no doc comment at all.
- **`internal/layout/layout.go:36`** — "see SizeNote in Render." There is no
  `SizeNote` anywhere in the repository.
- **`internal/config/config.go:262`** — explains that cross-field rules like "a
  PRO variant needing a pack" are checked at install time. ADR-006 removed
  variants and packs; neither word survives anywhere else in the code. Same
  file at `:173`, "expanding ~ in the pack path".
- **`internal/state/manifest.go:9`** — "Phase B needs their versions and
  checksums." Phase B is a `PLAN.md` phase, completed some time ago.
- **`cmd/bothy/dev.go:74`** — two paragraphs sit above `ensureInstalled`; the
  first describes the `decideLaunch`/`spawnTerminal` block eight lines below it.

### Duplicated helpers

`doctor.go:120` carries this, on `Env.lookPath`:

> This exists because forgetting it has now caused the same bug four times.

The helper that fixes the bug has itself been written four times. All four
share the same `os.Stat` + `!IsDir()` + `Mode()&0o111` triple:
`doctor.go:125`, `dev.go:165` (`lookPathIn`), `install/tools.go:124`
(`ToolPath`) and `:138` (`InstalledBinary`). `InstalledBinary` is the canonical
one; the others should call it.

Also: `expand` (`config.go:274`) and `expandDir` (`main.go:365`) are the same
function bar a trailing `filepath.Abs`; `fileExists` is byte-identical in
`main.go:440` and `install/uninstall.go:109`; and `render.go` repeats its
`commentPrefix` lookup with a `"#"` fallback at both `:44` and `:113`.

### Two files that outgrew themselves

`internal/doctor/doctor.go` is 716 lines holding all 19 checks plus the types
and the registry. `layoutcheck.go` already demonstrates one-check-per-file and
nothing followed it. Split into `doctor.go` (types and registry) plus
`checks_yazi.go`, `checks_terminal.go`, `checks_tools.go`,
`checks_isolation.go`.

`cmd/bothy/main.go` is 452 lines mixing dispatch with `tilde`, `expandDir`,
`fileExists`, `pids`, `mark` and `printTools`, and holding the full bodies of
`cmdTools`, `cmdUninstall` and `cmdConfig` — while `cmdDev`, `cmdDesktop` and
`cmdLock` already live in their own files. Follow the convention that is
already half-established.

One constraint on that second split, which is not obvious: `cmd/bothy/docs_test.go`
reads `main.go` **by filename** and greps it for `case "<cmd>"` arms and for
`const usage =`, checking both directions against the README's command table.
The switch and the usage string have to stay in `main.go`. Moving function
bodies is fine; moving a `case` arm breaks `make check`.

Both splits are pure moves and should be committed as pure moves. A large
diff with no behaviour change is the easiest place in the world to hide a
behaviour change.

---

## Part 2 — Ubuntu

Less is broken than expected, and what is broken is well-localised.

CI already runs `make check` on `ubuntu-latest`, so the unit tests pass there
today. `bootstrap/install.sh` consults `uname` and no package manager. The
binaries are `CGO_ENABLED=0`. And distro dispatch has exactly one consumer in
the whole codebase — `advice.Command` at `internal/advice/advice.go:59`, which
tries `[]string{DistroID, OS, "default"}` in order. Nothing else branches on
which distribution it is running on.

What has never run on Ubuntu is `bothy install` and `bothy doctor`.

### `bothy attach` cannot hop into a distrobox

`cmd/bothy/dev.go:199` tries `toolbox` and stops. `cmdDev` at `:180` correctly
falls back to `distrobox`; attach never learned. On an Ubuntu host — where
distrobox is the realistic choice — `bothy attach` skips the container hop
entirely and reports "zellij is not installed", which is true of the host and
irrelevant to the question.

### Named podman containers are detected as Toolbx

`internal/platform/platform.go:166` reads a `name=` line in
`/run/.containerenv` as evidence of Toolbx, on this reasoning:

> A Toolbx container always has a name in .containerenv; a bare podman
> container usually does not.

The second half is wrong. `/run/.containerenv` is written by podman, not by
Toolbx — on this machine it reads `engine="podman-5.8.4"`, `name="bothy-test"`,
`rootless=1` — and podman names every container it runs. So every
`podman run --name …` is a Toolbx as far as bothy is concerned.

The consequence is narrower than it first appears. `SharedHome` is set from the
container kind and then read nowhere in production — only by a test. What
actually goes wrong is `ContainerName`, which feeds `config.ContainerFor` and
so has bothy on the host attempt `toolbox run --container <a-podman-name>`, a
hop that cannot succeed.

The fix is to require `/run/.toolboxenv`, which Toolbx does write, and let a
named podman container be `Generic`. Losing `ContainerName` for those is the
repair, not a regression.

Two things make this more than a one-line change. First, **a test currently
asserts the bug is correct**: `platform_test.go:63` skips only when
`/run/.containerenv` is absent — a precondition any named podman container
satisfies — and then asserts `SharedHome == true`. Second, container detection
is untestable as written, because it reads absolute paths. Parameterising it on
a root (`detectContainerIn(root string)`) costs about ten lines and buys a
table test over the marker files: toolboxenv, distrobox, named-podman without
toolboxenv, dockerenv, and none of the above.

### The `xdg-open` shim is written where it cannot work

`internal/install/install.go:226` writes the host-forwarding shim whenever
`InContainer()` — but the shim forwards via `flatpak-spawn --host`, which
exists only under Toolbx and Distrobox. In a plain podman or docker container
bothy writes a shim that cannot function.

Worse, it then reports success. `checkOpener` asks only whether `xdg-open`
resolves, and `Env.lookPath` checks bothy's own bin first — so it finds the
broken shim and passes. Gate the shim on there actually being a host to forward
to, and let the check skip with a reason where there is not one.

### Smaller things

- `internal/doctor/doctor.go:146` hardcodes `toolbox run -c …` as its fix text
  regardless of container kind. A distrobox user needs `distrobox enter`.
- `slots/advice/ghostty.toml` has two `[[avoid]]` entries, both Fedora COPRs,
  and `Warnings()` prints them unconditionally. An Ubuntu user is told to avoid
  a COPR. Give `Avoid` a `distros` field and filter on it. The `ubuntu` and
  `debian` install strings are honest as they stand — Ghostty genuinely
  publishes no package for either.
- `osRelease()` at `platform.go:128` reads `ID` and `VERSION_ID` and ignores
  `ID_LIKE`, so Mint, Pop!_OS and every other Ubuntu derivative fall through to
  the generic advice. Parsing it and inserting it after `DistroID` in
  `advice.Command`'s key list is about five lines.
- `checkTerminfo` has no useful answer on Ubuntu. `infocmp xterm-ghostty` fails
  and no apt package provides the entry, while the non-container fix text
  ("install the terminfo entry for …") tells the reader nothing they did not
  know. Either give the `tic` route or downgrade to a warning off-Fedora — but
  decide, rather than shipping an unactionable failure.

### Decided, not fixed

Ubuntu ships `fd` as `fdfind`. `tools.Resolve` looks for `fd`, misses, and
downloads bothy's own copy. That is arguably correct — bothy pins versions on
purpose — but it means Ubuntu users always fetch a tool they already have.
Either the tool schema grows alternate binary names, or the reason gets written
down. Not both, and not neither.

---

## Part 3 — Container CI

One `container` job on `ubuntu-latest`, driving Docker from a Go test behind a
`//go:build container` tag, with a subtest per image.

Not a shell script: this project has exactly one shell file and that is a
decision, not an accident (ADR-001). Not a `container:` job either — those run
as root with GitHub's own `HOME`, and leave `run: |` shell as the only place to
put an assertion. The assertions here are a set comparison over a
`doctor.Report` and a walk of a filesystem tree. Both are pleasant in Go and
miserable in shell.

Three things make this cheaper than it looks. `bothy doctor --json` already
exists and `Result`/`Report` are already json-tagged — `PLAN.md` §8 asked for
it and Phase D built it. `bothy install --offline` already exists. And
`make budgets` counts `find cmd internal -name '*.go' -not -name '*_test.go'`,
so a test file costs nothing against the 5000-line cap.

### Shape

**Docker on the runner**, because Docker writes `/.dockerenv` and is detected
as `Generic`, so nothing is misdetected. The podman fix in Part 2 still has to
happen — otherwise reproducing a CI failure locally, on a Fedora machine with
podman, exercises a different code path from the one that failed. A test whose
local reproduction differs from CI is the kind of thing this project removes.

**`fedora:44` and `ubuntu:24.04`**, plain — not `fedora-toolbox:44`. The
toolbox image already ships git and ncurses, and it is the environment
`PLAN.md` §10 says is *already* verified by hand. The plain image is the honest
"nothing is here" baseline, and it is what tests whether the README's list of
what you need first is actually complete.

**The container's `$HOME` is a bind-mounted host directory**, so every
filesystem assertion is a `filepath.WalkDir` on the runner side, reusing the
shape of `snapshot()` in `internal/install/isolation_test.go`. No `docker
diff`, no dependence on what the base image happens to carry.

**One job, not a matrix.** A matrix produces per-entry check contexts, so
`.github/rulesets/main.json` would need an entry per image and adding a distro
later would mean either editing the ruleset or quietly having a non-required
check. One job with subtests means one context, and adding `debian:12` is a
line in a Go slice. The cost is serial execution, four to six minutes.

**Root prep lives in one visible map in Go**, not scattered through YAML,
because it is the only distro-specific thing in the test and it should be easy
to check against the README. Fedora needs `git ncurses` — `infocmp` lives in
`ncurses`, and `checkTerminfo` shells out to it. Ubuntu needs `git
ca-certificates`; without CA roots, Go finds no TLS trust store and every
download fails.

### What it asserts

With `TERM=xterm-256color` and `slots.agent none`, the expected result is 13
pass, one warning (`terminal-capability`, which has no graphics terminal to
find), five skips, and no failures.

Assert it twice. Coarsely: `bothy doctor` exits 0 — the check a user would
make. Strictly: parse `--json` and compare the whole ID-to-severity map,
**asserting the report's set of IDs equals the table's set of keys**. That
second half is the point. A twentieth doctor check then fails this test until
somebody decides what it means on a headless machine, instead of quietly going
uncovered. A plain allowlist of permitted failures would not do that.

`slots.agent none` is tuning the configuration until the test passes, which
deserves saying out loud rather than burying. The justification is real — the
README is explicit that bothy does not install the agent, so a CI job demanding
`claude` would be testing npm. It gets paid for with a counter-assertion: under
the default configuration, `doctor` must fail, and `agent` must be the *only*
failing ID. That proves the check still fires.

An offline subtest runs first, because it is hermetic and takes seconds:
`bothy install --offline` exits **1** — via `runDoctor`, not the install — with
exactly `yazi-config-discarded`, `zellij-config` and `tool-provenance` failing.
Asserting a specific exit code is a stronger claim than asserting a non-zero
one.

The online run downloads all nine tools every time, and that is deliberate.
Release assets are CDN redirects rather than API calls, so the unauthenticated
rate limit is not the risk to design around, and 131 MB from a GitHub runner is
about twenty seconds. Caching into bothy's bin would do nothing anyway, because
`EnsureTools` resolves through `exec.LookPath` and never consults it. And the
realistic failure — upstream re-tagging a release, producing a checksum
mismatch — is a true positive about `bothy.lock`. That is what the checksum
gate is for. Papering over it would defeat the point.

### What it will find

Snapshot the home directory before install; afterwards, every path must be
under `.local/share/bothy/`, `.config/bothy/`, or be the binary itself. After
`uninstall`, at most `config.toml` remains.

**That mid-run assertion is expected to go red the first time it runs, and that
is the test earning its keep.** `install.SessionEnv` sets `PATH`,
`ZELLIJ_CONFIG_DIR`, `YAZI_CONFIG_HOME`, `EDITOR`, `VISUAL`, `VIMINIT` and
`BOTHY_SESSION` — but not `XDG_CACHE_HOME`. It is also the doctor's `ToolEnv`,
so `checkYaziConfigDiscarded` runs `yazi --clear-cache` against `~/.cache/yazi`
and `checkZellijConfig` runs `zellij setup --check`, both outside bothy's tree.

`internal/install/plugins.go:118` already names this exact gap:

> The isolation guarantee covers what bothy writes. It does not automatically
> cover what the tools bothy runs decide to write, and that gap has to be
> closed one subprocess at a time.

This is the first thing in the repository that would *measure* that gap instead
of closing it by hand one subprocess at a time. When it fires, the fix is to
point the XDG cache, data and state directories inside bothy's tree — not to
widen the allowlist.

### Required, but not on day one

Land it unrequired, watch three or four pull requests, then add `container` to
`.github/rulesets/main.json`. And assert the subtests actually ran, the way the
`isolation` job does — this project has already shipped one green job that
tested nothing, and the lesson was expensive enough to apply twice.

### One thing found along the way

`make build` does not set `CGO_ENABLED=0`; `.goreleaser.yaml` does. So the
binary `make budgets` weighs against the 10 MB cap is not the binary that
ships, and a locally built one carried into a Fedora container is linked
against the runner's Ubuntu glibc — which would test glibc compatibility rather
than bothy. One word, but it moves the budget number, so it lands on its own.

---

## Also worth doing

- **Three packages have no test file at all**: `internal/config` (282 lines),
  `internal/render` (244) and `internal/state` (107). `config.Set()` is a
  45-case switch with no coverage. `render.inRoot` is what enforces the ADR-009
  isolation guarantee that an entire CI job exists to protect. And `state` is
  the manifest `uninstall` reads to decide what to delete — the one file where
  a bug means either leftovers or removing the wrong thing.
- `bothy install` **exits 1 in any container under the default configuration**,
  because `runDoctor` exits non-zero when anything failed and `checkAgent`
  fails without `claude` on PATH. Defensible, but surprising from outside, and
  worth a line in the README.
- `make budgets` uses `stat -c%s`, which is GNU-only and will break on macOS —
  a stated future target.
- ADR-009 says filling in the toolset costs 48 MB; `README.md` says 131 MB. One
  of them is stale.

## Deferred to 0.1.4

**apt.** Add `nfpms:` to `.goreleaser.yaml` so each tag also produces
`bothy_x.y.z_amd64.deb` and its arm64 sibling, attached to the GitHub release
and installed with `sudo apt install ./bothy_*.deb`. No hosted repository and
no signing key, which means no `apt update` upgrades — the honest trade for
zero ongoing maintenance on a project of this size.

Two details that will bite. The existing `archives:` block deliberately carries
no version in its name so that `/releases/latest/download/` resolves without
the bootstrap script knowing a version; the nfpm artifacts need their own
naming and must not disturb that. And the version has to stay single-sourced —
both `make release-tag` and `make copr` derive it by reading `Version:` out of
`packaging/bothy.spec`.

**Watching for tool updates.** A weekly workflow that runs `bothy lock`, diffs
the result, and opens or updates a single tracking issue listing pinned against
latest. The engine already exists: `fetch.Relock` resolves every tool's latest
release and `cmdLock` already prints `name: old -> new`.

An issue rather than a pull request, and never an automatic bump. `PLAN.md`
§11 lists auto-updaters as a non-goal, and `internal/fetch/lock.go` says why
the lockfile is regenerated deliberately: *"an installer that quietly moves its
own pins is an installer whose output nobody can reproduce."* A bot that opens
a PR is one merge away from being that installer.

Two cautions for whoever builds it. `fetch.LatestRelease` sends no
`User-Agent` and no authentication, so it draws on the 60-requests-per-hour
unauthenticated limit from a shared runner IP — pass `GITHUB_TOKEN`. And
`Relock` downloads every asset for all four platforms to compute checksums from
the real bytes, which is slow and is exactly the property that makes the
lockfile worth anything.
