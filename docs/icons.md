# Built-in icon registry

Netdiag ships a deterministic offline icon registry used by the native SVG
renderer. List canonical icon IDs, categories, descriptions, colors, and role
aliases with:

```sh
netdiag icons
netdiag icons --json
```

Select a canonical icon explicitly on a node:

```yaml
nodes:
  edge:
    label: Internet Edge
    role: edge-router
    icon: router
```

When `icon` is omitted, the node role is resolved through the same registry.
Aliases such as `route-reflector`, `core-router`, `metro-switch`, and
`public-cloud` select the appropriate built-in glyph. Unknown IDs retain the
generic network-device fallback.

The registry contains original inline SVG glyphs and requires no remote assets.
