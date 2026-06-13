# IS-IS discovery

`netdiag discover isis` converts routing adjacency output into normal netdiag
YAML. It supports Cisco IOS XR `show isis neighbors` and OpenConfig IS-IS
adjacency JSON, plus Junos IS-IS adjacency XML:

```sh
netdiag discover isis show-isis-neighbors.txt -o isis-topology.yaml
netdiag discover isis openconfig-isis.json --format openconfig --local router-01 -o isis-topology.yaml
netdiag discover isis junos-isis.xml --format juniper-xml --local router-01 -o isis-topology.yaml
netdiag discover isis captures/ -o isis-network.yaml
netdiag discover isis captures/ --auto-layout -o isis-network.yaml
netdiag render isis-network.yaml -o isis-network.svg
```

The parser recognizes IOS XR prompts and collection wrappers such as:

```text
+++ R2_xr: executing command 'show isis neighbors' +++
```

When a local hostname is absent, use `--local` for one file. Directory imports
use each filename stem as the fallback local hostname.

IS-IS links preserve the local interface, remote SNPA, instance, adjacency
type, state, holdtime, and IETF NSF capability. Reciprocal observations with
matching endpoints are merged when the available output identifies the same
endpoints; identical duplicate observations are always collapsed. IOS XR
summary output exposes an SNPA rather than the remote interface, so reciprocal
captures may remain as separate directional observations.

OpenConfig JSON is parsed namespace-tolerantly. Interface and level context are
inherited from the containing OpenConfig lists. Because OpenConfig adjacency
state commonly omits a remote interface/SNPA, the generated remote endpoint is
named `isis-adjacency`.

Junos `show isis adjacency | display xml` is namespace-tolerant and extracts
the local interface, system name, level, adjacency state, and holdtime. Raw XML
requires `--local`; surrounding `user@router>` prompt text supplies it
automatically.

## 80-node IOS XR example

The repository includes a deterministic generator for a large fake IOS XR
network. It creates eight interconnected 10-router pods, with four L2
adjacencies per router:

```sh
go run ./examples/discovery/generate_isis_iosxr_80.go

go run ./cmd/netdiag discover isis \
  examples/discovery/isis-iosxr-80-captures \
  --auto-layout \
  -o examples/discovery/isis-iosxr-80.yaml

go run ./cmd/netdiag validate examples/discovery/isis-iosxr-80.yaml

go run ./cmd/netdiag render \
  examples/discovery/isis-iosxr-80.yaml \
  --renderer native \
  -o examples/discovery/isis-iosxr-80.svg
```

The directory import reads 320 reciprocal observations from 80 captures and
merges them into 160 physical links. `--auto-layout` recognizes hostname
prefixes as routing clusters, selects the wrapped site layout, enables
orthogonal routing and interface badges, and removes repeated middle labels
that add noise at this scale. When hostnames do not expose useful prefixes,
large discoveries fall back to deterministic balanced clusters. This is the
same protocol-neutral analysis used by LLDP and future discovery commands; see
[discovery-layout.md](discovery-layout.md).
