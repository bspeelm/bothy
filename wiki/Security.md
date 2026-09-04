# Security

What bothy verifies, what it deliberately does not, and where each wall ends.
To report a hole, use
[SECURITY.md](https://github.com/bspeelm/bothy/blob/main/SECURITY.md) — privately, through GitHub's security
advisories, not a public issue.

## What bothy downloads, and what checks it

bothy fetches eight tools. Every one is pinned to a version **and a sha256** in
[`bothy.lock`](https://github.com/bspeelm/bothy/blob/main/bothy.lock), and the checksum is verified before the file is
kept. A tool already on your `PATH` at a good version is used as-is and nothing
is downloaded.

Where a project publishes a checksum of its own, `bothy lock` compares the two
before recording the pin, and refuses outright if they disagree. Several
projects publish nothing, and for those the pin is whatever the maintainer
downloaded on the day they ran `bothy lock`.

**What that comparison is worth, precisely.** The published file sits in the
same release as the asset, so whoever can replace one can replace the other. It
rules out a substitution *after* publication, and a download corrupted or
intercepted on the way. It does not rule out a compromised release pipeline,
and no checksum can.

**The limit, stated plainly:** these are other projects' releases. A checksum
proves you got the bytes bothy expected — it does not prove those bytes are
good. bothy does not sign what it did not build.

## What bothy itself is signed with

Nothing you have to trust a key for. Release artifacts are signed by the GitHub
Actions workflow that built them, keyless, and recorded in a public
transparency log — so the binary can be traced to a specific run of a specific
workflow on a specific commit
([ADR-030](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-030--releases-are-signed-by-the-workflow-that-built-them)).

There is deliberately no private key: none to store, rotate, or leak.

**What that does not do:** it does not make a compromise impossible, it makes
one *visible and permanent*. If this repository's Actions were compromised, the
attestation would faithfully record the attacker's build — and that record
could not be quietly withdrawn.

**And it is not what Gatekeeper asks.** Apple wants an identifiable paying
developer account; the attestation answers a different question. Both are real,
neither substitutes for the other. See [Installing](Installing).

## `curl | sh`

The install script is fetched over HTTPS and run, unsigned, before bothy exists
to verify anything. **This is a real gap and it is not solved** — no signature
on an artifact fixes verifying the verifier.

What you can do instead: download it, read it, then run it. It is one file and
deliberately small.

```sh
curl -fsSL -o install.sh https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh
less install.sh
sh install.sh
```

Or skip it: `go install`, the `.deb`, or dnf all avoid the question.

## The wall around the agent

`bothy confine` runs the agent in a rootless podman container with your project
directory and the agent's own credentials mounted, and nothing else from
`$HOME`. It is **opt-in and there is no setting that turns it on**.

**What it stops:** every other project, `~/.ssh`, `~/.aws`, your shell history.
Verified rather than assumed — a test fails if the invocation ever mounts
`$HOME`.

**What it does not stop, on purpose:**

| | |
|---|---|
| the network | the agent calls its API; that is the job. This is a filesystem wall, not a network one |
| the agent's own credentials | mounted, or it cannot log in and the wall protects nothing you wanted |
| the project directory | mounted writable, because editing it is the point |

**Without `confine`, there is no wall at all** — and bothy says so rather than
implying otherwise. The agent runs as you, with your access, exactly as it
would if you had started it yourself. bothy is not making that worse; it owns
the launch, which is a position to make it better if you ask.

## What bothy does not do

- **No telemetry, no auto-updater, no background service.** A test fails if any
  shipping file names a host that is not GitHub.
- **It does not touch your dotfiles.** Not `~/.vimrc`, not `~/.config/yazi`,
  not your git config. It writes into its own tree and points tools there for
  one process
  ([ADR-009](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-009--bothy-is-isolated-it-brings-its-own-config-tree)).
- **It never asks for root.**

## Scope for a report

In scope: the release pipeline, the confinement wall, `bootstrap/install.sh`,
the Homebrew cask's quarantine removal, and the configs bothy generates for
other tools. Out of scope: what the agent does, and holes in the tools bothy
fetches — though a hole in *how bothy pins or verifies them* is very much in
scope. The full statement is
[SECURITY.md](https://github.com/bspeelm/bothy/blob/main/SECURITY.md).
