# Example Gallery

Open the [interactive playground](playground.html) for a dependency-free
pan-and-zoom demo with inspection and group-collapse controls.

Every YAML file below renders offline with:

```sh
go run ./cmd/netdiag render examples/<name>.yaml
```

| Example | Focus |
| --- | --- |
| `01-wan-dwdm-campus.yaml` | Multiple campus switches and L3 routers over DWDM circuits with circuit IDs |
| `02-branch-mpls-wan.yaml` | Headquarters and branch LANs connected through an MPLS cloud |
| `03-campus-lan.yaml` | Core, distribution, access, wireless, and endpoint hierarchy |
| `04-internet-dmz.yaml` | Dual-ISP internet edge, red brick firewall, DMZ, and public servers |
| `05-aws-hybrid-cloud.yaml` | AWS public cloud, Direct Connect, VPN backup, and on-premises firewall |
| `06-retail-sdwan.yaml` | Retail stores connected through a cloud-managed SD-WAN |
| `07-wireless-campus.yaml` | Routed campus, access switches, wireless APs, and clients |
| `08-manufacturing-ot.yaml` | Segmented manufacturing and OT network |
| `09-dual-isp-headquarters.yaml` | Redundant ISP circuits feeding a secured headquarters LAN |
| `10-data-center-interconnect.yaml` | Data-center interconnect using protected DWDM wavelengths |
| `11-ospf-multi-area.yaml` | OSPF Area 0 backbone, ABRs, and Area 10/20 adjacencies |
| `12-isis-levels.yaml` | IS-IS Level-2 backbone with two Level-1 routing domains |
| `13-bgp-route-reflectors.yaml` | Redundant iBGP route reflectors, RR clients, and eBGP peers |
| `14-metro-ethernet-ring.yaml` | Eight-node protected Metro Ethernet ring using the ring layout |
| `15-metro-mpls-core.yaml` | Four regional metro networks connected through an MPLS core cloud |
| `16-site-aware-wan.yaml` | Native site containers, nested LAN boundaries, endpoint addresses, and obstacle-aware orthogonal routing |
| `17-themed-link-status.yaml` | Nord global theme with protocol and operational-status link rules |

## Highlights

| Site-aware WAN | Metro Ethernet ring |
| --- | --- |
| [![Site-aware WAN](../examples/16-site-aware-wan.svg)](../examples/16-site-aware-wan.svg) | [![Metro Ethernet ring](../examples/14-metro-ethernet-ring.svg)](../examples/14-metro-ethernet-ring.svg) |
| **Nord status styling** | **BGP route reflectors** |
| [![Nord themed protocol and status links](../examples/17-themed-link-status.svg)](../examples/17-themed-link-status.svg) | [![BGP route reflectors](../examples/13-bgp-route-reflectors.svg)](../examples/13-bgp-route-reflectors.svg) |
| **Internet DMZ** | **Premium national telco composition** |
| [![Internet DMZ](../examples/04-internet-dmz.svg)](../examples/04-internet-dmz.svg) | [![Premium national telco topology](../examples/templates/national-telco-premium.png)](../examples/templates/national-telco-premium.png) |

The gallery exercises router, switch, firewall, cloud, AWS/public-cloud, DWDM,
wireless, endpoint, and server SVG icons. It also demonstrates OSPF, IS-IS,
iBGP, and eBGP protocol link styles. Full circuit IDs remain in YAML and are
displayed as central link labels. The site-aware WAN also exercises native
group containment and deterministic orthogonal routing. The themed link-status
example demonstrates global palettes and status rules overriding protocol
styles.

Reusable blocks derived from these examples are composed in
`examples/templates/gallery-blocks-template.yaml`.

## Premium native theme

Set `diagram.theme: premium` to opt into the higher-fidelity native SVG style:
layered device cards, status LEDs, cable underlays, illuminated port markers,
a subtle technical grid, and refined gradients. Layout and source semantics
remain unchanged.

`examples/templates/national-telco-template.yaml` is the showcase composition:

```sh
netdiag render examples/templates/national-telco-template.yaml \
  --icons examples/custom-icons \
  -o national-telco-premium.svg
```
