# SVG, HTML, PNG, PDF, and draw.io export

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

Create an editable draw.io document by selecting the draw.io renderer:

```sh
netdiag render diagram.yaml --renderer drawio -o diagram.drawio
```

The draw.io serializer is dependency-free and maps the renderer-neutral
diagram model to editable groups, role-based device shapes, interface labels,
and orthogonal links. It is an export target for manual refinement; netdiag
remains the source of truth.

To preserve supported manual layout decisions separately from topology data,
give important links explicit IDs and render with a layout override file:

```yaml
# diagram.yaml
links:
  - id: core-a__core-b
    from: core-a:HundredGigE0/0/0/1
    to: core-b:HundredGigE0/0/0/1
```

```yaml
# diagram.layout.yaml
version: 1
layout_overrides:
  nodes:
    core-a:
      x: 420
      y: 180
      locked: true
  links:
    core-a__core-b:
      source_side: right
      target_side: left
      waypoints:
        - {x: 540, y: 160}
        - {x: 600, y: 160}
      locked: true
```

```sh
netdiag render diagram.yaml --renderer drawio \
  --layout-overrides diagram.layout.yaml -o diagram.drawio
```

Draw.io cells include stable `netdiag-id` and `netdiag-kind` metadata for
nodes, groups, links, and endpoint labels. Links without an explicit ID receive
a deterministic ID based on their normalized endpoints, so reordering links
does not change their identity.

Layout overrides currently support node and group bounds, locks, and style
metadata, plus link endpoint sides, waypoints, locks, and the `orthogonal`,
`straight`, or `curved` style presets. Node and nested-group coordinates are
relative to their Draw.io parent. Importing edits from an existing `.drawio`
file is not yet supported; arbitrary Draw.io-only edits may not survive a fresh
render.

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
