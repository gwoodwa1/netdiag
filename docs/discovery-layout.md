# Discovery auto-layout

All discovery protocols use the same deterministic layout analysis:

```sh
netdiag discover lldp captures/ --auto-layout -o topology.yaml
netdiag discover isis captures/ --auto-layout -o topology.yaml
netdiag render topology.yaml -o topology.svg
```

The shared analyzer currently:

- selects `ring` for small cycle topologies;
- keeps small non-ring discoveries in `rows`;
- selects wrapped `sites` with orthogonal routing for large discoveries;
- groups large topologies using useful hostname prefixes;
- falls back to deterministic balanced clusters when names do not expose
  grouping information;
- enables endpoint interface-label badges for large diagrams; and
- suppresses heavily repeated middle labels.

The generated YAML contains every decision, so it remains inspectable,
reproducible, and editable without an AI assistant. Future discovery protocols
such as CDP or BGP should call the same analyzer.
