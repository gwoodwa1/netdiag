# Netdiag Skill Loop

Use this workflow when an LLM creates or repairs a network diagram.

1. Generate concise version-1 YAML using structured endpoints when attachment
   hints matter.
2. Run `netdiag validate --json diagram.yaml`.
3. If `valid` is false, repair only the reported schema or semantic errors.
4. Run `netdiag fmt -w diagram.yaml` to normalize the document.
5. Run `netdiag recommend diagram.yaml`, then inspect
   `netdiag plan --json diagram.yaml`.
6. Render with an audit report:
   `netdiag render diagram.yaml --report render-report.json -o diagram.svg`.
7. Inspect the native layout:
   `netdiag inspect --json diagram.yaml`.
8. Run bounded deterministic repair:
   `netdiag improve-layout diagram.yaml -o diagram-improved.yaml`.
9. Inspect the improved file and repair any remaining geometry issues using
   endpoint sides, positions, stubs, rotations, and layout clearance values.
10. If required, compare ELK explicitly:
   `netdiag render diagram.yaml --renderer d2 --layout elk -o diagram.d2.svg`.

## Dense telco hub-and-spoke diagrams

For large PE/P or core/edge diagrams, do not try to solve interface-label
collisions by moving generated labels in Draw.io. Netdiag does not currently
preserve manually moved generated labels during `extract-overrides`.

Prefer schema-owned spacing controls:

- keep `diagram.layout: hub-spoke` for PE/P topologies when the source data has
  a clear hub/core; this layout intentionally uses very large hub cards and
  large spoke cards by default
- further enlarge unusual high-degree hub/core or spoke nodes with explicit
  `width` and `height`
- increase `diagram.endpoint_clearance` when labels crowd node edges
- increase `diagram.route_clearance` when links run too close together
- use endpoint `side`, `position`, `stub`, and `label_rotation` on the busiest
  links
- use endpoint `label_along` and `label_offset` when an interface label needs a
  durable route-relative nudge
- put `label_rotation`, `label_along`, and `label_offset` inside the link
  endpoint block (`from:` or `to:`), never at the top level of a link item
- every link endpoint must include both `node` and `port`; the YAML key is
  `port`, not `interface`
- never remove endpoint ports while adjusting labels; label controls are
  additional endpoint fields, not replacements for `port`
- for dense hub/spoke links, prefer a straight endpoint lead-out followed by a
  diagonal transit lane: set endpoint `side`, `position`, and `stub` instead of
  forcing one long diagonal directly from the card edge
- when a second PE/spoke layer sits behind the first, keep the PE cards spaced
  far enough apart for transit lanes to route through the gaps; increase real
  node `width`/`height`, group spacing, and `route_clearance` before adding
  many manual label offsets

Valid link endpoint shapes:

```yaml
links:
  - from: core-hub-01:HundredGigE0/0/0/0
    to: spoke-pe-17:HundredGigE0/0/0/0
```

```yaml
links:
  - from:
      node: core-hub-01
      port: HundredGigE0/0/0/0
      side: bottom
      position: 0.35
      stub: 120
      label_along: 0.22
      label_offset: 34
      label_rotation: 90
    to:
      node: spoke-pe-17
      port: HundredGigE0/0/0/0
```

Invalid endpoint shapes:

```yaml
links:
  - from: {node: core-hub-01}          # missing port
    to: {node: spoke-pe-17}            # missing port
  - from:
      node: core-hub-01
      interface: HundredGigE0/0/0/0    # wrong key; use port
    to:
      node: spoke-pe-17
      port: HundredGigE0/0/0/0
    label_rotation: 90                 # wrong level; put under from/to
```

Example extra card sizing for dense hub-and-spoke diagrams:

```yaml
diagram:
  layout: hub-spoke
  endpoint_clearance: 36
  route_clearance: 28
nodes:
  core-hub-01:
    label: Core Hub 01
    role: core-router
    width: 900
    height: 260
  spoke-pe-17:
    label: Spoke PE 17
    role: edge-router
    width: 480
    height: 170
```

Native interface labels already render as filled badge rectangles with a
stroke. Use `diagram.interface_label_style` for badge fill, text color, border,
radius, and padding rather than asking the editor to redraw labels manually.
Avoid automatic `label_rotation: 180` as a default; use side, position, stubs,
and larger hub cards first, and reserve 180-degree labels for explicit user
intent.

Example durable label nudge:

```yaml
from:
  node: core-hub-01
  port: HundredGigE0/0/0/0
  label_along: 0.22
  label_offset: 34
  label_rotation: 90
```

## Routing around busy areas

Native hub-and-spoke routing treats unrelated **device node boxes** as hard
obstacles. It does not currently have first-class YAML blackout rectangles, and
groups are visual/container structure rather than general routing-obstacle
objects.

Do not create fake invisible nodes or empty "spacer" groups just to force
routing. That pollutes the topology model and can confuse validation,
inspection, discovery diffs, and future regeneration.

Preferred current controls:

- increase `diagram.route_clearance` to separate busy link bundles
- increase `diagram.endpoint_clearance` to spread ports on crowded node sides
- enlarge real hub/spoke device cards with node `width` and `height`
- pin endpoint `side` and `position` on specific busy links
- add endpoint `stub` so a link leaves the card cleanly before turning
- use `label_along` and `label_offset` for durable route-relative label nudges
- use Draw.io layout overrides for per-link waypoints when native YAML controls
  are not enough

If the user needs true no-route zones, describe that as a future feature:
explicit YAML obstacle/blackout rectangles that the native router avoids while
remaining separate from network topology.

Use Draw.io for durable polish only after the YAML can render legibly:

1. Render Draw.io:
   `netdiag render diagram.yaml --renderer drawio -o diagram.drawio`.
2. Move or resize nodes/groups and adjust connector waypoints.
3. Do not rely on moved generated labels being preserved.
4. Check round-trip safety:
   `netdiag doctor drawio diagram.drawio`.
5. Extract durable layout intent:
   `netdiag extract-overrides diagram.drawio --source diagram.yaml -o diagram.layout.yaml --report`.
6. Re-render with overrides:
   `netdiag render diagram.yaml --renderer drawio --layout-overrides diagram.layout.yaml -o diagram.drawio`.

Preserved Draw.io edits include node/group geometry, link waypoints,
source/target attachment sides, routing style, and lock state. Unsupported
manual annotations or generated-label movements should be treated as final
publication edits, not durable topology layout intent.

Do not assume D2 endpoint-side hints are authoritative. The D2 spike proves
nested groups, routed links, three link-label positions, and parallel links.
Precise port-side placement and collision-free network labels remain owned by
the netdiag schema and finishing layer.

Fixtures:

- `examples/skills/d2-elk-hard-cases.yaml`
- `examples/skills/invalid-repair-loop.yaml`
- `examples/regression/`
