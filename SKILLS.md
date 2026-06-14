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
8. Repair reported geometry issues using endpoint sides, positions, stubs,
   rotations, and layout clearance values.
9. If required, compare ELK explicitly:
   `netdiag render diagram.yaml --renderer d2 --layout elk -o diagram.d2.svg`.

Do not assume D2 endpoint-side hints are authoritative. The D2 spike proves
nested groups, routed links, three link-label positions, and parallel links.
Precise port-side placement and collision-free network labels remain owned by
the netdiag schema and finishing layer.

Fixtures:

- `examples/skills/d2-elk-hard-cases.yaml`
- `examples/skills/invalid-repair-loop.yaml`
- `examples/regression/`
