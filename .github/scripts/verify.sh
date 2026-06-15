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
generated_outputs=()
while IFS= read -r output; do
  generated_outputs[${#generated_outputs[@]}]="$output"
done < <(.github/scripts/regenerate.sh --list)
.github/scripts/regenerate.sh
git diff --exit-code -- "${generated_outputs[@]}"
end_section

section "Check repeated-render determinism"
go run ./cmd/netdiag render examples/spine-leaf.yaml \
  -o /tmp/netdiag-ci/determinism-native-a.svg >/dev/null
go run ./cmd/netdiag render examples/spine-leaf.yaml \
  -o /tmp/netdiag-ci/determinism-native-b.svg >/dev/null
cmp /tmp/netdiag-ci/determinism-native-a.svg /tmp/netdiag-ci/determinism-native-b.svg

go run ./cmd/netdiag render examples/round-trip/topology-v2.yaml \
  --renderer drawio \
  --layout-overrides examples/round-trip/topology-v1.layout.yaml \
  -o /tmp/netdiag-ci/determinism-drawio-a.drawio >/dev/null
go run ./cmd/netdiag render examples/round-trip/topology-v2.yaml \
  --renderer drawio \
  --layout-overrides examples/round-trip/topology-v1.layout.yaml \
  -o /tmp/netdiag-ci/determinism-drawio-b.drawio >/dev/null
cmp /tmp/netdiag-ci/determinism-drawio-a.drawio /tmp/netdiag-ci/determinism-drawio-b.drawio
end_section

section "Documentation and diff checks"
python3 .github/scripts/check_markdown_links.py
git diff --check
end_section

echo "verification passed"
