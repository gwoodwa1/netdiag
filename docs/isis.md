# IS-IS discovery

`netdiag discover isis` converts routing adjacency output into normal netdiag
YAML. It supports Cisco IOS XR `show isis neighbors` and OpenConfig IS-IS
adjacency JSON:

```sh
netdiag discover isis show-isis-neighbors.txt -o isis-topology.yaml
netdiag discover isis openconfig-isis.json --format openconfig --local router-01 -o isis-topology.yaml
netdiag discover isis captures/ -o isis-network.yaml
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
