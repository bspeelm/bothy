# Packaging

Two packages, made two different ways. Fedora's rpm is built from source by
Copr out of `bothy.spec`. Debian's `.deb` is built by goreleaser at release
time from the binary it has just produced, and attached to the GitHub release
as a file. Only one of them is a repository.

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
make release VERSION=0.2.0   # tests, bumps the spec, opens the PR
# merge the PR, then:
git switch main && git pull
make release-tag             # tags the merged bump
```

Two steps because main requires a pull request. That is not only a rule to
satisfy: Copr reads `Version:` from the spec at the tagged commit, so tagging
a commit that still carries the old version would publish an rpm under the
wrong number. The bump has to be on main before the tag exists.

The tag is the trigger for everything downstream — GitHub Actions builds the
release archives, and the Copr webhook builds the rpms. Branch rules do not
cover tags, so `release-tag` needs no PR of its own.

`make release` needs `gh` installed and logged in with the `repo` scope; a
fine-grained token without *Pull requests: write* fails at the last step,
after the bump commit has already been made.

`release-tag` refuses if main is behind origin, if the spec version is already
tagged (which means the PR was never merged), or if `.copr/Makefile` is
missing from the checkout.

### Publishing to Copr by hand

```sh
make copr
```

Only needed if the webhook misses one. It hands Copr a tag and nothing else:
Copr clones the repo at that tag and runs `.copr/Makefile`, so what gets
published can only ever be the tag — there is no locally built SRPM that could
drift from it. Tags cut before `.copr/Makefile` existed (v0.1.2 and earlier)
cannot be built this way, and `make copr` says so rather than letting Copr
discover it five minutes into a build.

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

1. Open <https://copr.fedorainfracloud.org/coprs/bspeelman/bothy/integrations/>
   while logged in. It shows the GitHub webhook URL with the secret already
   in it. Copy it and append `bothy/`.

   The secret is not readable from `copr-cli` — `new-webhook-secret`
   *regenerates* it rather than printing the current one, so running that
   invalidates any hook already using it.
2. GitHub → repo → Settings → Webhooks → Add webhook.
   - Payload URL: the copied URL, ending `/<secret>/bothy/`
   - Content type: `application/json`
   - Events: *Let me select individual events* → **Branch or tag creation**
     only. Uncheck Push.

The trailing `/bothy/` matters. Without it Copr matches the tag name against
the package name and expects `bothy-0.2.0`; with it, any tag matches, which is
what lets `v0.2.0` work. Branch creations arrive on the same subscription but
carry no commits, so they never trigger a build.

Copr builds the pushed tag, not the package's stored committish — the stored
value is only a filter for webhook builds and a default for manual ones.

Both defaults in that form are wrong, and both fail quietly-ish:

| Symptom | Cause |
|---|---|
| No delivery listed in GitHub at all | Subscribed to Push. Copr keys tag handling off a field only "Branch or tag creation" sends. |
| Delivery returns **415** | Content type left as `x-www-form-urlencoded`. Copr parses JSON only. |
| Delivery returns **404** | Secret truncated, or the URL is missing the `/bothy/` suffix or its trailing slash. |
| Delivery returns 200, no build | Package is not `scm`, `webhook_rebuild` is off, or the stored `clone_url` does not match. |

GitHub's *Recent Deliveries* tab on the hook shows the response code, which
is the fastest way to tell these apart. There is no Redeliver button on a
failed delivery, so retry by pushing a throwaway tag and deleting it after.

Verified working: a tag push produced a Copr build nine seconds later.

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

## Debian and Ubuntu

`.goreleaser.yaml` has an `nfpms:` block, so the tag that builds the release
archives also builds `bothy_<version>_amd64.deb` and the arm64 one, attaches
both to the GitHub release, and includes them in `checksums.txt` for the same
reason the tarballs are. There is nothing extra to run: `make release-tag`
pushes the tag and Actions does the rest.

Install one with:

    sudo apt install ./bothy_0.1.4_amd64.deb

The leading `./` is not optional. Without it apt looks for a *package* by that
name in your configured sources, fails to find one, and says so in a way that
reads like the file is broken.

### It is not a repository

There is no apt source to add, no file in `sources.list.d`, and no signing key.
That is the point, and it has one consequence a user will meet: **`apt upgrade`
will never bring you a new bothy.** dpkg knows the package is installed and what
version it is, so `apt list --installed` finds it and `sudo apt remove bothy`
removes it, but nothing on the machine knows where a newer one would come from.
Upgrading means going back to the releases page for the next `.deb` and running
`apt install ./` on it again, which upgrades in place rather than objecting that
the package is already there.

Anyone who would rather that happened by itself is better served by the install
script, which resolves `/releases/latest/` every time it runs. ADR-013 records
why there is no PPA and no hosted repository.

### It declares no dependencies

Like the rpm, and for reasons of its own rather than by inheritance. bothy needs
`git` at runtime to fetch its Yazi plugins, and `infocmp` for one doctor check;
neither is declared. A missing one is not a broken install — `bothy doctor`
names it and gives you the command, which is the thing bothy is for. Declaring
them would trade that diagnostic for an apt error, and would pull perl onto the
machine of someone who wanted one static binary in `/usr/bin`. Declaring them on
one distribution and not the other would be worse: the answer to "what does this
package require" would depend on which package you asked.

`Recommends:` is the Debian-shaped middle ground and was considered. It is not
taken because rpm has weak dependencies too, so doing it honestly means doing it
in the spec as well — a decision about both packages, not about apt.

The useful consequence is that `sudo dpkg -i bothy_*.deb` works on its own.
There is nothing for apt to resolve, so the usual `apt -f install` follow-up is
never needed.

### Building one without cutting a release

The `.deb` only exists once a release is published, which makes it awkward to
check the way everything else here is checked. goreleaser will build the whole
artifact set locally and publish none of it:

    goreleaser release --snapshot --clean --skip=publish

That writes `dist/bothy_<next>~next_amd64.deb` through exactly the config path
CI uses. On a machine without `dpkg-deb` — a Fedora one, for instance — a `.deb`
is an `ar` archive and reads fine without it:

    ar x bothy_*.deb && tar -xzf control.tar.gz -O ./control

That answers the questions that actually go wrong on a first package:
architecture (`amd64`, not `linux-amd64`), a version dpkg will accept, a
non-empty `Maintainer:`, and whether the binary landed in `/usr/bin`.

Verified working: the snapshot deb carries `Architecture: amd64`, no `Depends:`
field at all, `/usr/bin/bothy` as a statically linked ELF that runs, and the
three doc files under `/usr/share/doc/bothy/`.
