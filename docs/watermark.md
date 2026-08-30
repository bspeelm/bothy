# The watermark extra

Off by default. `bothy config set workspace.watermark true` turns it on.

A terminal cannot scope a background image to one pane — but with a fixed
layout, the image can be made to *land* on one. Ghostty 1.3+ has
`background-image`, and the `cockpit` profile puts the shell pane at
bottom-right, so art composited at that fraction of the window appears to sit
behind that pane.

## Do not position in pixels

The obvious approach — a small image with `fit = none` and a corner anchor —
positions in absolute pixels, so it is correct on exactly one monitor: too small
and too low on 4K, oversized on 2K, and spilling below the pane into the status
bar.

Instead build a mostly-transparent canvas the *shape of the window*, composite
the art at the percentage where the pane sits, and stretch it. `fit = stretch`
maps the image onto the window, so the percentages hold at any resolution.
`position` is ignored under stretch.

```sh
# 1. render source art to a transparent PNG (any mono TTF works)
magick -background none -fill "#BD93F9" \
  -font /path/to/mono.ttf -pointsize 28 \
  label:@art.txt -trim +repage -resize x480 art.png

# 2. composite onto a window-shaped canvas at the pane's PERCENTAGE
#    232x324 at +1621+626 on 1920x1080 == x 84.4-96.5%, y 58-88%
magick -size 1920x1080 xc:none \
  \( art.png -resize 232x324! \) -geometry +1621+626 -composite \
  ~/.config/ghostty/watermark.png
```

Then:

```ini
background-image = ~/.config/ghostty/watermark.png
background-image-opacity = 0.05
background-image-fit = stretch
background-image-repeat = false
```

## Leaving margin at the bottom

Pane sizes are percentages and scale cleanly, but the tab bar and status bar are
**fixed line counts**. At lower resolutions they take a larger fraction of the
window and push the content panes' bottom edge up, so leave extra margin below
the art.

## Opacity

Measured against a dark background:

| Art | Opacity |
|---|---|
| Solid silhouette | `0.10` |
| Sparse line-art ASCII | `~0.28` |
| Dense ASCII over live text | `0.04`–`0.07` |

Below `0.04` it disappears. Above `0.07`, dense art garbles the text on top of
it — which is the failure mode that matters, since the whole point is that the
pane stays readable.

## Restrictions

PNG or JPEG only (no SVG — rasterise first), no animation, alpha honoured and
effectively required. Valid `fit` values: `contain cover stretch none`. If you
change the profile's pane sizes, re-measure the offsets from a screenshot.
