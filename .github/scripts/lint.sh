#!/usr/bin/env bash
set -euo pipefail

readonly version="v2.12.2"

lint="$(command -v golangci-lint || true)"
if [[ -z "$lint" ]]; then
  go_bin="$(go env GOPATH)/bin/golangci-lint"
  if [[ -x "$go_bin" ]]; then
    lint="$go_bin"
  fi
fi
if [[ -z "$lint" ]]; then
  echo "golangci-lint ${version} is required." >&2
  echo "Install it with:" >&2
  echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${version}" >&2
  exit 1
fi

installed="$("$lint" version --short 2>/dev/null || true)"
if [[ "$installed" != "$version" && "$installed" != "${version#v}" ]]; then
  echo "golangci-lint ${version} is required; found ${installed:-unknown}." >&2
  exit 1
fi

export GOCACHE="${GOCACHE:-/tmp/netdiag-go-build-cache}"
export GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/netdiag-golangci-lint-cache}"

"$lint" config verify
"$lint" run ./...
