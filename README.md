# Network Diagram Builder 

[![CI](https://github.com/gwoodwa1/netdiag/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/gwoodwa1/netdiag/actions/workflows/ci.yml?query=branch%3Amain+event%3Apush)
[![Go version](https://img.shields.io/github/go-mod/go-version/gwoodwa1/netdiag)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

This repository contains an early modern, YAML-driven network diagram builder
inspired by [cidrblock/drawthe.net](https://github.com/cidrblock/drawthe.net).

The first working slice renders deterministic spine-leaf SVG diagrams with
source interface labels, central link labels, and target interface labels.

**Netdiag does not try to make auto-layout perfect; it makes manual layout
repeatable.**

## Quick start

```sh
go run ./cmd/netdiag validate examples/spine-leaf.yaml
go run ./cmd/netdiag render examples/spine-leaf.yaml
```

The render command creates `examples/spine-leaf.svg`.

Build a standalone CLI:

```sh
go build -o netdiag ./cmd/netdiag
./netdiag render examples/spine-leaf.yaml
```

Try it in Docker with no local Go installation:

```sh
docker build -t netdiag . && docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" -w /work netdiag render examples/templates/national-telco-template.yaml -o /work/national-telco.png
```

## Fit and limitations

Netdiag is built for topology-aware, repeatable network documentation. It is a
good fit for YAML-owned topology, deterministic publication output, discovery
workflows, and Draw.io refinement with durable layout intent.

It is deliberately not a general-purpose diagramming canvas or a live
monitoring system. Dense topologies may still need human refinement. D2/ELK is
deterministic and useful for generic containment and parallel routing, but
explicit endpoint-side control is partial and it lacks the native renderer's
network-specific finishing layer. Templates have Phase 1 limits, and host
PNG/PDF exports require a local converter. The project also currently has a
small community and example ecosystem.

See [Fit and limitations](docs/fit-and-limitations.md) for the candid scope,
current maturity constraints, mitigations, and features that are not promised.
See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, architecture,
common change workflows, and the CI-equivalent verification command.

## Playground

Open the dependency-free [netdiag playground](docs/playground.html) to explore
a site-aware diagram with pan, zoom, element inspection, and collapsible
groups. It is generated entirely by netdiag and works without a server.

## Rendered gallery

| Site-aware WAN | Metro Ethernet ring |
| --- | --- |
| [![Site-aware WAN](examples/16-site-aware-wan.svg)](examples/16-site-aware-wan.svg) | [![Metro Ethernet ring](examples/14-metro-ethernet-ring.svg)](examples/14-metro-ethernet-ring.svg) |
| **Nord status styling** | **Premium telco composition** |
| [![Nord themed protocol and status links](examples/17-themed-link-status.svg)](examples/17-themed-link-status.svg) | [![Premium national telco topology](examples/templates/national-telco-premium.png)](examples/templates/national-telco-premium.png) |

See the [full example gallery](docs/gallery.md) for routing protocols, campus,
cloud, security, data-center, and service-provider topologies.

## Round-trip workflow

Generate an editable Draw.io diagram, polish it manually, extract only the
durable layout intent netdiag owns, then apply that intent as topology evolves:

```text
render -> edit -> extract-overrides -> re-render -> evolve -> layout-report
```

| Command | Purpose |
| --- | --- |
| `netdiag render topology.yaml --renderer drawio -o topology.drawio` | Generate an editable diagram |
| `netdiag extract-overrides topology.drawio --source topology.yaml -o topology.layout.yaml --report` | Extract supported human layout intent |
| `netdiag render topology.yaml --renderer drawio --layout-overrides topology.layout.yaml -o topology.drawio` | Reapply manual polish strictly |
| `netdiag render topology-v2.yaml --renderer drawio --layout-overrides topology.layout.yaml --layout-report -o topology-v2.drawio` | Preserve old polish, place new objects, and explain changes |

The reproducible [worked round-trip example](docs/round-trip.md) includes v1
and v2 topology YAML, layout overrides, generated Draw.io files, and a visual
raw-versus-refined IOS-XR discovery example.

### What survives regeneration?

| Draw.io edit | Preserved? |
| --- | --- |
| Move/resize managed nodes and groups | Yes |
| Adjust managed link waypoints, attachment sides, routing style, or lock state | Yes |
| Move or rewrite generated labels | No |
| Add annotations, decorative shapes, or unmanaged connectors | No |
| Rename nodes or add/remove topology links in Draw.io | No; edit topology YAML instead |

Check and review the lifecycle artefacts directly:

```sh
netdiag doctor drawio topology.drawio
netdiag diff-layout topology-v1.layout.yaml topology-v2.layout.yaml
netdiag diff-layout topology-v1.layout.yaml topology-v2.layout.yaml --json
netdiag render topology-v2.yaml --renderer drawio \
  --layout-overrides topology-v1.layout.yaml \
  --layout-report --output-report json -o topology-v2.drawio
```

## CLI workflow

```sh
netdiag schema > netdiag.schema.json
netdiag validate --json examples/spine-leaf.yaml
netdiag expand examples/templates/mpls-wan-template.yaml -o expanded.yaml
netdiag validate examples/includes/mpls-wan.yaml
netdiag templates
netdiag icons
netdiag render examples/custom-icon-pack.yaml --icons examples/custom-icons -o custom-icons.svg
netdiag fmt -w examples/spine-leaf.yaml
netdiag capabilities
netdiag recommend examples/spine-leaf.yaml
netdiag inspect examples/spine-leaf.yaml
netdiag improve-layout discovered.yaml -o discovered-improved.yaml
netdiag discover lldp show-lldp-neighbors-detail.txt --local leaf-01 -o discovered.yaml
netdiag discover lldp lldp-captures/ -o discovered-network.yaml
netdiag discover isis isis-captures/ -o discovered-isis.yaml
netdiag plan --renderer d2 examples/spine-leaf.yaml
netdiag render examples/spine-leaf.yaml -o examples/spine-leaf.svg
netdiag render examples/spine-leaf.yaml -o examples/spine-leaf.html
netdiag render examples/spine-leaf.yaml -o examples/spine-leaf.png
netdiag render examples/spine-leaf.yaml -o examples/spine-leaf.pdf
netdiag render examples/spine-leaf.yaml --renderer drawio -o examples/spine-leaf.drawio
netdiag render examples/spine-leaf.yaml --renderer drawio --layout-overrides spine-leaf.layout.yaml --layout-report -o examples/spine-leaf.drawio
netdiag extract-overrides examples/spine-leaf.drawio --source examples/spine-leaf.yaml -o spine-leaf.layout.yaml --report
netdiag doctor drawio examples/spine-leaf.drawio
netdiag diff-layout old.layout.yaml new.layout.yaml
```

When no renderer is selected, `netdiag render` recommends and selects one from
the diagram's requirements. CLI `--renderer` takes precedence over
`diagram.renderer`, which takes precedence over automatic recommendation.
Use `--report` to persist the capability assessment and warnings:

```sh
netdiag render examples/skills/d2-elk-hard-cases.yaml --renderer d2 --layout elk \
  --report render-report.json -o examples/skills/d2-elk-hard-cases.elk.svg
```

Inspect the native renderer's deterministic geometry for crossings, routes
through nodes, crowded endpoints, and interface-label collisions:

```sh
netdiag inspect examples/discovery/lldp-iosxr-8-site-dual-plane.yaml
netdiag inspect --json --fail-on error diagram.yaml
```

See [docs/inspection.md](docs/inspection.md) for finding codes and CI usage.

Apply bounded deterministic layout repair. Candidates are accepted only when
they improve the inspector objective:

```sh
netdiag improve-layout diagram.yaml -o diagram-improved.yaml
netdiag inspect diagram-improved.yaml
```

See [docs/layout-repair.md](docs/layout-repair.md) for the search policy and
tradeoffs.

The output extension selects SVG, interactive HTML, PNG, PDF, or editable
draw.io XML. The draw.io backend maps network roles to editable draw.io shapes
and preserves groups, nodes, links, and interface labels without requiring a
draw.io installation. HTML embeds
the native SVG with offline pan, zoom, inspection, and group-collapse controls.
PNG and PDF use a locally installed converter. See
[docs/export.md](docs/export.md) and [docs/interactive.md](docs/interactive.md).
LLDP discovery output from OpenConfig JSON, Cisco, Juniper, and Arista can be
converted into diagram YAML; see [docs/lldp.md](docs/lldp.md).
Cisco IOS XR IS-IS neighbor output can be converted into routing topology
YAML; see [docs/isis.md](docs/isis.md).

D2/ELK is an optional automatic-layout backend. It produces deterministic
output and handles nested containment and parallel links, but explicit endpoint
side hints are only partially enforced and it does not provide the native
renderer's network-aware finishing layer. See
[docs/d2-elk-spike.md](docs/d2-elk-spike.md) for the hard-case evidence and
[SKILLS.md](SKILLS.md) for the LLM repair loop.

## Architecture

Authored YAML is resolved and expanded into `spec.Document`, then validated and
compiled into the renderer-neutral `model.Diagram` intermediate
representation. Native SVG, D2, draw.io export, planning, and recommendation
code consume that IR rather than parsing source YAML. Renderer support is
advertised through the planner's `RendererCapability` contract, keeping
recommendation logic separate from backend implementation details.

## Template blocks

Reusable template blocks compose larger telco-style diagrams while keeping the
renderers simple. A diagram's `use` entries instantiate templates from
`templates/`, and `connect` adds links after expansion:

```yaml
use:
  - template: site.dual-pe
    as: london
    params:
      site_label: London

connect:
  - from: london-pe1:Ethernet0/0
    to: uk-core-p1:Ethernet0/0
    label: 100G
```

Render template diagrams directly, or inspect their normal canonical form:

```sh
netdiag render examples/templates/mpls-wan-template.yaml -o mpls-wan.svg
netdiag expand examples/templates/mpls-wan-template.yaml -o expanded.yaml
```

See [docs/templates.md](docs/templates.md) for the template format, naming
rules, parameters, and Phase 1 limitations.

The native renderer's deterministic offline icon catalog is available through
`netdiag icons` and `netdiag icons --json`. See [docs/icons.md](docs/icons.md).
Replace built-ins with a local SVG pack using `render --icons <directory>` or
`NETDIAG_ICONS`; missing or unsafe files fall back to the built-in catalog.
Use `diagram.theme: premium` for opt-in gradients, layered device cards,
status LEDs, cable underlays, and a subtle technical-grid background.
Use `diagram.theme: nord` or `diagram.theme: dracula` for a global dark color
scheme. Protocol and operational-state link rules can be declared once and
reused across links; status rules override protocol rules field-by-field:

```yaml
diagram:
  theme: nord
  link_styles:
    protocol:
      ospf: {color: "#a3be8c", pattern: solid, width: 3}
    status:
      inactive: {color: "#7b8496", pattern: dashed}

links:
  - from: core-01:Ethernet0/0
    to: core-02:Ethernet0/0
    protocol: ospf
    status: active
  - from: core-01:Ethernet0/1
    to: backup-01:Ethernet0/0
    protocol: ospf
    status: inactive
```

## Explicit includes

Split larger projects into normal YAML fragments with top-level `include`.
Paths resolve relative to the containing file, and included fragments may
instantiate templates:

```yaml
version: 1
include:
  - parts/sites.yaml
  - parts/core.yaml

diagram: {title: UK MPLS WAN, layout: sites}
connect:
  - from: london-pe1:Ethernet0/0
    to: uk-core-p1:Ethernet0/0
    label: 100G
```

Includes merge deterministically before template expansion. Duplicate IDs,
cycles, absolute paths, and paths escaping the entry diagram's directory are
rejected. See [docs/includes.md](docs/includes.md) for the full contract.

```yaml
links:
  - from:
      node: spine-01
      port: Ethernet1/1
      address: 10.10.10.1/30
    to:
      node: leaf-01
      port: Ethernet1/49
      address: 10.10.10.2/30
    labels:
      source: Eth1/1
      middle: 100G DWDM CKT-1001
      target: Eth1/49
```

Scalar endpoints and the legacy middle `label:` remain supported. Structured
endpoints add CIDR addresses and explicit source/middle/target labels without
forcing a heavier syntax on simple diagrams.

LACP bundles and VLAN trunks use structured link metadata:

```yaml
links:
  - from: leaf-01:Ethernet1/1
    to: app-01:Ethernet0/0
    label: 25G
    bundle: Port-Channel10
    lacp: true
    multi_chassis: true
    trunk:
      encapsulation: dot1q
      allowed_vlans: ["10", "20", "100-120"]
```

Physical bundle members remain visible. The topology uses compact bundle
markers such as `Po10`. Aggregate bandwidth, LACP, trunk encapsulation, and
allowed VLANs move into a fixed bundle-details legend in the left gutter so
adjacent bundles cannot create overlapping boxes.

Set `multi_chassis: true` when bundle members terminate on different switches.
The rendered caption then identifies the bundle as `MC-LAG · LACP`.

Full interface names remain in YAML, while rendered endpoint labels use common
network abbreviations. For example, `Ethernet0/0` renders as `Eth0/0`,
`GigabitEthernet0/1` as `Gi0/1`, and `TenGigabitEthernet1/1` as `Te1/1`.
Unknown long interface prefixes are shortened to five characters.

Nodes are automatically placed into rows based on their `role`. Within a row,
node IDs determine stable left-to-right placement. Add a small numeric `order`
to a node when topology meaning should control placement:

```yaml
nodes:
  west-router: {label: West Router, role: router, order: 10}
  core-router: {label: Core Router, role: router, order: 20}
  east-router: {label: East Router, role: router, order: 30}
```

Dense `hub-spoke` diagrams deliberately use oversized default cards: hub/core
nodes start very large and spoke/PE nodes start large. That is intentional for
telco diagrams where readable interface labels are more important than compact
cards. Individual nodes can still set explicit dimensions when a particular hub
or spoke needs even more room:

```yaml
nodes:
  core-hub-01:
    label: Core Hub 01
    role: core-router
    width: 900
    height: 260
  spoke-pe-17:
    label: Spoke PE 17
    role: edge-router
    width: 480
    height: 170
```

Set `diagram.layout: ring` to arrange ordered nodes clockwise around a resilient
ring. The first node is placed at the top:

```yaml
diagram: {title: Metro Ring, layout: ring, link_style: direct}
nodes:
  ring-01: {label: Ring Node 01, role: router, order: 10}
  ring-02: {label: Ring Node 02, role: router, order: 20}
```

Set `diagram.layout: sites` to make top-level groups into native site
containers. Devices are arranged into stable role rows inside each site,
core/WAN groups are placed between sites, and nested groups render as
subordinate boundaries. Site layouts automatically use deterministic,
obstacle-aware orthogonal routing:

```yaml
diagram: {title: Enterprise WAN, layout: sites, link_style: orthogonal}
groups:
  london:
    label: London
    kind: site
    nodes: {london-pe: {}}
  mpls-core:
    label: MPLS Core
    kind: core
    nodes: {p-01: {}}
```

See `examples/16-site-aware-wan.yaml` for a complete multi-site example.

The default `clean` link style uses aligned vertical port lead-ins before
crossing between layers. Bundled members converge through a compact circular
port-channel marker, keeping trunk metadata separate from physical interface
labels.

Interface labels are attached to the rendered route geometry. When a dense
diagram still needs manual label polish, endpoint labels can be nudged in YAML:

```yaml
from:
  node: core-hub-01
  port: HundredGigE0/0/0/0
  label_along: 0.22
  label_offset: 34
  label_rotation: 90
```

`label_along` is the normalized distance from that endpoint toward its peer.
`label_offset` moves the label perpendicular to the rendered route; negative
values flip it to the other side. These controls preserve manual intent during
regeneration better than moving generated labels in Draw.io.

Device cards include deterministic, original isometric SVG role icons inspired
by familiar network-stencil conventions. Spine switches use a multilayer
fabric-switch chassis, while leaf switches use a low-profile access-switch
chassis with a visible port bank.

Layer headings occupy a dedicated left gutter outside the topology placement
area. Links and devices cannot enter that gutter, so headings never mask or
overlap diagram geometry.

See [docs/gallery.md](docs/gallery.md) for seventeen additional rendered
examples covering WAN, DWDM, campus LAN, firewalls, wireless, SD-WAN, OT, AWS,
OSPF, IS-IS, BGP route reflection, Metro Ethernet rings, and MPLS metro
networks, including native site-aware containment and orthogonal routing.
