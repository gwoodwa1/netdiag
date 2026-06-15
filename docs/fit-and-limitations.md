# Fit and limitations

Netdiag is a topology-aware network diagram lifecycle tool. It is designed for
repeatable generation, review, human refinement, and regeneration of static or
slowly changing network topology diagrams.

## Good fit

Netdiag is a strong fit when you need:

- network topology stored as reviewable YAML
- deterministic SVG output for documentation and CI
- editable Draw.io output without making Draw.io the topology source of truth
- durable human layout intent that survives topology evolution
- diagrams generated from LLDP, IS-IS, includes, or reusable templates
- inspection and bounded deterministic repair for layout problems

## Deliberate boundaries

These are product-scope decisions, not missing general-diagramming features:

- Netdiag is not a replacement for a general-purpose canvas such as Draw.io.
  Arbitrary shapes, freehand annotations, and unmanaged connectors are not part
  of the durable topology model.
- Netdiag targets static or slowly evolving topology documentation, not live
  monitoring dashboards, telemetry visualization, or alerting.
- Topology YAML remains network truth. Draw.io owns final polish, while layout
  override YAML preserves only supported layout intent.
- Auto-layout is a starting point. Netdiag does not try to make auto-layout
  perfect; it makes manual layout repeatable.

## Current maturity limitations

| Area | Current limitation | Mitigation |
| --- | --- | --- |
| Layout | Very dense or unusual topologies may still need manual refinement | Use `inspect`, `improve-layout`, Draw.io refinement, and layout overrides |
| D2 backend | D2/ELK is deterministic and handles nested groups and parallel links, but explicit endpoint-side hints are only partially enforced and it lacks netdiag's network-specific finishing layer: device cards, role-specific icons, bundle semantics, and predictable interface-label placement | Use D2 when generic containment/routing is the priority; prefer native or Draw.io when network-specific presentation and endpoint control matter. See [d2-elk-spike.md](d2-elk-spike.md) |
| Templates | Phase 1 templates intentionally omit loops, conditionals, inheritance, and nested instantiation | Compose includes and explicit template instances |
| Learning curve | The schema and CLI expose many network-specific controls | Start with the gallery, templates, and worked round-trip example |
| PNG/PDF | Host exports require a local SVG converter | Use SVG directly or the provided Docker image |
| Custom icons | Unsafe or missing SVG assets are rejected or fall back to built-ins | Validate local icon packs and keep assets inside configured roots |
| Discovery | Parsers currently cover selected LLDP and IOS XR IS-IS formats | Normalize unsupported sources into topology YAML |
| Community | The project currently has a small contributor and example ecosystem | Treat compatibility fixtures and committed examples as the primary contract |
| Installation | Building the CLI requires Go | Use `go run`, build once, or use Docker |
| Native renderer maintenance | SVG geometry and routing are implemented directly in Go | Keep geometry deterministic, inspectable, and covered by focused regression tests |

## Not currently promised

- full fidelity for arbitrary edits made directly in Draw.io
- preservation of manually moved or rewritten generated labels
- automatic identity inference for renamed topology objects
- perfect layout repair for every graph
- a browser-based authoring UI
- dynamic topology monitoring

## Addressable next steps

The highest-value maturity work is:

- expand task-oriented docs, worked examples, and regression fixtures
- broaden templates without turning them into a general programming language
- add discovery parsers and export integrations based on concrete user demand
- improve command-specific help and simpler guided workflows
- continue strengthening inspection, repair, and topology-evolution reporting

Use `netdiag doctor drawio` before relying on a Draw.io file for round-tripping,
and use `netdiag diff-layout` to review the durable intent that changed.

See [round-trip.md](round-trip.md), [inspection.md](inspection.md),
[layout-repair.md](layout-repair.md), and [export.md](export.md) for the
supported workflows and contracts.
