#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

export GOCACHE="${GOCACHE:-/tmp/netdiag-go-build-cache}"

section() {
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    echo "::group::$1"
  else
    echo "==> $1"
  fi
}

end_section() {
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    echo "::endgroup::"
  fi
}

section "Formatting, vet, and tests"
test -z "$(gofmt -l cmd internal)"
go vet ./...
go test ./...
end_section

section "Validate and smoke-render examples"
for source in examples/*.yaml examples/templates/*.yaml examples/includes/*.yaml; do
  go run ./cmd/netdiag validate "$source" >/dev/null
done
go run ./cmd/netdiag validate examples/round-trip/topology-v1.yaml >/dev/null
go run ./cmd/netdiag validate examples/round-trip/topology-v2.yaml >/dev/null

mkdir -p /tmp/netdiag-ci
for source in examples/*.yaml; do
  output="/tmp/netdiag-ci/$(basename "${source%.yaml}").svg"
  go run ./cmd/netdiag render "$source" -o "$output" >/dev/null
done

# Exercise the shared interspersed parser and --key=value syntax on real
# commands, including documented input-first invocations.
go run ./cmd/netdiag render examples/spine-leaf.yaml \
  --renderer=native --output=/tmp/netdiag-ci/key-value.svg >/dev/null
go run ./cmd/netdiag expand examples/templates/mpls-wan-template.yaml \
  --output=/tmp/netdiag-ci/key-value-expanded.yaml >/dev/null
go run ./cmd/netdiag improve-layout examples/spine-leaf.yaml \
  --rounds=1 --max-candidates=1 --output=/tmp/netdiag-ci/key-value-improved.yaml >/dev/null
go run ./cmd/netdiag discover lldp examples/discovery/lldp-iosxr-captures \
  --format=auto --output=/tmp/netdiag-ci/key-value-discovered.yaml >/dev/null
go run ./cmd/netdiag inspect examples/spine-leaf.yaml --limit=1 >/dev/null
end_section

section "Regenerate committed demos"
go run ./cmd/netdiag render examples/17-themed-link-status.yaml \
  -o examples/17-themed-link-status.svg >/dev/null
go run ./cmd/netdiag render examples/16-site-aware-wan.yaml \
  -o docs/playground.html >/dev/null
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
go run ./cmd/netdiag extract-overrides examples/round-trip/topology-v1.drawio \
  --source=examples/round-trip/topology-v1.yaml \
  --output=/tmp/netdiag-ci/key-value.layout.yaml >/dev/null
go run ./cmd/netdiag discover lldp \
  examples/discovery/lldp-iosxr-8-site-dual-plane-captures \
  -o /tmp/lldp-iosxr-8-site-raw.yaml >/dev/null
go run ./cmd/netdiag render /tmp/lldp-iosxr-8-site-raw.yaml \
  -o examples/round-trip/iosxr-raw-discovery.svg >/dev/null

git diff --exit-code -- \
  examples/17-themed-link-status.svg \
  docs/playground.html \
  examples/round-trip/topology-v1.drawio \
  examples/round-trip/topology-v2.drawio \
  examples/round-trip/topology-v2.layout.yaml \
  examples/round-trip/iosxr-raw-discovery.svg
end_section

section "Documentation and diff checks"
python3 .github/scripts/check_markdown_links.py
git diff --check
end_section

echo "verification passed"
