#!/bin/sh

set -eu

release_root="${1:-dist}"
scanner="${GOVULNCHECK:-govulncheck}"

find "$release_root" -type f \( -name netdiag -o -name netdiag.exe \) -print |
while IFS= read -r binary; do
	echo "=== metadata ${binary} ==="
	go version -m "$binary"
	echo "=== vulnerability scan ${binary} ==="
	"$scanner" -mode binary "$binary"
done

if ! find "$release_root" -type f \( -name netdiag -o -name netdiag.exe \) -print -quit | grep -q .; then
	echo "no release binaries found under ${release_root}" >&2
	exit 1
fi
