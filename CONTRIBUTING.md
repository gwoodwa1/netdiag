# Contributing to netdiag

Thank you for improving netdiag. The project values deterministic output,
focused changes, clear network semantics, and regression tests that explain why
a behavior matters.

## Development setup

Requirements:

- Go version declared in [`go.mod`](go.mod)
- golangci-lint `v2.12.2` for the focused lint check
- Python 3 for the Markdown-link check
- Git
- Docker only when testing the container or host-independent PNG/PDF export

Run the CLI without installing it:

```sh
go run ./cmd/netdiag validate examples/spine-leaf.yaml
go run ./cmd/netdiag render examples/spine-leaf.yaml -o /tmp/spine-leaf.svg
```

Run the same verification used by GitHub Actions:

```sh
./.github/scripts/lint.sh
./.github/scripts/verify.sh
```

The lint script runs the pinned, low-noise golangci-lint configuration used by
CI, including correctness analyzers and `gosec` security checks. The
verification script runs formatting checks, vet, tests, example validation and
smoke rendering, generated-artifact freshness checks, Markdown-link validation,
and `git diff --check`.

## Architecture and ownership

Authored YAML is decoded into `spec.Document`, validated, and compiled into the
renderer-neutral `model.Diagram`. Renderers and analysis tools consume that
model rather than parsing source YAML themselves.

| Area | Package |
| --- | --- |
| Source loading, includes, and YAML utilities | `internal/source`, `internal/yamlutil` |
| Schema and validation | `internal/spec` |
| Renderer-neutral compiled model | `internal/model` |
| Native SVG layout, routing, inspection, and rendering | `internal/svg` |
| Editable Draw.io render/extract lifecycle | `internal/drawio`, `internal/layoutoverride` |
| Optional D2/ELK backend | `internal/d2backend` |
| Discovery importers | `internal/lldp`, `internal/isis`, `internal/discoverylayout` |
| Templates and expansion | `internal/templates` |
| Export conversion and interactive HTML | `internal/export`, `internal/interactive` |
| CLI wiring | `cmd/netdiag` |

Keep network truth and reusable semantics in `spec`/`model`. Renderer-specific
geometry belongs in its renderer package.

## Change workflow

1. Make the smallest change that solves the behavior.
2. Add focused tests near the owning package.
3. Regenerate any committed outputs affected by the change.
4. Run `./.github/scripts/verify.sh`.
5. Review the final diff, especially generated SVG, HTML, Draw.io, and layout
   files.

Do not manually edit committed generated outputs when a reproduction command
exists.

## Common contributions

### Add or change native layout/routing

- Work in `internal/svg`.
- Add table-driven tests for pure geometry helpers where possible.
- Add a focused regression example when the visual behavior is important.
- Run `netdiag inspect` on dense affected examples.
- Confirm deterministic output with repeated renders.

### Add a discovery parser

- Keep vendor parsing inside the owning discovery package.
- Normalize names and reciprocal observations deterministically.
- Add representative raw capture fixtures and parser tests.
- Ensure generated topology validates and renders offline.

### Add a template

- Follow the Phase 1 contract in [docs/templates.md](docs/templates.md).
- Add the template to the registry and provide a renderable example.
- Keep parameters explicit; do not introduce hidden network semantics.

### Add an icon

- Follow [docs/icons.md](docs/icons.md).
- Keep built-in assets offline, deterministic, and license-safe.
- Test aliases, unsafe SVG rejection, and fallback behavior where relevant.

### Change Draw.io round-tripping

- Preserve the boundary documented in [docs/round-trip.md](docs/round-trip.md):
  topology YAML is network truth and layout YAML is durable human intent.
- Add strict and tolerant/report-mode tests.
- Keep `render -> extract -> render` byte-identical for supported edits.

## Generated examples

Committed demos are executable documentation.
`.github/scripts/regenerate.sh` is the executable manifest of generated
artifacts and their reproduction commands. The verification script regenerates
every path listed by `.github/scripts/regenerate.sh --list`, fails if any
tracked output drifts, and compares repeated representative renders.

Run the regeneration manifest directly after an intentional renderer,
discovery, or example change:

```sh
./.github/scripts/regenerate.sh
```

Custom icon SVGs, refined discovery YAML, and
`examples/round-trip/topology-v1.layout.yaml` are hand-authored or curated
inputs and are deliberately excluded from the generated-output manifest.

Useful locations:

- `examples/` for canonical user-facing diagrams
- `examples/regression/` for focused renderer cases
- `examples/discovery/` for discovery fixtures and outputs
- `examples/round-trip/` for the Draw.io lifecycle
- `docs/playground.html` for the generated interactive demo

## Scope and review guidance

Read [docs/fit-and-limitations.md](docs/fit-and-limitations.md) before expanding
the schema or product scope. Netdiag is a network topology lifecycle tool, not
a general-purpose diagramming canvas or monitoring platform.

Open engineering candidates and their current triage are recorded in
[docs/improvement-backlog.md](docs/improvement-backlog.md).
