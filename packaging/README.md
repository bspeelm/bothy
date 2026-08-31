# Packaging

## Copr

[Copr](https://copr.fedorainfracloud.org) is Fedora's build service for
community packages. It suits bothy's audience — Fedora and Silverblue people —
and gives them `dnf install bothy` rather than piping a script into a shell.

`bothy.spec` builds from the GitHub source tag, not from the release binary. A
distro package that repackages someone else's prebuilt binary is a distro
package in name only, and rebuilding from source is what makes the `%check`
section meaningful.

Two usernames are in play and they are not the same: the Copr project lives
under the **Fedora account** name (`bspeelman`), while the source and releases
live under the **GitHub** name (`bspeelm`). Copr uses the Fedora one.

### One-time setup

```sh
# Log in at https://copr.fedorainfracloud.org/oidc_login/ first — being signed
# in to accounts.fedoraproject.org is not the same thing, and without a Copr
# session the token page shows LOGIN_TO_REVEAL instead of the values.
#
# Then copy the block from https://copr.fedorainfracloud.org/api/ into
# ~/.config/copr and chmod 600 it: your home is shared with every toolbox.
# `copr-cli new-api-token` renews an existing token; it cannot create the first
# one, because it works by updating the file that does not exist yet.

copr-cli create bothy \
    --chroot fedora-rawhide-x86_64 \
    --chroot fedora-44-x86_64 \
    --chroot fedora-43-x86_64 \
    --chroot fedora-44-aarch64 \
    --description "A turn-key terminal workspace built from tools you already trust" \
    --instructions "dnf copr enable bspeelman/bothy && dnf install bothy"
```

### Cutting a release

```sh
make release VERSION=0.2.0
```

That runs the tests, sets `Version` in the spec, adds a changelog entry,
commits, tags, and pushes. The tag is what GitHub Actions watches, so the
release archives — and with them `curl | sh` and `go install @latest` — follow
on their own.

Copr does not watch anything. Once the GitHub release is green:

```sh
make copr
```

which rebuilds the SRPM from the spec and submits it.

### Why the build is offline

Copr build roots have no network. The one dependency is vendored in the
repository, and the spec sets `GOFLAGS=-mod=vendor` and `GOPROXY=off` — so if
either were wrong the build fails there rather than silently reaching out for
something.

### What this does not change

The package installs the *binary* to `/usr/bin/bothy`. Everything bothy manages
at runtime still lives in `~/.local/share/bothy`, per user, exactly as with any
other install method. There are deliberately no `Requires:` — bothy supplies
whatever tool is missing into its own directory, checksum-verified, and never
asks a package manager for anything (PLAN.md §4). Declaring dependencies here
would be a second and contradictory opinion about how the workspace gets its
tools.

### A note for Silverblue

On an image-based host a Copr package needs `rpm-ostree install` and a reboot,
so for your own machine the install script is the better route. The Copr is for
Fedora Workstation and anyone who would rather their package manager knew about
this.
