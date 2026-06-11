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

## Local SVG icon packs

The native renderer can replace built-in glyphs with SVG files from a local
directory:

```sh
netdiag render examples/custom-icon-pack.yaml \
  --icons examples/custom-icons \
  -o custom-icon-pack.svg
```

Name each file after the node's `icon` or role, for example `router.svg` or
`firewall.svg`. An exact match such as `edge-router.svg` wins first. Otherwise,
a canonical file such as `router.svg` replaces all built-in router aliases.

Set `NETDIAG_ICONS=/path/to/icons` to use a pack without passing `--icons` on
every render. Custom packs apply to the native renderer only.

Icons are embedded into the output, so rendered diagrams remain portable and
offline. SVGs must have a `viewBox`; common paths, shapes, groups, gradients,
and clip paths are supported. Scripts, event handlers, text, images, external
references, and unsupported elements are rejected. If a requested file is
missing, malformed, or unsafe, netdiag quietly uses the matching built-in
glyph instead.

See [`examples/custom-icons`](../examples/custom-icons) for a small example
pack.
