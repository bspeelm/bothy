# AUR

`PKGBUILD` builds bothy from the tagged source tarball, the same way
`.goreleaser.yaml` does — no cgo, symbols stripped, the same version string.

It is **not** published from here. The AUR is a separate git remote, and
pushing to it needs an account with an SSH key registered and the `bothy` name
claimed. Until then this file is maintained and tested but unpublished.

## Testing it without an AUR account

Everything except the push can be checked, in an Arch container:

```sh
podman run --rm -v "$PWD/packaging/aur:/pkg:z" archlinux:latest sh -c '
  pacman -Sy --noconfirm --needed base-devel go git namcap
  useradd -m b && cp /pkg/PKGBUILD /home/b/ && chown -R b /home/b
  namcap /home/b/PKGBUILD
  su b -c "cd ~ && makepkg -f --nosign"
  pacman -U --noconfirm /home/b/bothy-*.pkg.tar.zst
  bothy --version'
```

`makepkg` runs `check()`, which runs the test suite, so a build that passes has
also run the tests on Arch.

## Two versions to keep in step

`pkgver` and `sha256sums` both change with every release, and neither is
derived from `packaging/bothy.spec` yet. A channel whose version drifts is
worse than no channel, so either the release step updates both from the spec,
or the README says this channel may lag. That decision is #60's.
