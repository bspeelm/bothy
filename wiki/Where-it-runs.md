# Where it runs

If your terminal carries the Kitty keyboard protocol (Ghostty, Kitty, WezTerm,
iTerm2), bothy runs in it. If it does not, bothy opens a Ghostty window
instead, so that Shift+Enter and Ctrl+Enter reach an agent rather than arriving
as plain Enter. `--in-place` and
`--window` overrule that decision in either direction. To make an override
permanent, set `bothy config set workspace.launch here`, or `window` for the
opposite; the flags still win for a single run. With no graphical display —
over SSH, for example — it runs where it is.

That is bothy noticing you are somewhere else. `bothy connect` is the other
direction: you stay here and the workspace opens against another machine's
files, with nothing installed on it. See [Connecting](Connecting).

No terminal gets inline image previews, on any platform: Yazi picks its
protocol from what the terminal says it is, Zellij answers that in its own
name, and what Yazi then sends is not what Zellij renders. The file pane says
`(press Enter to preview)` and Enter opens the file in your system's own
viewer. The doctor names the cause rather than leaving you to adjust the wrong
setting.

Inside a Toolbx or Distrobox container, bothy remembers which container it
put its tools in and goes back there when you launch it from the host. If you
install from a container that has none of the tools, bothy downloads them all
into its own directory, and the result works from either side.

Which machines: Fedora, Ubuntu, Debian and Arch in containers, and macOS on a
Mac, on every release — installed, exercised, uninstalled, with the whole doctor
report compared against an expected one. That is what supported means here: not
that it should work, but that a run proved it did.

**Windows is in beta.** There is a `bothy_windows_amd64.zip` on every release
and `scoop install bothy` brings it down. bothy installs there, fetches all
eight tools and verifies them, generates every config, keeps its own config out
of `%APPDATA%`, opens the workspace, and the session survives closing the
window and comes back with its panes where you left them. All of that was done
on a Windows 11 machine rather than reasoned about.

It is beta and not supported because supported here means the machines named
in the paragraph above, which CI installs and checks on every release. There is
a Windows job now, and it has not been green for long enough to mean anything
yet. Three things are known rather than guessed:

- **No arm64.** Zellij publishes no Windows build for it, and a workspace with
  no multiplexer is not a workspace.
- **Two things must be installed first.** The Microsoft C runtime, or Yazi
  exits the moment it starts with nothing on screen to say why; and Git for
  Windows, whose `file.exe` is how Yazi works out what a file is. The doctor
  names both and what to run.
- **`confine`, `box` and `connect` are Linux and macOS features** — they want
  podman, toolbox, flatpak-spawn and FUSE. They report themselves unavailable
  rather than failing oddly.

Use WezTerm, which is what the Windows work was done against. `bothy doctor` is
the authoritative answer on that machine, as it is on any machine CI does not
cover.

Two things bothy gives advice for and does not test. **Silverblue and other
image-based systems**, where `dnf` means `rpm-ostree` and a reboot: bothy is
written on one and cannot put one in a container. **Mint, Pop!_OS and the other
derivatives**, which inherit Debian's advice through `ID_LIKE` — their container
images report `ID=ubuntu`, so a job using one would prove nothing the Ubuntu job
does not. That path is unit-tested instead.

macOS was listed as supported for eight releases on the strength of the
binaries being built, which is not the same thing. The first Mac to run it found
a file opener naming a program macOS does not have; the first machine in CI
found an uninstall that reported success and left the binary behind. Both are
fixed, and neither would have been found by reading the code.

Anywhere else, bothy runs and the doctor reports what it cannot do on that
machine. Those platforms are unverified, and bothy says so rather than implying
otherwise.

---

Why a platform counts as supported only when CI installs it:
[ADR-012](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-012--portability-is-a-ci-claim-not-a-readme-claim).
