# Improvement backlog

This backlog records external review suggestions and current project triage.
It is intentionally not a promise or release plan. Items should be promoted
into focused issues when there is a concrete use case, owner, and acceptance
criteria.

Last reviewed: June 15, 2026.

## Priority candidates

### Contributor development guide

**Status:** addressed  
**Priority:** high

`CONTRIBUTING.md` now covers:

- supported Go version and local/Docker setup
- test, vet, formatting, and CI-equivalent commands
- package ownership and renderer-neutral model boundaries
- adding icons, discovery parsers, templates, and layout modes
- purpose and regeneration commands for committed examples

GitHub Actions and contributors both run `.github/scripts/verify.sh`, keeping
the documented local verification command aligned with CI.

### Focused geometry and routing tests

**Status:** partially addressed  
**Priority:** high

The original review identified native SVG geometry as a major risk. Coverage
has improved substantially: `internal/svg` now has focused inspection tests and
50 render tests, and CI runs the full suite. Continue adding table-driven tests
for pure geometry and routing helpers when defects or new edge cases are found.

Candidate areas:

- endpoint attachment and default-side selection
- orthogonal route construction and obstacle avoidance
- label offsets, rotations, and collision resolution
- dense parallel-link and nested-group cases
- invariant or property-based tests for pure routing helpers where they catch
  cases more effectively than table-driven examples

**Done when:** each geometry defect receives a focused regression test, and
high-risk pure helpers have direct boundary-case coverage.

### Strengthen ISIS and YAML diagnostic coverage

**Status:** open

**Priority:** medium-high

LLDP and specification parsing have broad focused coverage, while the ISIS
package currently has one main happy-path test for each supported format.
`internal/yamlutil` is exercised indirectly by callers but has no direct tests
for its strict decoding and compiler-style diagnostic behavior.

Candidate ISIS cases:

- malformed and empty IOS XR, Junos XML, and OpenConfig inputs
- partial rows, missing fields, invalid holdtimes, and malformed system IDs
- local-node detection variants and format auto-detection failures
- reciprocal-link deduplication and incomplete-neighbor conversion behavior

Candidate YAML diagnostic cases:

- unknown fields and malformed YAML
- empty input and multi-document input behavior
- line/column excerpts at the beginning and end of a file
- errors without locations and deeply nested input

**Done when:** each supported ISIS format has focused failure and partial-input
coverage, and `yamlutil.DecodeStrict`/`Context` have direct stable diagnostic
tests.

### Consistent CLI parsing and command help

**Status:** addressed  
**Priority:** high

All option-bearing commands now use a shared `flag.FlagSet` convention with an
interspersed-argument normalizer. Existing input-first commands remain valid,
and options consistently support both `--key value` and `--key=value`.

Focused parser tests cover input-first options, aliases, boolean flags, stdin
(`-`), and literal arguments after `--`. The shared verification script also
exercises `--key=value` through real render, extraction, discovery, expansion,
repair, inspection, and layout-diff commands.

### Strengthen deterministic output regression coverage

**Status:** addressed

**Priority:** high

`.github/scripts/regenerate.sh` is the executable manifest for every committed
generated artifact. CI regenerates every listed SVG, Draw.io, layout, discovery,
and playground output and fails on drift. It also compares repeated native and
Draw.io renders, while Draw.io extraction retains its byte-identical
render-extract-render test. Hand-authored custom icons and the initial round-trip
layout intent are explicitly excluded, as are curated discovery YAML files that
preserve refinements beyond raw import output.

The repeated-render comparison is distinct from freshness checking. A
non-deterministic render can occasionally match the committed artifact and pass
a single-run freshness check; comparing two outputs from the same input exposes
that latent flakiness directly.

### Linting and error-handling audit

**Status:** addressed

**Priority:** medium-high

CI and local development run pinned golangci-lint `v2.12.2` with its
conservative standard analyzer set plus `errorlint` and `gosec`. This includes
`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, and focused
security checks without enabling the noisy style-heavy full set. The initial
audit fixed ignored flag/output/temporary-file errors, wrapped EOF comparisons,
and a dead helper. Quick-fix and error-string style suggestions are explicitly
excluded. Gosec's generic user-path and restrictive-file-mode rules are also
excluded because netdiag intentionally reads user-selected topology assets and
writes shareable diagrams; deliberate subprocess and tainted-path cases retain
local rationale comments.

After the initial audit, evaluate:

- incomplete switches where typed enums are introduced (`exhaustive`)
- error wrapping where callers benefit from `errors.Is` / `errors.As`

Avoid enabling noisy style-only rules without a demonstrated maintenance
benefit.

Further lint rules should be enabled only when they have separately demonstrated
value.

### Bound resource use for untrusted inputs

**Status:** open

**Priority:** high

Custom SVG icons already have a size cap, an element and attribute allowlist,
and focused unsafe-input tests. Other inputs are less consistently bounded.
In particular, compressed Draw.io page payloads are inflated with an unbounded
read, and several YAML, discovery, include, and stdin paths read the complete
input before parsing.

This is primarily a denial-of-service and accidental-resource-exhaustion risk
when processing very large or highly compressed files. It is not evidence that
the current XML parser expands external entities.

**Sequencing:** coordinate this with “Make CLI command execution directly
testable.” Resource limits should live in reusable readers/parsers that command
helpers can call in-process; otherwise important stdin and oversized-file tests
remain unnecessarily dependent on subprocess execution.

**Done when:**

- Draw.io file and decompressed-page sizes have explicit limits with clear
  diagnostics
- YAML, include, discovery, and stdin reads use documented practical limits
- parsers cap relevant object counts or nesting where file-size limits alone are
  insufficient
- tests cover oversized files and compressed payloads with extreme expansion
- a consistently named CLI or configuration option lets trusted workflows
  deliberately raise documented default limits when needed

### Add large-topology performance budgets

**Status:** open

**Priority:** medium-high

The repository has substantial topology fixtures, including the 80-node IOS XR
example, but it has no Go benchmarks or documented performance budgets. That
makes regressions in render, inspect, discovery, and layout work difficult to
detect before users notice them.

**Done when:**

- representative small, medium, and large fixtures have benchmarks for the
  important lifecycle operations
- benchmarks report allocations and separate build/startup cost from command
  execution cost
- Draw.io round-tripping and other memory-heavy paths record peak-memory or
  high-water-mark measurements in addition to Go allocation counts
- CI or a documented release check tracks practical regression budgets
- profiling identifies measured hotspots before optimization work is accepted

## Medium-term maintainability

### Replace repeated domain strings with owned typed constants

**Status:** open

Values such as renderer, layout, route style, and theme names recur across
packages. Introduce typed constants in the package that owns each concept
rather than a broad shared constants package.

**Done when:** validation and switches use owned domain types where that reduces
invalid states and duplicated string comparisons.

### Decompose specification validation

**Status:** open

`spec.Validate` is a large sequential validator. Split it into focused
validators for diagram metadata, nodes, links, groups, and cross-references
while preserving deterministic problem ordering and existing error messages.
It currently collects plain strings, which also makes validation output
difficult for other tools to consume reliably.

**Done when:** validation rules are easier to test and extend; failures expose a
stable structured form such as code, object path, and message; and CLI text
output remains compatible and readable.

### Make CLI command execution directly testable

**Status:** open

The shared `flag.FlagSet` layer fixed the inconsistent option parsing problem,
but command orchestration still mixes output, process exit behavior, and command
logic in places. This encourages subprocess-only testing and makes individual
failure paths harder to exercise.

This item is an enabling dependency for complete resource-bounds coverage:
decoupled command helpers make stdin limits, oversized files, diagnostics, and
exit-code behavior practical to test directly.

**Done when:** command helpers return errors or explicit result/status values;
`main` owns final stderr formatting and exit-code selection; representative
success and failure paths can be tested in-process; and user-facing output and
exit-code compatibility are preserved.

### Measure and control the native CLI build footprint

**Status:** open

A current Darwin arm64 Go 1.26.3 build is approximately 35 MB and the CLI
dependency graph contains roughly 325 packages. The optional D2 backend brings
in a substantial part of that graph. This is not automatically a defect, but
the cost should be measured and intentional.

**Done when:** release builds record binary-size and dependency-footprint
baselines; optional backend costs are measured, including a prototype no-D2
build variant; unnecessary dependencies are removed where doing so has a
meaningful benefit; and packaging changes such as build tags or split binaries
are adopted only if measurements justify the added complexity. Any decision to
omit D2 from the default binary must account for CLI compatibility and actual
user demand.

### Automate releases and binary distribution

**Status:** open

CI verifies the repository, but there is no automated tagged release workflow
for distributing versioned CLI binaries and checksums.

**Done when:** a tagged release produces tested binaries for supported
platforms, artifacts include checksums and reproducible version information,
the container image release path is documented or automated, and additional
channels such as Homebrew are added only when demand justifies their maintenance
cost.

### Define schema evolution and migration tooling

**Status:** open

The topology, template, and layout-override formats currently require
`version: 1` and reject other versions. That is appropriate today, but a
compatibility and migration path should be designed before the first v2 format
change rather than after users have incompatible files.

**Done when:** supported-version policy is documented; compatibility fixtures
exist for every supported version; deprecated or moved fields produce actionable
diagnostics; and a `migrate` command or equivalent safe rewrite path can upgrade
files when a new schema version is introduced.

### Refactor native renderer shared state

**Status:** addressed

The native renderer is split into focused files for definitions/themes,
backgrounds, labels/bundles, nodes/icons, layout, routing, inspection, and
orchestration. A narrow renderer context owns only the main output buffer and
immutable visual dependencies. Documents, layouts, routes, geometry, label
styles, and the phase-local annotation buffer remain explicit.

Deterministic output remains byte-identical, visual helpers remain directly
testable, and no renderer-neutral responsibilities moved into the SVG package.

### Break up inline SVG definitions

**Status:** partially addressed

`renderDefinitions` and theme CSS now live in `internal/svg/definitions.go`,
separate from orchestration. The definitions remain inline fragments and could
still be split into cohesive helpers if a concrete review or testing need
justifies it.

**Done when:** filters, gradients, patterns, and theme definitions are
independently testable where that adds value, and existing generated SVG remains
current.

## Product expansion candidates

Promote these only from concrete user demand:

- broaden templates without turning them into a general programming language
- add discovery parsers and export integrations
- expand task-oriented docs, worked examples, and regression fixtures
- improve guided CLI workflows for users who do not need the full schema
- continue strengthening inspection, repair, and topology-evolution reporting

## Addressed since the review

- **CI pipeline:** `.github/workflows/ci.yml` now runs formatting, vet, tests,
  example validation/rendering, generated-demo freshness, and Markdown links.
- **Test coverage:** all substantial renderer/model/template/planner/source/spec
  packages now have tests; SVG coverage includes inspection and render cases.
- **Draw.io lifecycle:** render, extraction, metadata doctoring, layout diffing,
  topology evolution, human/JSON reports, worked examples, and CI freshness
  checks are implemented.
- **Limitations documentation:** scope, maturity constraints, mitigations, and
  D2/ELK tradeoffs are documented explicitly.

## Rejected or outdated findings

### “No CI/CD pipeline”

Outdated. CI exists and is required to keep committed demos current.

### “Go 1.26 is a typo/future version”

Outdated. The project currently targets Go 1.26 and is verified with Go
`1.26.3`. Any future version change should be treated as an explicit support
policy decision, not an automatic downgrade.

### Exact error/test-count claims from the original review

Do not treat historical counts as current facts. Re-audit the current tree
before creating work from them.

### “`cmd/netdiag` has zero tests”

Overstated. The package has focused shared-flag parser tests, but command
execution in `main.go` remains largely subprocess-only and is still a
significant coverage gap tracked above.

### Broad old-style error-wrapping counts

Do not promote a raw `%v`/`%s` count without classifying occurrences. The
reviewed Draw.io, ISIS, LLDP, and CLI production paths already use `%w` where
callers benefit from preserving an underlying error; several apparent
old-style occurrences are test failure messages rather than wrapped production
errors. Continue the targeted error-handling audit instead.

### “Replace CLI parsing with Cobra to support `--key=value`”

Outdated. Commands now use the shared standard-library `flag.FlagSet` parser,
support both option forms, and have focused parsing tests. A framework change is
not justified without a remaining concrete problem.

### “Custom SVG icons lack input safety controls”

Outdated as a general claim. Icons have a size cap, allowlisted SVG content,
external-reference rejection, and unsafe-input tests. Broader input resource
limits remain open above.

### Introduce a layout-engine interface immediately

Not promoted. Layout behavior is already separated in `internal/svg/layout.go`;
an interface should be introduced only when a concrete second implementation or
testing need makes it useful.

### Bundle a pure-Go or browser-based export converter

Not promoted. Host-side PNG/PDF conversion remains an explicit dependency, and
the Docker workflow provides the supported packaged path.
