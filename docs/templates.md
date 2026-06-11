# Template blocks

Template blocks are reusable topology fragments for composing larger network
diagrams. They are a preprocessing feature: netdiag expands them into ordinary
groups, nodes, and links before validation, planning, or rendering. Renderers
never need to know that templates were used.

## Template format

Templates live under `templates/` by default and have a stable dotted ID:

```yaml
id: site.dual-pe
version: 1
description: Dual PE site block

params:
  site_label:
    type: string
    required: true
  pe1_label:
    type: string
    default: "{{ site_label }} PE1"

groups:
  "{{ instance }}":
    label: "{{ site_label }}"
    kind: site
    nodes:
      "{{ instance }}-pe1": {}

nodes:
  "{{ instance }}-pe1":
    label: "{{ pe1_label }}"
    role: edge-router
```

Phase 1 supports string interpolation for `{{ instance }}` and declared
parameters. Defaults may refer to other parameters.

## Using blocks

Add `use` entries to a diagram. `as` supplies the instance name used for
scoping IDs:

```yaml
version: 1
diagram: {title: MPLS WAN, layout: sites}

use:
  - template: site.dual-pe
    as: london
    params:
      site_label: London
```

If a template defines `{{ instance }}-pe1`, the expanded ID is `london-pe1`.
Expansion fails when instances produce duplicate node or group IDs.

Use `connect` to add links after all blocks have expanded. It accepts the same
shape as ordinary links, including structured endpoints:

```yaml
connect:
  - from:
      node: london-pe1
      port: Ethernet0/0
      address: 10.0.0.1/30
    to: core-p1:Ethernet0/0
    label: 100G
```

Every endpoint must reference a node that exists after expansion.

## Commands

Template diagrams render directly:

```sh
netdiag validate examples/templates/mpls-wan-template.yaml
netdiag render examples/templates/mpls-wan-template.yaml -o mpls-wan.svg
```

Inspect or persist the canonical expanded form with:

```sh
netdiag expand examples/templates/mpls-wan-template.yaml -o expanded.yaml
netdiag render expanded.yaml -o expanded.svg
```

Set `NETDIAG_TEMPLATES` to use a template directory other than `templates/`.

## Phase 1 limitations

Phase 1 intentionally has no conditionals, loops, inheritance, nested template
uses, or general-purpose template language. Parameters are strings, templates
expand in `use` order, and expansion is deterministic.
