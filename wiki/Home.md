# bothy

A turn-key terminal workspace built from tools you already trust.
The front door is the [README](https://github.com/bspeelm/bothy) — what bothy
is, the six ways to install it, and what it touches.

These pages are the detail that would crowd a front page: things you need once,
in depth, after deciding rather than while deciding.

- **[Installing and verifying](Installing-and-verifying)** — what macOS does to
  an unsigned binary and how to clear it, which install channel checks what, and
  the two commands that verify a download came from this repository.
- **[Walling off the agent](Walling-off-the-agent)** — setting up `bothy
  confine`: the three commands, the toolbox case where podman lives on the host,
  configuration, and removing it.

Why bothy is shaped the way it is lives in
[`docs/decisions.md`](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md)
— every decision numbered, with what was given up and what was refused.

---

*These pages are generated from `wiki/` in the main repository. Edits made here
are overwritten; a wrong answer is a pull request.*
