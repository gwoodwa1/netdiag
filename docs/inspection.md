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
- `label_overlap`: interface-label boxes overlap.
- `label_link_overlap`: an unrelated link passes through an interface label.
- `label_node_overlap`: an interface label overlaps an unrelated device.
- `label_outside_canvas`: an interface label extends beyond the canvas.

Use `--fail-on` in CI. Findings are printed before the command exits non-zero:

```sh
netdiag inspect --fail-on error diagram.yaml
netdiag inspect --json --fail-on warning diagram.yaml > inspection.json
```

Inspection measures the native renderer. It does not parse SVG and does not use
AI, browser rendering, fonts, or image comparison, so identical inputs produce
identical reports.
