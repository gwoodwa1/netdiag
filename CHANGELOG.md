# Changelog

All notable changes to netdiag are documented here. Releases follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.1] - 2026-09-20

### Security

- Upgraded the build, test, release, and container toolchain from Go 1.26.6 to Go 1.27.1.
- Pinned the Docker builder and CI toolchain checks to the exact Go patch release.
- Updated golangci-lint to v2.13.2 for Go 1.27 analysis support.
- Updated gosec to v2.28.0 for Go 1.27 analysis support.

## [0.1.0] - 2026-09-20

### Added

- LLDP and IS-IS topology discovery with multi-file capture support and automatic discovery layouts.
- Draw.io rendering, layout extraction, override files, layout reports, and topology-change-resistant round trips.
- Diagram inspection, renderer recommendations, capability reports, and iterative layout repair.
- Reusable topology templates, icon packs, includes, YAML formatting, schema output, and interactive HTML rendering.
- PNG and PDF export plus native, D2, and Draw.io renderer selection.
- CodeQL, dependency review, Dependabot, race and coverage tests, `govulncheck`, `gosec`, binary scanning, and container scanning.

### Changed

- Automatic native routing now shares geometry between rendering and inspection, honors endpoint direction and clearance, coordinates parallel paths, avoids nodes and labels, and reduces crossings.
- Connectivity-aware ordering and grouping improve generated topology layouts.
- Link labels and interface labels use improved placement and collision handling.
- CLI flags support input-first and `--key=value` forms consistently.
- Go dependencies and the build toolchain were updated to address known vulnerabilities.

### Fixed

- Repeated native and Draw.io renders are deterministic.
- Remote shared-node crossings are reported correctly during inspection.
- Nested groups, endpoint sides, structured link labels, network cards, and parallel links have dedicated regression coverage.

## [0.0.1] - 2026-06-11

### Added

- Initial YAML-driven network diagram renderer with deterministic SVG output and premium themes.

[Unreleased]: https://github.com/gwoodwa1/netdiag/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/gwoodwa1/netdiag/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/gwoodwa1/netdiag/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/gwoodwa1/netdiag/releases/tag/v0.0.1
