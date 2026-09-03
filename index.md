---
title: bothy
---

A turn-key terminal workspace built from tools you already trust. One command
opens a file browser, an agent and a shell in one window, configured and
verified, using the tools you already have.

<figure>
  <a href="workspace.png">
    <img src="workspace.png" width="3842" height="2090"
         alt="The cockpit layout: a file browser across the top with a preview pane, an agent pane and a shell below.">
  </a>
  <figcaption>The default <code>cockpit</code> layout, inside Zellij. Click for full size.</figcaption>
</figure>

## Get it

One command, no root. It installs into your home directory, so it works
unchanged on immutable distros and inside Toolbx.

```sh
curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh
```

Then `bothy` in any project directory. There are six ways in — dnf, apt,
Homebrew, Go, source — and [the install page][install] covers each and what
verifies it.

## Read about it

- [**The documentation**][wiki] — the words bothy uses, a first session, every
  command, the doctor, and what it does and does not secure.
- [**The code**][repo] — one Go binary, one dependency, and the decisions that
  produced it.
- [**How this was built**](how-it-was-built.html) — the longer account.

[install]: https://github.com/bspeelm/bothy/wiki/Installing
[wiki]: https://github.com/bspeelm/bothy/wiki
[repo]: https://github.com/bspeelm/bothy
