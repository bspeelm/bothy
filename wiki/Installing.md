# Installing

**bothy installs into your home directory and never asks for root.** The binary
goes to `~/.local/bin`, configs to the two XDG directories, tools it fetches to
its own tree. Nothing is layered onto the host, nothing is written system-wide,
and there is no step that needs a reboot
([ADR-002](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-002--user-space-install-is-the-default)).

That is why it works unchanged on Silverblue, Kinoite and Bazzite, and inside
Toolbx and Distrobox.

## What you need first

| | |
|---|---|
| **git** | Required: bothy uses it to fetch its Yazi plugins. On Windows, install [Git for Windows](https://git-scm.com/download/win) — Yazi also uses the `file.exe` it ships to determine a file's type. |
| **curl** or **wget** | Only for the install script below, and only one of them. |
| **[Ghostty](https://ghostty.org)** | Recommended, not required. Ghostty, Kitty, WezTerm and iTerm2 carry the Kitty keyboard protocol, which is what lets Shift+Enter and Ctrl+Enter reach an agent instead of arriving as plain Enter. In anything else bothy opens a Ghostty window to get them. |
| **an AI agent** | Optional, and what the middle pane is for. It runs `claude` unless configured otherwise, and is empty if there is nothing to run. |
| **the Microsoft C runtime** | Windows only, and required: Yazi's build links it, and without it Yazi exits the moment it starts with nothing on screen to say why. `winget install Microsoft.VCRedist.2015+.x64`. |
| **Go** and **make** | Only to build from source. |

bothy brings everything else. If any of the above is missing, `bothy doctor`
reports it and gives the command to install it.

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
| **scoop** | Windows | `scoop bucket add bothy https://github.com/bspeelm/scoop-bothy && scoop install bothy` |
| **Go** | people who already have Go | `go install github.com/bspeelm/bothy/cmd/bothy@latest` |
| **source** | contributors | `git clone` then `make install-binary` |

**Prefer your package manager** — dnf on Fedora, the `.deb` on Debian and
Ubuntu, Homebrew on macOS. They are signed, they upgrade the way the rest of the
machine upgrades, and the script's unsigned first step does not apply to them.

**The script is the better choice on an image-based host**, where `dnf` means
`rpm-ostree` and a reboot for a binary that runs from `~/.local/bin`. The `.deb`
is a file rather than a repository, so `apt upgrade` will not bring the next
release; download it when you want it
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
| scoop | the sha256 in the manifest, automatic; the signature on request |
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

**The cask clears the flag during installation**, and you should know that it
does: Homebrew removed `--no-quarantine` in 4.7, so there is no user-side
opt-out, and the cask runs `xattr -dr com.apple.quarantine` itself. That is a
Gatekeeper check skipped on your behalf. `curl` never attaches the flag, so the
script avoids the question.

For a copy macOS is already refusing:

```sh
xattr -dr com.apple.quarantine "$(which bothy)"
```

`bothy doctor` recognises the flag and prints that command with your path in it.
The reasoning, including why the project does not hold an Apple Developer ID,
is
[ADR-037](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-037--bothy-is-not-signed-and-the-cask-clears-the-flag-itself).

## What gets installed

With none of them already present, bothy fetches eight tools: about 49 MB to
download and roughly 124 MB on disk once unpacked. The figure is stated here so
that it is known before the install rather than discovered afterwards.

| | | | |
|---|---|---|---|
| zellij | yazi | lazygit | ripgrep |
| fzf | fd | jq | zoxide |

Each is pinned to a version and a checksum in `bothy.lock`, and verified before
bothy keeps it. These are other projects' releases, so a checksum is as far as
verification goes: bothy does not sign what it did not build.

If you already have good enough copies, bothy downloads nothing and tells you
which of yours it is using. What it did fetch, it keeps: when a later bothy
pins a newer version, the next `bothy install` fetches that one and leaves
your own copies exactly where they were.

It never calls your package manager, never asks for root, and never adds
anything to your `PATH`.

### If a download fails partway

Nothing is rolled back, and nothing is left broken. The tools that installed
stay where they are and are recorded, so running `bothy install` again skips
them and retries only what failed.

The tool that failed leaves nothing at all. bothy downloads the file, checks it
against the pin, and unpacks it, all in memory — nothing is written until every
one of those passes. A bad checksum or a missing release means there is no
half-written file to clean up.

So a failed run leaves a partial install rather than a broken one, and running
it again completes the job.

Two things bothy does not install. Ghostty, because it ships no ready-made
binaries and every route to it requires root; and the agent, because AI tools
and their credentials are yours to manage. `bothy doctor` gives the command for
both.

## Tab completion

`bothy` completes its commands, subcommands and flags in bash and zsh.

**From dnf, apt, pacman or the AUR you already have it.** The package installs
the two files the way it installs the licence and the docs — the package
manager writing to its own prefix, which is its job. Open a new shell and press
Tab.

| shell | where the package puts it |
|---|---|
| bash | `/usr/share/bash-completion/completions/bothy` |
| zsh | `/usr/share/zsh/site-functions/_bothy` |

**From the install script, Homebrew or `go install`, ask for it once.** No
package manager ran, so nothing placed it:

```sh
bothy completion bash --install
bothy completion zsh --install
```

Without `--install` it prints the script to standard output.

This writes outside bothy's own directory, because the shell has to find it
there — the same exception the desktop entry makes. `bothy uninstall` names the
file rather than removing it, and `bothy completion bash --remove` undoes it.

bash searches that directory on its own. zsh has no per-user default, so it
needs one line in your `~/.zshrc`, before `compinit` — the command prints it
with the path filled in:

```sh
fpath=(~/.local/share/zsh/site-functions $fpath)
```

Completion works inside a bothy session too, with nothing extra: the shell pane
is an ordinary interactive shell, and bothy does not redirect the directories
completions live in.

**It offers what bothy accepts, not what you have**: commands, subcommands and
flags, but not toolbox names, hosts or running sessions, which would mean
running bothy on every keypress. Both files are held against the code in both
directions by the test suite, so renaming a command breaks the build rather
than the completion.

## Removing it

`bothy uninstall` removes bothy's tree and the binary; `--dry-run` shows what
would go. What it leaves, it names as it goes:

| left behind | why | remove it with |
|---|---|---|
| `~/.config/bothy` | your settings, not bothy's | `rm -r ~/.config/bothy` |
| the container image, if you confined the agent | ~550 MB bothy did not build | `podman rmi bothy-agent` |
| the desktop entry, if you added one | outside the tree by necessity | `bothy desktop-entry --remove` |

Nothing else needs undoing: bothy never touched your dotfiles, never called
your package manager, and never added anything to your `PATH`.

This works the same on a half-finished install as on a complete one: uninstall
removes the whole directory rather than working through a list, so how far the
install got does not matter.

The binary goes too, but only the copy the install script placed in
`~/.local/bin`. One installed by dnf, apt or Homebrew belongs to them, so bothy
leaves it and says so.
