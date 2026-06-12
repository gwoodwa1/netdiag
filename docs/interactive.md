# Interactive HTML previews

Native diagrams can be exported as dependency-free, single-file interactive
HTML:

```sh
netdiag render examples/templates/national-telco-template.yaml \
  --icons examples/custom-icons \
  -o national-telco.html
```

The HTML embeds the deterministic native SVG, diagram model, styles, and
JavaScript. It does not load libraries or assets from the network.

Features:

- wheel zoom, drag pan, zoom buttons, and fit-to-page
- click-to-inspect nodes, links, and groups
- node metadata and complete link endpoint details in the sidebar
- top-level group controls that collapse contained nodes and connections
- one portable file suitable for local viewing or static web hosting

Interactive HTML currently requires the native renderer. Group collapse hides
content without recalculating layout; expanding restores the original
deterministic SVG.

The repository includes a ready-to-open [playground](playground.html),
generated from `examples/16-site-aware-wan.yaml`, for trying these controls
without first building the CLI.
