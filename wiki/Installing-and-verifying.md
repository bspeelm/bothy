# Installing, and checking what you got

The six ways in are in the
[README](https://github.com/bspeelm/bothy#getting-bothy). This page is the two
things that need more room than a front page should give them: what macOS does
to an unsigned binary, and how to check a download came from this repository.

## macOS and Gatekeeper

bothy is not signed with an Apple Developer ID. macOS attaches
`com.apple.quarantine` to anything a browser or Homebrew downloads, and
Gatekeeper refuses an unsigned file carrying it:

> **"bothy" not opened.** Apple could not verify "bothy" is free of malware
> that may harm your Mac or compromise your privacy.

That is not a claim anything is wrong with the binary. Gatekeeper asks whether
Apple can identify a paying developer account behind it; the answer is no.
bothy's releases are signed by the workflow that built them and recorded in a
public log, which says where the bytes came from — a stronger statement, and a
different question from the one being asked
([ADR-037](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-037--bothy-is-not-signed-and-the-cask-clears-the-flag-itself)).

**The cask clears the flag for you, and you should know that it does.**
Homebrew removed `--no-quarantine` in 4.7, so there is no user-side opt-out;
the cask runs `xattr -dr com.apple.quarantine` on the binary as it installs.
That is a Gatekeeper check skipped on your behalf. If you would rather it were
not, `curl` does not attach the flag, so the install script is unaffected.

For a copy macOS is already refusing — installed before this landed, or
downloaded from the releases page in a browser:

```sh
xattr -dr com.apple.quarantine "$(which bothy)"
```

`bothy doctor` recognises the flag and prints that command with the right path
filled in.

## Checking what you got

Every release artifact is signed by the workflow that built it, in a public
log — so a swapped download is detectable whether or not anyone checks.

| installed with | checked by |
|---|---|
| Homebrew | the sha256 in the cask, automatic; the signature on request |
| dnf | dnf, against Copr's key — automatic |
| `go install` | Go, against [sum.golang.org](https://sum.golang.org) — automatic |
| script | a checksum, automatic; the signature on request |
| `.deb` | the signature on request |
| source | you compiled it yourself |

The checksum catches a corrupted download. The signature also says the bytes
came out of this repository's workflow; checking it needs the
[`gh` CLI](https://cli.github.com), so it is opt-in. Without it you are
trusting HTTPS and GitHub, as you do to clone the repository.

```sh
curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh -s -- --verify
gh attestation verify ./bothy_*.deb --repo bspeelm/bothy --bundle attestation.jsonl
```

Take `attestation.jsonl` from the release page. Neither command needs a GitHub
account, and neither passes quietly when it cannot check.

## Two channels with edges

**Silverblue and other image-based systems.** `dnf` means `rpm-ostree install`
and a reboot, which is the price of an operating system that does not change
under you. The script needs neither.

**The `.deb` is a file, not a repository**, so `apt upgrade` will not bring you
the next one; download it when you want it. Running a Debian archive is a
commitment, and this one has not made it
([ADR-013](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-013--the-debian-package-is-a-file-not-a-repository)).
