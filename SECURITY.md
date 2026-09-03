# Security

## Reporting

Report privately through GitHub's [security advisories][advisory] rather than
in an issue. If that is not available to you, anything that reaches the
maintainer works; a public issue is the one route to avoid for a real hole.

[advisory]: https://github.com/bspeelm/bothy/security/advisories/new

No bounty, and no fixed response time. bothy is one person's project and says
so rather than promising a schedule it cannot keep.

## In scope

**The release pipeline.** Releases are signed by the workflow that builds them
and the signature is public and permanent; `install.sh --verify` checks it.
A way to get a binary past that check is in scope. See ADR-030.

**The Homebrew cask's quarantine removal.** The published cask runs
`xattr -dr com.apple.quarantine` on the binary as it installs. Homebrew removed
`--no-quarantine` in 4.7, so there is no user-side opt-out, and bothy is not
signed with an Apple Developer ID — the reasoning is ADR-037 and the README says
plainly that the cask does this. **That it happens at all is a deliberate
choice, not a finding.** What is in scope is a way to make the hook operate on a
path it should not, or to get a different binary into the place it operates on.
Anyone who would rather no Gatekeeper check were skipped has a route: `curl`
does not attach the flag, so the install script is unaffected.

**The confinement wall.** `bothy confine` runs the agent in a container with
the project directory and the agent's own credentials mounted, and nothing else
from `$HOME`. A way for the confined agent to reach the rest of the machine is
in scope. What the wall deliberately does not stop is listed in the README and
ADR-034 — the network is not walled, and the agent's own credentials are
mounted by necessity.

**`bootstrap/install.sh`.** It runs before bothy exists, from a `curl | sh`, so
it is the highest-consequence file in the repository.

**The generated configs.** bothy writes config for other people's tools. A
template that makes one of them do something its user did not ask for is in
scope.

## Not in scope

**What the agent does.** bothy launches an agent you chose, with credentials
you gave it. Its behaviour is its own project's business. That bothy can wall
it off does not make bothy responsible for what it does inside the wall.

**The tools bothy fetches.** zellij, yazi and the rest are other projects'
releases, pinned by version and checksum in `bothy.lock`. A hole in one of them
belongs to them. A hole in how bothy *pins or verifies* them is in scope.

**Anything requiring root, or an attacker who already has your account.** bothy
never asks for root and cannot defend a machine somebody already controls.
