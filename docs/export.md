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

After manually moving supported netdiag-managed objects in diagrams.net,
extract their current presentation metadata and render it again:

```sh
netdiag extract-overrides edited.drawio --source diagram.yaml \
  -o diagram.layout.yaml --report

netdiag render diagram.yaml --renderer drawio \
  --layout-overrides diagram.layout.yaml -o diagram.drawio
```

`--report` prints managed object counts, ignored Draw.io object counts, and
warnings for managed objects that exist on only one side of the source/Draw.io
pair. Report mode ignores stale managed objects so that valid remaining layout
state can still be extracted. Without `--report`, stale managed IDs remain a
hard error.

Draw.io cells include stable `netdiag-id` and `netdiag-kind` metadata for
nodes, groups, links, and endpoint labels. Links without an explicit ID receive
a deterministic ID based on their normalized endpoints, so reordering links
does not change their identity.

Layout overrides currently support node and group bounds and locks, plus link
endpoint sides, waypoints, locks, and the `orthogonal`, `straight`, or `curved`
style presets. Node and nested-group coordinates are relative to their Draw.io
parent. `extract-overrides` reads only netdiag-managed nodes, groups, and links
carrying stable metadata. Arbitrary Draw.io shapes, annotations, text changes,
and decoration are deliberately ignored and may not survive a fresh render.

### Safe edits in Draw.io

The durable model boundary is:

- topology YAML is the network truth
- Draw.io is the editable visual artefact
- layout override YAML is durable human layout intent
- SVG, HTML, PNG, and PDF are publication outputs

Edits currently preserved by `extract-overrides`:

- moving and resizing netdiag-managed nodes and groups
- adjusting link waypoints and source/target attachment sides
- selecting straight, curved, or orthogonal link routing
- locking managed nodes, groups, and links

Edits intentionally ignored or not currently preserved:

- changing generated labels or moving labels directly in Draw.io
- renaming nodes or changing topology in Draw.io
- adding links manually without netdiag metadata
- deleting or changing `netdiag-id` / `netdiag-kind` metadata
- adding annotations, decorative shapes, or other unmanaged objects

Keep the topology YAML and extracted layout override YAML in version control.
The Draw.io file can be regenerated for another editing pass without making its
XML the primary model.

When topology grows, an existing layout override file can be applied directly
to the updated source. Existing managed nodes, groups, and links retain their
saved layout state. A new node is placed near the first already positioned
adjacent node in the same Draw.io parent, with deterministic collision
avoidance; otherwise it receives deterministic generated placement. New links
receive generated routing. If topology removes or renames managed objects, use
`extract-overrides --report` against the updated source to identify and omit
stale layout state before returning to strict automation.

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
