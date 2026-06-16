# LLDP conversion

`netdiag discover lldp` converts LLDP discovery output into normal netdiag YAML. It
supports OpenConfig JSON and detailed show-command output from Cisco, Juniper,
and Arista:

```sh
netdiag discover lldp show-lldp-neighbors-detail.txt -o discovered.yaml
netdiag discover lldp juniper-lldp.txt --format juniper --local edge-01 -o discovered.yaml
netdiag discover lldp openconfig-lldp.json --format openconfig --local leaf-01 -o discovered.yaml
netdiag discover lldp captures/ -o discovered-network.yaml
netdiag discover lldp captures/ --auto-layout -o discovered-network.yaml
netdiag render discovered.yaml -o discovered.svg
```

`netdiag lldp` remains available as a compatibility alias.

Use `-` to read from standard input. The default `--format auto` recognizes
JSON and common vendor markers. Provide `--format` when captured output omits
those markers. `--format juniper-xml` explicitly selects Junos XML.

Show commands frequently omit the local hostname. In that case, pass
`--local`. Cisco output containing a prompt such as
`leaf-01# show lldp neighbors detail` supplies it automatically.
Cisco NFVIS `show switch lldp neighbors` summary tables are also supported.
Cisco IOS XR `show lldp neighbors [interface]` summary tables and XR
location-prefixed prompts are supported.
Juniper Junos `show lldp neighbors` summary tables and `user@switch>` prompts
are supported. Sectioned `show lldp neighbors detail` output is also supported,
including capabilities and management addresses. Junos
`show lldp neighbors detail | display xml` is supported with or without the
surrounding CLI prompt text.

A directory input merges all immediate `.txt`, `.log`, `.out`, `.json`, `.xml`,
and extensionless capture files into one topology. Reciprocal observations of the
same node-and-port endpoints become one link. Each capture should include a
detectable local hostname; when it does not, the filename stem is used. For
example, promptless output in `edge-01.txt` is attributed to `edge-01`.

The converter uses the remote system name as the node identity, falling back to
the chassis ID or management address. It skips incomplete records lacking a
local port, remote port, or remote identity. Chassis ID, management address,
system description, and capabilities are preserved as node metadata.

## IOS XR directory example

The repository includes four fake reciprocal IOS XR captures in
`examples/discovery/lldp-iosxr-captures/`. Build and render the merged topology
from the repository root:

```sh
go run ./cmd/netdiag discover lldp \
  examples/discovery/lldp-iosxr-captures \
  --auto-layout \
  -o examples/discovery/lldp-iosxr-ring.yaml

go run ./cmd/netdiag validate examples/discovery/lldp-iosxr-ring.yaml

go run ./cmd/netdiag render \
  examples/discovery/lldp-iosxr-ring.yaml \
  --renderer native \
  -o examples/discovery/lldp-iosxr-ring.svg
```

Each capture contains the local router prompt and its neighbor table. The
prompt identifies the local node, while each row supplies the remote node,
local interface, remote port ID, and capability. Eight directional
observations become four links because matching observations from both ends are
merged. `--auto-layout` detects this small cycle and selects the ring layout.

Interface labels render as rounded badges in the native SVG renderer. Customize
their colors and dimensions under `diagram.interface_label_style`:

```yaml
diagram:
  interface_labels: ends
  interface_label_style:
    fill: "#ffffff"
    color: "#334155"
    border: "#94a3b8"
    radius: 6
    padding_x: 10
    padding_y: 5
```

Omitted values use the default white badge, slate text and border, `5px`
corners, `9px` horizontal padding, and `5px` vertical padding.
Set `interface_labels: none` to hide endpoint interface labels.

To repeat this with real outputs, place one capture per device in a directory.
Keep the device prompt in each capture when possible. If a prompt is missing,
name the file after the local device, such as `edge-router-01.txt`; directory
discovery uses the filename stem as the fallback local node name.

## Eight-site IOS XR dual-plane example

`examples/discovery/lldp-iosxr-8-site-dual-plane-captures/` contains fake
reciprocal IOS XR LLDP captures for eight dual-PE sites:

- NYC, WAS, PHX, LAX, ORL, SFO, SAN, and DAL
- each site's `PE1` connects to both Plane A P routers
- each site's `PE2` connects to both Plane B P routers
- each P plane has a P1-to-P2 core link

Build the complete physical topology directly from the capture directory:

```sh
go run ./cmd/netdiag discover lldp \
  examples/discovery/lldp-iosxr-8-site-dual-plane-captures \
  --auto-layout \
  -o examples/discovery/lldp-iosxr-8-site-dual-plane.yaml

go run ./cmd/netdiag validate \
  examples/discovery/lldp-iosxr-8-site-dual-plane.yaml

go run ./cmd/netdiag render \
  examples/discovery/lldp-iosxr-8-site-dual-plane.yaml \
  -o examples/discovery/lldp-iosxr-8-site-dual-plane.svg
```

The directory contains 20 device captures. Discovery merges 68 directional
observations into 20 nodes and 34 physical links. Auto-layout recognizes
numbered network-role suffixes such as `NYC-PE1` and `CORE-A-P1`, producing
eight site groups and two P-plane groups. LLDP conversion assigns distinct PE
and P roles from those suffixes, while site layout expands high-degree P
routers to spread their many port labels across a larger device and canvas.
Because the topology contains a clear P-router core and several PE sites,
auto-layout selects `hub-spoke`: the two P planes are stacked centrally and
the eight sites are distributed above and below them. Hub-spoke mode uses very
large hub cards and large spoke cards by default so dense telco interface
labels have room before any manual polish. Hub-and-spoke links use separated
curved lanes with white crossing gaps. Interface labels are placed
proportionally along those routes, with labels for high-degree P routers moved
farther away from the hub to reduce core-label overlap. Lane geometry is
derived from endpoints and deterministic link order rather than device names
or topology-specific coordinates. Multi-homed PE links also fan out across
distinct rectangle sides when possible, preventing links from crossing
immediately after leaving the device. Explicit endpoint-side choices in YAML
remain authoritative. Before rendering, the SVG backend evaluates alternate
curved lanes for every hub-and-spoke link and selects a deterministic
combination that minimizes crossings across the complete diagram. Crossings
between links leaving the same device are also penalized when they use
different attachment points.

For diagrams that still need a manual routing adjustment, an endpoint can pin
both its rectangle side and its normalized position along that side:

```yaml
to:
  node: phx-pe1
  port: HundredGigE0/0/0/0
  side: top
  position: 0.25
```

`position` ranges from `0.0` to `1.0` and requires `side`. This keeps manual
adjustments stable when the canvas or device box changes size.

Use `stub` when a link should leave an endpoint in a straight line before
turning diagonal:

```yaml
to:
  node: orl-pe1
  port: HundredGigE0/0/0/0
  side: bottom
  position: 0.25
  stub: 180
```

`stub` is measured in SVG units and requires `side`. The global router still
optimizes the diagonal section after the requested straight departure.
The same controls work at either end of a link. For high-degree P routers,
assign upper-site links to ordered positions on `side: top` and lower-site
links to ordered positions on `side: bottom`, then add a shared stub distance.
This creates a clear routing channel around the P layer before links fan out
diagonally.

Port-label cards can also be rotated independently at either endpoint:

```yaml
from:
  node: core-a-p1
  port: HundredGigE0/0/0/0
  side: top
  position: 0.64
  stub: 140
  label_rotation: 90
```

Supported rotations are `0`, `90`, `180`, and `270` degrees. Rotation applies
to the complete label card.

The global crossing corrector also protects a configurable clearance around
routes, including near-misses that do not technically intersect:

```yaml
diagram:
  route_clearance: 30
  endpoint_clearance: 56
```

The native renderer defaults to `24` SVG units. Larger values make the route
planner move diagonal sections farther away from nearby lines and label
channels. `endpoint_clearance` defaults to `44` SVG units and enforces minimum
spacing between terminations sharing one device side. The renderer keeps the
declared side, then adjusts overly crowded positions just enough to satisfy
the available clearance.

## Architecture

The LLDP package separates format detection, vendor parsers, normalization,
topology conversion, and discovery reporting. Vendor adapters emit normalized
LLDP observations; only the converter creates netdiag documents. This keeps
future discovery protocols and vendor variants independent from rendering.
