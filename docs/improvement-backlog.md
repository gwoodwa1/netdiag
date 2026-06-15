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

**Done when:** each geometry defect receives a focused regression test, and
high-risk pure helpers have direct boundary-case coverage.

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

### Linting and error-handling audit

**Status:** open  
**Priority:** medium-high

CI currently runs formatting, `go vet`, tests, example validation/rendering,
generated-artifact freshness, and Markdown-link checks. Start with a pinned
`errcheck` step because silently ignored failures are especially risky in a CLI
that writes diagrams, reports, and extracted layout intent.

After the initial audit, evaluate:

- incomplete switches where typed enums are introduced (`exhaustive`)
- error wrapping where callers benefit from `errors.Is` / `errors.As`

Avoid enabling noisy style-only rules without a demonstrated maintenance
benefit.

**Done when:** `errcheck` is pinned, reproducible, low-noise, and enforced in
CI; any further lint rules have separately demonstrated value.

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

**Done when:** validation rules are easier to test and extend without changing
the public validation contract.

### Refactor native renderer shared state

**Status:** open

The native SVG renderer passes shared values such as the output buffer, icon
pack, theme, and premium state through many functions. Consider a renderer
context only where it reduces parameter plumbing without hiding dependencies.

**Done when:** rendering functions are easier to read and test, deterministic
output remains byte-identical, and no renderer-neutral responsibilities move
into the SVG package.

### Break up inline SVG definitions

**Status:** open

`renderDefinitions` contains large inline SVG/CSS fragments. Extract cohesive
helpers or carefully evaluated embedded fragments so definitions can be tested
and reviewed independently. This refactor is safer than its size first
suggests: CI already regenerates committed demos and fails when SVG output
changes unexpectedly, providing a broad regression guard alongside focused
tests.

**Done when:** filters, gradients, patterns, and theme definitions have clear
ownership and existing generated SVG remains current.

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
