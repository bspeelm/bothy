# Color Palette

<!--
  SYNTHETIC FIXTURE. These are not Dracula PRO's colours and are not meant to
  look like them — every value here is invented. What is reproduced faithfully
  is the *shape* of the real pack's palette.md, which is what the parser cares
  about: two top-level sections, a shared "Base" accent table, per-variant
  tables that override only some tokens, one light variant that restates
  everything, and a heading whose name does not follow the "Dracula PRO - X"
  pattern. See docs/decisions.md ADR-006 for why no real value appears here.
-->

## Base Dracula PRO

| Palette    | Hex       | RGB             | HSL             | ANSI 256 |
| ---------- | --------- | --------------- | --------------- | -------- |
| Foreground | `#010101` | `1, 1, 1`       | `0°  0% 0%`     | `231`    |
| Cyan       | `#020202` | `2, 2, 2`       | `0°  0% 1%`     | `159`    |
| Green      | `#030303` | `3, 3, 3`       | `0°  0% 1%`     | `157`    |
| Orange     | `#040404` | `4, 4, 4`       | `0°  0% 2%`     | `223`    |
| Pink       | `#050505` | `5, 5, 5`       | `0°  0% 2%`     | `218`    |
| Purple     | `#060606` | `6, 6, 6`       | `0°  0% 2%`     | `147`    |
| Red        | `#070707` | `7, 7, 7`       | `0°  0% 3%`     | `217`    |
| Yellow     | `#080808` | `8, 8, 8`       | `0°  0% 3%`     | `229`    |

## Dracula PRO

| Palette    | Hex       | RGB             | HSL            | ANSI 256 |
| ---------- | --------- | --------------- | -------------- | -------- |
| Background | `#111111` | `17, 17, 17`    | `0° 0% 7%`     | `59`     |
| Comment    | `#222222` | `34, 34, 34`    | `0° 0% 13%`    | `103`    |
| Selection  | `#333333` | `51, 51, 51`    | `0° 0% 20%`    | `60`     |

## Dracula PRO - Blade

| Palette    | Hex       | RGB             | HSL            | ANSI 256 |
| ---------- | --------- | --------------- | -------------- | -------- |
| Background | `#444444` | `68, 68, 68`    | `0° 0% 27%`    | `59`     |
| Comment    | `#555555` | `85, 85, 85`    | `0° 0% 33%`    | `109`    |
| Selection  | `#666666` | `102, 102, 102` | `0° 0% 40%`    | `66`     |

## Dracula PRO - Van Helsing

| Palette    | Hex       | RGB             | HSL            | ANSI 256 |
| ---------- | --------- | --------------- | -------------- | -------- |
| Background | `#777777` | `119, 119, 119` | `0° 0% 47%`    | `16`     |
| Comment    | `#888888` | `136, 136, 136` | `0° 0% 53%`    | `109`    |
| Selection  | `#999999` | `153, 153, 153` | `0° 0% 60%`    | `66`     |

## Alucard

| Palette    | Hex       | RGB             | HSL            | ANSI 256 |
| ---------- | --------- | --------------- | -------------- | -------- |
| Foreground | `#AAAAAA` | `170, 170, 170` | `0° 0% 67%`    | `234`    |
| Cyan       | `#BBBBBB` | `187, 187, 187` | `0° 0% 73%`    | `31`     |
| Green      | `#CCCCCC` | `204, 204, 204` | `0° 0% 80%`    | `28`     |
| Orange     | `#DDDDDD` | `221, 221, 221` | `0° 0% 87%`    | `136`    |
| Pink       | `#EEEEEE` | `238, 238, 238` | `0° 0% 93%`    | `126`    |
| Purple     | `#ABABAB` | `171, 171, 171` | `0° 0% 67%`    | `98`     |
| Red        | `#ACACAC` | `172, 172, 172` | `0° 0% 67%`    | `167`    |
| Yellow     | `#ADADAD` | `173, 173, 173` | `0° 0% 68%`    | `136`    |
| ---------- | --------- | --------------- | -------------- | -------- |
| Background | `#AEAEAE` | `174, 174, 174` | `0° 0% 68%`    | `255`    |
| Comment    | `#AFAFAF` | `175, 175, 175` | `0° 0% 69%`    | `103`    |
| Selection  | `#B0B0B0` | `176, 176, 176` | `0° 0% 69%`    | `188`    |

# Color Palette - Terminal Standard

Everything below this second top-level heading repeats the variant headings
with ANSI content, and must be ignored by the parser. If it were not, "Blade"
below would overwrite the design values parsed above.

## Dracula PRO - Base

| Palette       | Hex       | RGB             | HSL              | 256   |
| ------------- | --------- | --------------- | ---------------- | ----- |
| Black         | `#FF0001` | `255, 0, 1`     | `0° 100% 50%`    | `59`  |
| Red           | `#FF0002` | `255, 0, 2`     | `0° 100% 50%`    | `217` |

## Dracula PRO - Blade

| Palette       | Hex       | RGB             | HSL             | 256   |
| ------------- | --------- | --------------- | --------------- | ----- |
| Background    | `#FF0003` | `255, 0, 3`     | `0° 100% 50%`   | `59`  |
| Comment       | `#FF0004` | `255, 0, 4`     | `0° 100% 50%`   | `109` |
| Selection     | `#FF0005` | `255, 0, 5`     | `0° 100% 50%`   | `66`  |
