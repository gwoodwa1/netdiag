# Draw.io round-trip example

This directory demonstrates the full lifecycle:

```text
topology-v1.yaml -> topology-v1.drawio -> topology-v1.layout.yaml
                                      |
                                      +-> topology-v2.yaml -> topology-v2.drawio
                                                               |
                                                               +-> topology-v2.layout.yaml
```

Recreate the committed Draw.io files:

```sh
go run ./cmd/netdiag render examples/round-trip/topology-v1.yaml \
  --renderer drawio \
  --layout-overrides examples/round-trip/topology-v1.layout.yaml \
  -o examples/round-trip/topology-v1.drawio

go run ./cmd/netdiag render examples/round-trip/topology-v2.yaml \
  --renderer drawio \
  --layout-overrides examples/round-trip/topology-v1.layout.yaml \
  --layout-report \
  -o examples/round-trip/topology-v2.drawio
```

The second render preserves the polished v1 nodes and routes, places `edge-02`
near `core-b`, and reports the generated placement and route.

Review only the durable layout changes:

```sh
go run ./cmd/netdiag diff-layout \
  examples/round-trip/topology-v1.layout.yaml \
  examples/round-trip/topology-v2.layout.yaml
```

After editing `topology-v1.drawio` in diagrams.net, extract supported layout
intent again:

```sh
go run ./cmd/netdiag extract-overrides examples/round-trip/topology-v1.drawio \
  --source examples/round-trip/topology-v1.yaml \
  --report \
  -o examples/round-trip/topology-v1.layout.yaml
```
