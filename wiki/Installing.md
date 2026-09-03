# Installing

**bothy installs into your home directory and never asks for root.** The binary
goes to `~/.local/bin`, configs to the two XDG directories, tools it fetches to
its own tree. Nothing is layered onto the host, nothing is written system-wide,
and there is no step that needs a reboot
([ADR-002](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-002--user-space-install-is-the-default)).

That is the whole reason it works unchanged on Silverblue, Kinoite and Bazzite,
and inside Toolbx and Distrobox.

## What you need first

| | |
|---|---|
| **git** | Required. bothy uses it to fetch its Yazi plugins. You have it. Everyone has it. |
| **curl** or **wget** | Only for the install script below, and only one of them. |
| **[Ghostty](https://ghostty.org)** | Recommended, not required. Yazi can only draw real image previews in a terminal that can draw images, which means Ghostty, Kitty or WezTerm. In anything else you get an approximation made of characters, which is roughly what terminals have been offering since 1978. |
| **an AI agent** | Optional, though it is rather the point. The middle pane runs `claude` unless told otherwise, and sits empty if there is nothing to run. |
| **Go** and **make** | Only if you build from source, which nobody has to. |

bothy brings everything else. If any of the above is missing, `bothy doctor`
says so, says what to type, and leaves it at that.

## The short version

```sh
curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh
```

Then `bothy` in any directory. First run lists what is missing, asks before
downloading, and opens the window.

If the command is not found afterwards, `~/.local/bin` is not on your `PATH`;
the script says so and prints the line to add.

## Every channel

| | for | |
|---|---|---|
| **script** | anyone on Linux or macOS | `curl -fsSL .../install.sh \| sh` |
| **dnf** | Fedora Workstation | `sudo dnf copr enable bspeelman/bothy && sudo dnf install bothy` |
| **apt** | Debian, Ubuntu, Mint | download the `.deb`, then `sudo apt install ./bothy_*.deb` |
| **Homebrew** | macOS, or Linux if you already use brew | `brew install --cask bspeelm/bothy/bothy` |
| **Go** | people who already have Go | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| **source** | contributors | `git clone` then `make install-binary` |

**The script is the recommended one on Linux**, and on an image-based host it is
the only one with no cost: `dnf` there means `rpm-ostree` and a reboot for
bothy itself, which is a lot to pay for a binary that runs perfectly well from
`~/.local/bin`. The `.deb` is a file rather than a repository, so `apt upgrade`
will not bring you the next one — download it when you want it
([ADR-013](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-013--the-debian-package-is-a-file-not-a-repository)).

`bothy upgrade` works out which of these you used and prints the right command.

## Checking what you got

Every release artifact is signed by the workflow that built it, in a public
log, so a swapped download is detectable whether or not anyone checks.

| installed with | checked by |
|---|---|
| dnf | dnf, against Copr's key — automatic |
| `go install` | Go, against [sum.golang.org](https://sum.golang.org) — automatic |
| script | a checksum, automatic; the signature on request |
| Homebrew | the sha256 in the cask, automatic; the signature on request |
| `.deb` | the signature on request |
| source | you compiled it yourself |

```sh
curl -fsSL .../install.sh | sh -s -- --verify          # checksum and signature
gh attestation verify ./bothy_*.deb --repo bspeelm/bothy --bundle attestation.jsonl
```

Take `attestation.jsonl` from the release page. Neither needs a GitHub account,
and neither passes quietly when it cannot check. More in [Security](Security).

## macOS: Gatekeeper

Only relevant on a Mac. bothy is not signed with an Apple Developer ID, so
macOS attaches `com.apple.quarantine` to anything a browser or Homebrew
downloads, and refuses to run it:

> **"bothy" not opened.** Apple could not verify "bothy" is free of malware.

**The cask clears the flag for you, and you should know that it does.** Homebrew
removed `--no-quarantine` in 4.7, so there is no user-side opt-out; the cask
runs `xattr -dr com.apple.quarantine` during install. That is a Gatekeeper check
skipped on your behalf. `curl` never attaches the flag, so the script avoids the
question entirely.

For a copy macOS is already refusing:

```sh
xattr -dr com.apple.quarantine "$(which bothy)"
```

`bothy doctor` recognises the flag and prints that command with your path in it.
The reasoning, including why this project will not pay Apple $99 a year, is
[ADR-037](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-037--bothy-is-not-signed-and-the-cask-clears-the-flag-itself).

## What gets installed

If you have none of them, eight tools: about 49 MB to download, roughly 124 MB
on disk once unpacked. That is a fair amount of disk for three panes, and it is
written here so that you find out from the README rather than from `du`.

| | | | |
|---|---|---|---|
| zellij | yazi | lazygit | ripgrep |
| fzf | fd | jq | zoxide |

Each is pinned to a version and a checksum in `bothy.lock`, and checked before
bothy keeps it. These are other projects' releases, so a checksum is as far as
it goes — bothy does not sign what it did not build.

If you already have good enough copies, bothy downloads nothing and tells you
which of yours it is using. What it did fetch, it keeps: when a later bothy
pins a newer version, the next `bothy install` fetches that one and leaves
your own copies exactly where they were.

It never calls your package manager, never asks for root, and never adds
anything to your `PATH`. It is aware that this is unusual, and would prefer
not to discuss it.

Two things it will not install for you. Ghostty, because it ships no
ready-made binaries and every route to it needs root. And the agent, because
your AI tools and their credentials are your business, and bothy has enough
of its own. `bothy doctor` gives you the right command for both and then goes
quiet.

## Removing it

`bothy uninstall` removes bothy's tree and the binary. `--dry-run` shows what
would go. Three things are left, and it names each on the way out:

| left behind | why | remove it with |
|---|---|---|
| `~/.config/bothy` | your settings, not bothy's | `rm -r ~/.config/bothy` |
| the container image, if you confined the agent | ~550 MB bothy did not build | `podman rmi bothy-agent` |
| the desktop entry, if you added one | outside the tree by necessity | `bothy desktop-entry --remove` |

Nothing else needs undoing: bothy never touched your dotfiles, never called
your package manager, and never added anything to your `PATH`.
