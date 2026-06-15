#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

export GOCACHE="${GOCACHE:-/tmp/netdiag-go-build-cache}"

canonical_sources=(
  examples/01-wan-dwdm-campus.yaml
  examples/02-branch-mpls-wan.yaml
  examples/03-campus-lan.yaml
  examples/04-internet-dmz.yaml
  examples/05-aws-hybrid-cloud.yaml
  examples/06-retail-sdwan.yaml
  examples/07-wireless-campus.yaml
  examples/08-manufacturing-ot.yaml
  examples/09-dual-isp-headquarters.yaml
  examples/10-data-center-interconnect.yaml
  examples/11-ospf-multi-area.yaml
  examples/12-isis-levels.yaml
  examples/13-bgp-route-reflectors.yaml
  examples/14-metro-ethernet-ring.yaml
  examples/15-metro-mpls-core.yaml
  examples/16-site-aware-wan.yaml
  examples/17-themed-link-status.yaml
  examples/spine-leaf.yaml
)

regression_sources=(
  examples/regression/01-native-endpoint-sides.yaml
  examples/regression/02-d2-nested-groups.yaml
  examples/regression/03-parallel-links.yaml
  examples/regression/04-structured-link-labels.yaml
  examples/regression/05-native-network-cards.yaml
  examples/regression/06-capability-warning.yaml
)

generated_outputs=(
  docs/playground.html
  examples/01-wan-dwdm-campus.svg
  examples/02-branch-mpls-wan.svg
  examples/03-campus-lan.svg
  examples/04-internet-dmz.svg
  examples/05-aws-hybrid-cloud.svg
  examples/06-retail-sdwan.svg
  examples/07-wireless-campus.svg
  examples/08-manufacturing-ot.svg
  examples/09-dual-isp-headquarters.svg
  examples/10-data-center-interconnect.svg
  examples/11-ospf-multi-area.svg
  examples/12-isis-levels.svg
  examples/13-bgp-route-reflectors.svg
  examples/14-metro-ethernet-ring.svg
  examples/15-metro-mpls-core.svg
  examples/16-site-aware-wan.svg
  examples/17-themed-link-status.svg
  examples/spine-leaf.svg
  examples/discovery/isis-iosxr-80.svg
  examples/discovery/lldp-iosxr-8-site-dual-plane.svg
  examples/discovery/lldp-iosxr-ring.svg
  examples/regression/01-native-endpoint-sides.svg
  examples/regression/02-d2-nested-groups.svg
  examples/regression/03-parallel-links.svg
  examples/regression/04-structured-link-labels.svg
  examples/regression/05-native-network-cards.svg
  examples/regression/06-capability-warning.svg
  examples/round-trip/iosxr-raw-discovery.svg
  examples/round-trip/topology-v1.drawio
  examples/round-trip/topology-v2.drawio
  examples/round-trip/topology-v2.layout.yaml
  examples/skills/d2-elk-hard-cases.dagre.svg
  examples/skills/d2-elk-hard-cases.elk.svg
)

if [[ "${1:-}" == "--list" ]]; then
  printf '%s\n' "${generated_outputs[@]}"
  exit 0
fi
if [[ $# -ne 0 ]]; then
  echo "usage: $0 [--list]" >&2
  exit 2
fi

render_native() {
  local source="$1"
  go run ./cmd/netdiag render "$source" -o "${source%.yaml}.svg" >/dev/null
}

for source in "${canonical_sources[@]}" "${regression_sources[@]}"; do
  render_native "$source"
done

go run ./cmd/netdiag render examples/16-site-aware-wan.yaml \
  -o docs/playground.html >/dev/null

render_native examples/discovery/isis-iosxr-80.yaml

render_native examples/discovery/lldp-iosxr-ring.yaml

render_native examples/discovery/lldp-iosxr-8-site-dual-plane.yaml

go run ./cmd/netdiag render examples/skills/d2-elk-hard-cases.yaml \
  --renderer d2 --layout dagre \
  -o examples/skills/d2-elk-hard-cases.dagre.svg >/dev/null
go run ./cmd/netdiag render examples/skills/d2-elk-hard-cases.yaml \
  --renderer d2 --layout elk \
  -o examples/skills/d2-elk-hard-cases.elk.svg >/dev/null

go run ./cmd/netdiag render examples/round-trip/topology-v1.yaml \
  --renderer drawio \
  --layout-overrides examples/round-trip/topology-v1.layout.yaml \
  -o examples/round-trip/topology-v1.drawio >/dev/null
go run ./cmd/netdiag render examples/round-trip/topology-v2.yaml \
  --renderer drawio \
  --layout-overrides examples/round-trip/topology-v1.layout.yaml \
  --layout-report \
  -o examples/round-trip/topology-v2.drawio >/dev/null
go run ./cmd/netdiag extract-overrides examples/round-trip/topology-v2.drawio \
  --source examples/round-trip/topology-v2.yaml \
  -o examples/round-trip/topology-v2.layout.yaml >/dev/null
go run ./cmd/netdiag doctor drawio examples/round-trip/topology-v2.drawio >/dev/null
go run ./cmd/netdiag diff-layout \
  examples/round-trip/topology-v1.layout.yaml \
  examples/round-trip/topology-v2.layout.yaml --json=true >/dev/null

go run ./cmd/netdiag discover lldp \
  examples/discovery/lldp-iosxr-8-site-dual-plane-captures \
  -o /tmp/lldp-iosxr-8-site-raw.yaml >/dev/null
go run ./cmd/netdiag render /tmp/lldp-iosxr-8-site-raw.yaml \
  -o examples/round-trip/iosxr-raw-discovery.svg >/dev/null
