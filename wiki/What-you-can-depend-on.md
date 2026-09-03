# What you can depend on

These are the parts bothy will not change out from under you within a major
version. Rename or remove any of them and that is a breaking change, announced
as one.

- **The keys in `config.toml`.** Existing keys are not renamed or removed. When
  one has to change, the old name keeps working and `bothy doctor` names the
  replacement — that is what `config.Retired` is for.
- **The profile and palette TOML schemas**, so a profile you wrote keeps
  rendering.
- **The two directories.** `~/.local/share/bothy/` holds what bothy installs,
  `~/.config/bothy/` holds what you set. Neither moves.
- **The `doctor --json` shape**, and the check IDs in it. `--json` exists so
  something else can read it; IDs may be added, but an existing one keeps its
  meaning. New checks are additive, so parse defensively and ignore what you do
  not recognise.

`config.toml` carries `schema = 1`, written at the top of the file. It is
bothy's bookkeeping rather than a setting — `bothy config set` refuses it — and
it exists so a newer bothy can recognise an older file deliberately rather than
by absence. A config from a *newer* bothy still loads: `doctor` warns, and the
keys this version understands still work.

What is not covered: anything printed for a human to read, the layout of
`bothy doctor`'s normal output, and the internal Go packages. Those change when
there is a reason.

---

[ADR-036](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md#adr-036--what-stable-obliges-and-why-the-config-schema-warns)
records what this obliges and why a config from a newer bothy warns rather than
refuses.
