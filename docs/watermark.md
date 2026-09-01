# The watermark

An image behind the terminal, off unless you point at one:

```sh
bothy config set workspace.background_image ~/pictures/bothy-watermark.png
bothy install
```

**bothy ships no art.** The trick wants an image composited where a particular
pane will be, which depends on your screen and your layout — so a picture bothy
chose would be wrong on most machines, and the one it used to ship was a Tux it
had no business redistributing uncredited.

## Why this works at all

A terminal cannot scope a background image to one pane. But with a fixed
layout, the image can be made to *land* on one: a mostly transparent canvas the
shape of your window, with the art composited at the percentage where the pane
sits. Ghostty's `background-image-fit = stretch` maps that canvas onto the
window, so the percentages hold at any resolution.

Do not reach for `fit = none` with a corner anchor. That positions in absolute
pixels and is correct on exactly one monitor.

## Making one

Any tool that composites will do; this is ImageMagick. Say you want art in the
bottom-right, where the `cockpit` profile puts the shell pane:

```sh
# A transparent canvas the shape of your screen, art placed at the fraction
# where the pane sits. 1920x1080 here; use your own.
magick -size 1920x1080 xc:none \
  \( art.png -resize 232x324 \) -geometry +1621+626 -composite \
  ~/pictures/bothy-watermark.png
```

Those numbers put a 232×324 image inside the shell pane on a 1920×1080 screen.
Yours will differ; the arithmetic is *fraction of the window*, so measure once
and it holds when you resize.

For source art you already have, `docs/images/bothy-dark.png` in this
repository is the shelter from the README.

## Tuning it

The opacity bothy writes is deliberately low. It is not a config key, because
every other Ghostty setting is tuned the same way — put your own value in
`~/.config/bothy/overrides/ghostty/ghostty.conf` and it is appended after
bothy's, so it wins:

```
background-image-opacity = 0.12
```

## When it does not appear

`bothy doctor` fails if `workspace.background_image` names a file that is not there,
because Ghostty says nothing about a `background-image` it cannot find. It
draws nothing, which looks identical to "the opacity is too low" and sends you
tuning a setting that was never the problem.
