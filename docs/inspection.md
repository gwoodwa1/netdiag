# Deterministic layout inspection

`netdiag inspect` evaluates the same placed nodes, endpoint attachments, and
routes used by the native SVG renderer:

```sh
netdiag inspect diagram.yaml
netdiag inspect --json diagram.yaml
```

The report includes a deterministic quality score, canvas dimensions, summary
counts, affected node and link IDs, and targeted suggestions.
Text output shows at most 50 findings by default. Use `--limit 0` or `--json`
to return every finding.

Current finding codes:

- `node_overlap`: two device boxes overlap.
- `link_crossing`: links without a shared endpoint cross.
- `link_through_node`: a route passes behind an unrelated device, which masks
  the route and creates an apparent unlabeled endpoint on that device.
- `crowded_endpoints`: link terminations on one device side are too close.
- `endpoint_labels_too_close`: a link's source and target interface labels do
  not have enough clearance to remain independently readable.
- `missing_interface_label`: interface labels are enabled, but a source or
  target endpoint has no resolved port or explicit label text.
- `label_clipped_by_canvas`: an interface label is clipped by the canvas bounds.
- `label_offset_from_route`: an interface label is far enough from its route to
  look detached from the line.
- `label_overlap`: interface-label boxes overlap.
- `label_link_overlap`: an unrelated link passes through an interface label.
- `label_node_overlap`: an interface label overlaps an unrelated device.

For label findings, first try endpoint-level controls such as `side`,
`position`, `stub`, `label_along`, `label_offset`, and `label_rotation`. For
large telco hub-and-spoke diagrams, larger hub/spoke card sizes and more layout
spacing are usually better than compact unreadable labels.

Use `--fail-on` in CI. Findings are printed before the command exits non-zero:

```sh
netdiag inspect --fail-on error diagram.yaml
netdiag inspect --json --fail-on warning diagram.yaml > inspection.json
```

Inspection measures the native renderer. It does not parse SVG and does not use
AI, browser rendering, fonts, or image comparison, so identical inputs produce
identical reports.

Hub-spoke diagonal and orthogonal routing treat unrelated node boxes as hard
obstacles. Both use `diagram.route_clearance`; orthogonal routes also honor
endpoint `stub` lengths and keep their first and last segments pointed out of
the selected device sides. Routing is coordinated across links, with bounded
penalties for crossings, crowded parallel runs, and interface-label boxes.
Rendering and inspection consume the same planned route geometry, including
bundled links.
Groups are visual/container structure, not general-purpose no-route zones, and
netdiag does not currently support first-class YAML blackout rectangles. Avoid
adding fake invisible nodes or empty spacer groups just to influence routing;
use route clearance, endpoint sides/positions/stubs, larger real device cards,
or Draw.io layout overrides for per-link waypoint control.
