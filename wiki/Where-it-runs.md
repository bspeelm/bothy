# Where it runs

If your terminal can draw images (Ghostty, Kitty, WezTerm), bothy runs in it.
If it cannot, bothy opens a Ghostty window instead, so that previews come out
as pictures rather than as a suggestion of pictures. `--in-place` and
`--window` overrule that judgement in either direction. If you overrule it every time, make it the standing
answer with `bothy config set workspace.launch here`, or `window` for the
opposite; the flags still win for a single run. With no graphical display —
over SSH, say — it stays where it is, which is generally the sensible thing to
do when somewhere else.

That is bothy noticing you are somewhere else. `bothy connect` is the other
direction: you stay here and the workspace opens against another machine's
files, with nothing installed on it. See [Connecting](Connecting).

iTerm2 draws images too, by a protocol of its own that Zellij does not carry,
so previews there arrive as characters after all. The doctor says which of the
two is at fault rather than leaving you to adjust the wrong setting.

Inside a Toolbx or Distrobox container, bothy remembers which container it
put its tools in and goes back there when you launch it from the host. If you
install from a container that has none of the tools, bothy downloads the lot
into its own folder, and the result works from either side. It has, on the
whole, had enough of being surprised by containers, and has written some of
this down.

As for which machines: Fedora, Ubuntu, Debian and Arch in containers, and
macOS on a Mac, on every release — installed, exercised, uninstalled, and the
whole doctor report compared against what it ought to have said. That is the
whole of what supported means here: not that it ought to work, but that
something proved it did this morning.

**Windows is in beta.** There is a `bothy_windows_amd64.zip` on every release
and `scoop install bothy` brings it down. bothy installs there, fetches all
eight tools and verifies them, generates every config, keeps its own config out
of `%APPDATA%`, opens the workspace, and the session survives closing the
window and comes back with its panes where you left them. All of that was done
on a Windows 11 machine rather than reasoned about.

It is beta and not supported because supported here means the table above, and
the table is what CI proves — there is a Windows job now, and it has not been
green for long enough to mean anything yet. Four things are known rather than
guessed:

- **No image previews.** None, on any Windows terminal. Yazi sends the iTerm2
  protocol on WezTerm and the multiplexer does not carry it. The doctor says so
  and Enter still opens the file in whatever Windows opens it with.
- **No arm64.** Zellij publishes no Windows build for it, and a workspace with
  no multiplexer is not a workspace.
- **Two things must be installed first.** The Microsoft C runtime, or Yazi
  exits the moment it starts with nothing on screen to say why; and Git for
  Windows, whose `file.exe` is how Yazi works out what a file is. The doctor
  names both and what to run.
- **`confine`, `box` and `connect` are Linux and macOS features** — they want
  podman, toolbox, flatpak-spawn and FUSE. They report themselves unavailable
  rather than failing oddly.

Use WezTerm; it is what the Windows work was done against. `bothy doctor` is
the honest answer on that machine, and it is the same answer it gives anywhere
else nobody has checked for long.

Two things bothy has advice for and does not test. **Silverblue and other
image-based systems**, where `dnf` means `rpm-ostree` and a reboot: bothy is
written on one and cannot put one in a container. **Mint, Pop!_OS and the other
derivatives**, which inherit Debian's advice through `ID_LIKE` — their container
images report `ID=ubuntu`, so a job using one would prove nothing the Ubuntu job
does not. That path is unit-tested instead.

macOS took eight releases to earn that sentence, having been listed from the
first on the strength of the binaries being built, which is not the same
thing. The first Mac to run it found a file opener naming a program macOS does
not have; the first machine in CI found an uninstall that reported success and
left the binary behind. Both are fixed. Neither would have been found by
reading the code.

Anywhere else, bothy runs and the doctor tells you what it cannot do on that
machine. Nobody has checked. It does not pretend otherwise.

---

Why a platform counts as supported only when CI installs it:
[ADR-012](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-012--portability-is-a-ci-claim-not-a-readme-claim).
