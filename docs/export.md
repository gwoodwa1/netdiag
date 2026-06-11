# SVG, PNG, and PDF export

SVG is netdiag's native deterministic output:

```sh
netdiag render diagram.yaml -o diagram.svg
```

Choose interactive HTML, PNG, or PDF by changing the output extension:

```sh
netdiag render diagram.yaml -o diagram.html
netdiag render diagram.yaml -o diagram.png
netdiag render diagram.yaml -o diagram.pdf
```

Interactive HTML embeds the native SVG and adds offline pan, zoom, inspection,
and group-collapse controls. See [interactive.md](interactive.md).

PNG and PDF conversion uses a local external SVG converter. Netdiag discovers
`rsvg-convert`, Inkscape, or ImageMagick in that order. Override discovery with
an `rsvg-convert`-compatible executable:

```sh
NETDIAG_CONVERTER=/usr/local/bin/rsvg-convert \
  netdiag render diagram.yaml -o diagram.pdf
```

The default Docker image includes `rsvg-convert` and DejaVu Sans/Mono fonts, so
SVG, PNG, and PDF exports work without host-side converter or font
installation.
