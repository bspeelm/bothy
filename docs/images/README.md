# Images

`workspace.png` is a screenshot of the default `cockpit` layout: Yazi across the
top with a file preview, an agent pane and a shell below, inside Zellij with
full pane frames.

`bothy-source.png` is the original artwork: a bothy in a landscape, rendered in
ASCII. It is greyscale on an opaque white ground.

`bothy-dark.png` and `bothy-light.png` are derived from it for the README. The
white ground becomes transparent — otherwise it sits as a white slab in GitHub's
dark theme — and the glyphs are flooded with one colour from the open Dracula
palette bothy ships: the light purple for dark backgrounds, the darker slate
purple for light ones. The README selects between them with `<picture>` and
`prefers-color-scheme`.

To regenerate after editing the source:

```sh
magick bothy-source.png -fuzz 12% -trim +repage /tmp/trimmed.png

for pair in "dark:#BD93F9" "light:#6272A4"; do
    mode="${pair%%:*}"; colour="${pair##*:}"
    magick \( /tmp/trimmed.png -colorspace gray -negate -level 8%,92% \) \
           -alpha copy \
           \( +clone -fill "$colour" -colorize 100 \) \
           +swap -compose copy_opacity -composite \
           "bothy-$mode.png"
done
```

The `-negate` is what turns ink into opacity: dark glyphs become opaque, the
white ground becomes transparent. The `-level` clips the anti-aliasing that
would otherwise leave a grey haze around every character.
