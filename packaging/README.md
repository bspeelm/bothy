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
make release VERSION=0.2.0   # tests, bumps the spec, commits, tags, pushes
make copr                    # once the GitHub release is green
```

The tag is what GitHub Actions watches, so the release archives — and with
them `curl | sh` and `go install @latest` — follow on their own.

`make copr` hands Copr a tag and nothing else. Copr clones the repo at that
tag and runs `.copr/Makefile`, so what gets published can only ever be the
tag; there is no locally built SRPM that could drift from it. Tags cut before
`.copr/Makefile` existed (v0.1.2 and earlier) cannot be built this way, and
`make copr` says so rather than letting Copr discover it.

### Making Copr build on its own

Copr can watch the repo, which removes `make copr` entirely. It is not set up,
and the setup is fiddly in a way worth writing down.

Copr's webhook does **not** decide what to do from the GitHub event name. It
reads a `ref_type` field that appears only in GitHub's *"Branch or tag
creation"* event and never in *Push*. A tag arriving via Push is therefore
treated as an ordinary commit push and filtered against the package's
committish with `ref.endswith(committish)` — `"refs/tags/v0.2.0".endswith("main")`
is false, so nothing builds, silently. Subscribing to Push is the intuitive
choice and it is the wrong one.

To set it up:

1. Copr → project → Settings → Integrations, and copy the webhook secret.
2. GitHub → repo → Settings → Webhooks → Add webhook.
   - Payload URL: `https://copr.fedorainfracloud.org/webhooks/github/255721/<SECRET>/bothy/`
   - Content type: `application/json`
   - Events: *Let me select individual events* → **Branch or tag creation**
     only. Uncheck Push.

The trailing `/bothy/` matters. Without it Copr matches the tag name against
the package name and expects `bothy-0.2.0`; with it, any tag matches, which is
what lets `v0.2.0` work. Branch creations arrive on the same subscription but
carry no commits, so they never trigger a build.

Copr builds the pushed tag, not the package's stored committish — the stored
value is only a filter for webhook builds and a default for manual ones.

This is read from Copr's source rather than tested here, because the webhook
needs repository settings access. Verify it on the first tag: if nothing shows
up in Copr within a minute, `make copr` still works.

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
