# Explicit includes

Includes split a large diagram into smaller project files without YAML anchors
or copied topology. They are resolved before template expansion, validation,
planning, and rendering.

```yaml
version: 1

include:
  - parts/sites.yaml
  - parts/core.yaml

diagram:
  title: UK MPLS WAN
  layout: sites

connect:
  - from: london-pe1:Ethernet0/0
    to: uk-core-p1:Ethernet0/0
    label: 100G
```

An included file uses the normal diagram format and may contain `groups`,
`nodes`, `links`, `use`, `connect`, and further `include` entries:

```yaml
version: 1

use:
  - template: site.dual-pe
    as: london
    params:
      site_label: London
```

## Resolution and merging

- Paths are relative to the file containing the `include`.
- Includes merge in declaration order, followed by the containing file.
- Links, `use`, and `connect` entries append in that order.
- The entry file owns `diagram` display and layout settings.
- Duplicate node or group IDs are errors.
- Included template instances expand after all files have merged.
- `netdiag expand` outputs one canonical file without `include`, `use`, or
  `connect`.

## Path safety

Includes must stay within the directory containing the entry diagram.
Absolute paths and `..` paths that escape this project root are rejected.
Include cycles are rejected with the chain of files involved.

## Commands

All diagram-consuming commands resolve includes automatically:

```sh
netdiag validate examples/includes/mpls-wan.yaml
netdiag render examples/includes/mpls-wan.yaml -o included-wan.svg
netdiag expand examples/includes/mpls-wan.yaml -o included-wan-expanded.yaml
```
