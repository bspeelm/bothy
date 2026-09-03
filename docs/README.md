# Docs

Start with the [README](../README.md). These go deeper.

## If you are using bothy


## If you are wondering why it is like this

- [**decisions.md**](decisions.md) — every architectural decision, numbered,
  with the reasoning and what was given up. The longest file here and the one
  that answers most questions.
- [**north-star.md**](north-star.md) — what bothy is aiming at, so a change can
  be judged by whether it gets closer.

## If you are changing it

- [**PLAN.md**](PLAN.md) — the architecture: what each package owns and where
  the seams are.
- [**adding-a-provider.md**](adding-a-provider.md) — adding a tool bothy can
  install and configure, without writing Go.
- [**../CONTRIBUTING.md**](../CONTRIBUTING.md) — the three rules, and where to
  look first.

## history/

Finished plans and the setup bothy grew out of. Kept because they show how it
got here, not because they describe how it works now — nothing in `history/` is
maintained, and where it disagrees with the docs above, the docs above are
right.

- [plan-0.1.3](history/plan-0.1.3.md) · [plan-0.1.4](history/plan-0.1.4.md) ·
  [plan-0.1.5](history/plan-0.1.5.md) — the early milestones
- [plan-0.4.0](history/plan-0.4.0.md) — the provider format, across 0.4.0 and
  0.5.0
- [plan-1.0](history/plan-1.0.md) — the road to 1.0 as it was planned. Its five
  milestones shipped; where it and reality differ, reality won (ADR-035).
- [origin-cheatsheet](history/origin-cheatsheet.md) — the hand-built setup
  bothy automates. Every trap described in it became a `doctor` check.
