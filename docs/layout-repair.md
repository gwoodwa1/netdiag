# Deterministic layout repair

`netdiag improve-layout` searches a bounded set of YAML changes and writes a
new canonical diagram:

```sh
netdiag improve-layout diagram.yaml -o diagram-improved.yaml
netdiag improve-layout diagram.yaml --rounds 3 --max-candidates 80 --json
```

The input file is never overwritten. Without `-o`, output is written beside
the input as `<name>.improved.yaml`.

The repairer tries global routing and clearance settings first. It then tries
targeted endpoint sides, positions, stubs, and label rotations on links named
by the deterministic inspector. Unreadable source/target label pairs are
prioritized and can be separated with independent rotations or attachment
positions. Links passing behind the same unrelated node can be rerouted as a
group to remove apparent unlabeled endpoints. Candidates are evaluated
concurrently, but selection remains deterministic.

Only strict improvements are accepted. The objective is:

1. Lower weighted inspection penalty: errors cost 20, warnings cost 5, and
   informational findings cost 1.
2. Fewer errors when penalties are equal.
3. Fewer warnings and total findings.
4. Higher 0-100 inspection score.

This allows a repair to introduce a small number of warnings when it removes a
severe error, but rejects changes that make the overall diagram materially
busier. Run `netdiag inspect` on the result before accepting it.

Template and include inputs are resolved before repair, so the output is an
expanded canonical diagram rather than a patch to the original template.

## Repository corpus audit

The repairer is regression-tested against the repository's existing examples.
Using two rounds and a 20-candidate budget on June 14, 2026:

- 39 valid diagrams were inspected, repaired, validated, and rendered.
- 25 diagrams started clean and remained unchanged.
- 13 diagrams accepted at least one strict improvement.
- Weighted penalty fell from 2015 to 1645.
- Errors fell from 20 to 13.
- Warnings fell from 323 to 277.
- No accepted repair increased weighted penalty or error count.

The intentionally invalid repair-loop fixture is expected to fail before
layout repair because it references an unknown node.
