# Where it runs

If your terminal can draw images (Ghostty, Kitty, WezTerm), bothy runs in it.
If it cannot, bothy opens a Ghostty window instead, so that previews come out
as pictures rather than as a suggestion of pictures. `--in-place` and
`--window` overrule that judgement in either direction. If you overrule it every time, make it the standing
answer with `bothy config set workspace.launch here`, or `window` for the
opposite; the flags still win for a single run. With no graphical display —
over SSH, say — it stays where it is, which is generally the sensible thing to
do when somewhere else.

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
