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
by the deterministic inspector. Candidates are evaluated concurrently, but
selection remains deterministic.

Only strict improvements are accepted. The objective is lexicographic:

1. Fewer errors, such as links passing through devices.
2. Fewer warnings, such as crossings and label collisions.
3. Fewer total findings.
4. Higher 0-100 inspection score.

This deliberately prioritizes removing severe errors even when a repair
temporarily introduces additional crossings. Run `netdiag inspect` on the
result before accepting it. Dense diagrams can remain at an inspection score
of zero while their underlying error count improves; the CLI reports that as
an error-first objective improvement.

Template and include inputs are resolved before repair, so the output is an
expanded canonical diagram rather than a patch to the original template.
