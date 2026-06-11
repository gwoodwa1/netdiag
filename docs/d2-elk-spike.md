# D2/ELK Backend Spike

The spike is available through:

```sh
netdiag render examples/skills/d2-elk-hard-cases.yaml \
  --renderer d2 --layout elk -o examples/skills/d2-elk-hard-cases.svg
```

## Results

| Hard case | Result |
| --- | --- |
| Nested groups | Pass: nested D2 containers render through ELK |
| Port placement | Partial: endpoint labels work, explicit side hints are not reliably enforced |
| Source interface label | Pass: D2 source-arrowhead label |
| Middle link label | Pass: D2 edge label |
| Target interface label | Pass: D2 target-arrowhead label |
| Parallel links | Pass: distinct routed connections and labels |
| Manual escape hatches | Partial: schema retains endpoint side hints; D2 adapter does not promise them |
| Stable deterministic output | Pass: identical IR renders byte-identical SVG in regression tests |

Run `netdiag capabilities`, `netdiag plan`, or render with `--report` to see
these limits as machine-readable capability levels rather than relying on this
table alone.

## Alternatives Considered

yFiles is a serious commercial graph-visualization SDK with mature automatic
layout and extensive customization. It is a credible candidate for producing
stronger generic layouts than ELK, but it is proprietary, requires a paid
license for production use, and does not provide a native Go SDK. It has not
been benchmarked by this spike.

TALA may produce more polished D2 layouts because it is designed around
orthogonal paths, first-class containers, icons, and label collision avoidance.
However, unrestricted local use requires a license key, and server or CI/CD use
requires an Enterprise license. D2 itself remains open source; TALA does not
share that unrestricted licensing model.

Official references:

- yFiles SDK and licensing: https://www.yfiles.com/the-yfiles-sdk
- D2 layout-engine tradeoffs: https://d2lang.com/tour/layouts/
- TALA features and licensing: https://terrastruct.com/tala/

## Decision

D2/ELK is retained as an optional automatic-layout backend for diagrams where
nested containment and parallel routing are more important than
network-specific presentation.

It does not replace the native renderer.

The spike confirms that generic layout engines can solve parts of the placement
problem, but they do not provide the network-aware finishing layer required for
high-quality infrastructure diagrams: device cards, role-specific icons,
stable endpoint-side control, bundle semantics, and predictable
interface-label placement.

The native renderer now includes an initial site-aware layout mode and a small
obstacle-aware orthogonal routing layer. Further visual-quality work should
strengthen that network-aware layer rather than search for a replacement
renderer. D2/ELK remains useful as a comparison backend and fallback for
complex containment.

## Bottom Line

For an open, controllable, Go-based network-diagram tool:

1. Continue strengthening the native site-aware layout mode.
2. Use D2/ELK as an optional helper and backend.
3. Do not let ELK define the visual quality of the product.

This preserves structured containment where it helps while keeping
network-specific polish under netdiag's control.
