package svg

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/icons"
)

func renderNodes(out *bytes.Buffer, nodes map[string]placedNode, iconPack *icons.Pack, premium bool) {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out.WriteString(`<g id="nodes">`)
	for _, id := range ids {
		item := nodes[id]
		b := item.Box
		color := item.Node.Color
		if color == "" {
			color = roleColor(item.Node.Role)
		}
		color = escape(color)
		filter := "shadow"
		fill := roleNodeFill(item.Node.Role)
		if premium {
			filter = "deviceShadow"
			fill = "url(#deviceCardGradient)"
		}
		fmt.Fprintf(out, `<g id="%s" data-netdiag-kind="node" filter="url(#%s)">`, escapeID(id), filter)
		fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="14" fill="%s" stroke="%s" stroke-width="2"/>`, b.X, b.Y, b.W, b.H, fill, color)
		fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="8" height="%.1f" rx="4" fill="%s"/>`, b.X, b.Y, b.H, color)
		if premium {
			fmt.Fprintf(out, `<path d="M%.1f %.1f H%.1f" stroke="#ffffff" stroke-width="2" stroke-linecap="round" opacity=".92"/>`, b.X+17, b.Y+7, b.X+b.W-16)
			fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="3" fill="#22c55e" stroke="#ffffff" stroke-width="1.2"/><circle cx="%.1f" cy="%.1f" r="2.2" fill="#38bdf8" stroke="#ffffff" stroke-width="1"/>`, b.X+b.W-17, b.Y+17, b.X+b.W-27, b.Y+17)
		}
		icon := item.Node.Icon
		if icon == "" {
			icon = item.Node.Role
		}
		renderDeviceIcon(out, b.X+40, b.Y+b.H/2, color, icon, item.Node.IconLabel, id, iconPack)
		fmt.Fprintf(out, `<text class="node-title" x="%.1f" y="%.1f" fill="#0f172a" font-family="Inter,Segoe UI,Arial,sans-serif" font-size="15" font-weight="700">%s</text>`, b.X+78, b.Y+34, escape(item.Node.Label))
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#64748b" font-family="ui-monospace,SFMono-Regular,monospace" font-size="11">%s</text>`, b.X+78, b.Y+55, escape(strings.ToUpper(item.Node.Role)))
		out.WriteString(`</g>`)
	}
	out.WriteString(`</g>`)
}

func renderDeviceIcon(out *bytes.Buffer, x, y float64, color, role, label, instanceID string, iconPack *icons.Pack) {
	canonical := role
	if icon, ok := icons.Resolve(role); ok {
		canonical = icon.ID
	}
	if custom, ok := resolveCustomIcon(iconPack, role, canonical); ok {
		fmt.Fprintf(out, `<g class="device-icon device-icon-%s custom-device-icon" transform="translate(%.1f %.1f)">`, escapeID(role), x, y)
		fmt.Fprintf(out, `<svg x="-29" y="-24" width="58" height="48" viewBox="%s" preserveAspectRatio="xMidYMid meet">`, escape(custom.ViewBox))
		out.WriteString(strings.ReplaceAll(custom.Content, custom.Prefix, "netdiag-icon-"+escapeID(instanceID)+"-"))
		out.WriteString(`</svg>`)
		renderIconLabel(out, label)
		out.WriteString(`</g>`)
		return
	}
	iconColor := color
	fmt.Fprintf(out, `<g class="device-icon device-icon-%s" transform="translate(%.1f %.1f)" stroke="%s" fill="none" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">`, escapeID(role), x, y, iconColor)
	switch canonical {
	case "spine":
		renderSpineIcon(out, iconColor)
	case "leaf":
		renderLeafIcon(out, iconColor)
	case "router":
		renderRouterIcon(out, iconColor)
	case "firewall":
		renderFirewallIcon(out)
	case "cloud":
		renderCloudIcon(out, "#64748b")
	case "public-cloud":
		renderCloudIcon(out, "#f59e0b")
	case "dwdm":
		renderDWDMIcon(out)
	case "wireless":
		renderWirelessIcon(out, iconColor)
	case "endpoint":
		renderEndpointIcon(out, iconColor)
	case "server":
		renderServerIcon(out, color)
	default:
		renderGenericSwitchIcon(out, color)
	}
	renderIconLabel(out, label)
	out.WriteString(`</g>`)
}

func renderIconLabel(out *bytes.Buffer, label string) {
	label = strings.ToUpper(strings.TrimSpace(label))
	if label == "" {
		return
	}
	width := math.Max(15, float64(len([]rune(label)))*6+8)
	fmt.Fprintf(out, `<g class="device-icon-label"><rect x="%.1f" y="9" width="%.1f" height="13" rx="5" fill="#ffffff" stroke="#0f172a" stroke-width="1.2"/>`, -width/2, width)
	fmt.Fprintf(out, `<text x="0" y="18.5" text-anchor="middle" fill="#0f172a" stroke="none" font-family="ui-monospace,SFMono-Regular,monospace" font-size="8.5" font-weight="800">%s</text></g>`, escape(label))
}

func resolveCustomIcon(pack *icons.Pack, requested, canonical string) (icons.SVG, bool) {
	if icon, ok := pack.Resolve(requested); ok {
		return icon, true
	}
	if canonical != requested {
		return pack.Resolve(canonical)
	}
	return icons.SVG{}, false
}

func renderRouterIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<ellipse cx="0" cy="0" rx="27" ry="18" fill="%s" fill-opacity=".12"/>`, color)
	out.WriteString(`<ellipse cx="0" cy="0" rx="27" ry="18"/>`)
	out.WriteString(`<path d="M-14 0h28M8-6l6 6-6 6M14 0H-14M-8-6l-6 6 6 6"/>`)
	out.WriteString(`<path d="M0-13v26M-6-7l6-6 6 6M-6 7l6 6 6-6"/>`)
}

func renderFirewallIcon(out *bytes.Buffer) {
	const red = "#dc2626"
	fmt.Fprintf(out, `<g stroke="%s">`, red)
	fmt.Fprintf(out, `<rect x="-27" y="-21" width="54" height="42" rx="4" fill="%s" fill-opacity=".10"/>`, red)
	out.WriteString(`<path d="M-27-7h54M-27 7h54M-16-21v14M7-21v14M-5-7V7M18-7V7M-16 7v14M7 7v14"/>`)
	out.WriteString(`</g>`)
}

func renderCloudIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<path d="M-24 13c-8 0-10-12-3-16 1-11 15-15 22-8 8-9 25-4 25 8 10 1 11 16 1 16Z" fill="%s" fill-opacity=".13" stroke="%s"/>`, color, color)
	out.WriteString(`<path d="M-18 4h36M-10-3h20"/>`)
}

func renderDWDMIcon(out *bytes.Buffer) {
	const purple = "#7c3aed"
	fmt.Fprintf(out, `<rect x="-27" y="-18" width="54" height="36" rx="6" fill="%s" fill-opacity=".10" stroke="%s"/>`, purple, purple)
	fmt.Fprintf(out, `<path d="M-21-9c8 0 8 7 16 7s8-7 16-7 8 7 16 7M-21 2c8 0 8 7 16 7s8-7 16-7 8 7 16 7" stroke="%s"/>`, purple)
}

func renderWirelessIcon(out *bytes.Buffer, color string) {
	out.WriteString(`<circle cx="0" cy="12" r="3"/>`)
	out.WriteString(`<path d="M-9 5a13 13 0 0 1 18 0M-16-2a23 23 0 0 1 32 0M-23-9a33 33 0 0 1 46 0"/>`)
}

func renderEndpointIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<rect x="-25" y="-18" width="50" height="32" rx="4" fill="%s" fill-opacity=".08"/>`, color)
	out.WriteString(`<rect x="-25" y="-18" width="50" height="32" rx="4"/><path d="M-12 21h24M0 14v7"/>`)
}

func renderSpineIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<path d="M-27 -10 -7 -23 27 -12 7 1Z" fill="%s" fill-opacity=".16"/>`, color)
	fmt.Fprintf(out, `<path d="M-27 -10 7 1 27 -12v13L7 15-27 3Z" fill="%s" fill-opacity=".08"/>`, color)
	out.WriteString(`<path d="M-27 -10 -7 -23 27 -12 7 1Z"/>`)
	out.WriteString(`<path d="M-27 -10v13L7 15 27 1v-13M7 1v14"/>`)
	out.WriteString(`<path d="M-11 -12 8 -6M-2 -17 17 -11M-15 -5 4 1"/>`)
	out.WriteString(`<path d="m6 -9 4 3-6 1M5 -14l4-2 1 5M-7 -7l-4-3 6-1M-6 -3l-4 2-1-5"/>`)
	out.WriteString(`<circle cx="-19" cy="-1" r="1.5"/><circle cx="-14" cy=".7" r="1.5"/><circle cx="-9" cy="2.4" r="1.5"/>`)
}

func renderLeafIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<path d="M-28 -9 -9 -20 28 -9 9 2Z" fill="%s" fill-opacity=".14"/>`, color)
	fmt.Fprintf(out, `<path d="M-28 -9 9 2 28 -9v12L9 15-28 4Z" fill="%s" fill-opacity=".07"/>`, color)
	out.WriteString(`<path d="M-28 -9 -9 -20 28 -9 9 2Z"/>`)
	out.WriteString(`<path d="M-28 -9v13L9 15 28 3v-12M9 2v13"/>`)
	out.WriteString(`<path d="M-19 -4 3 3M-19 1 3 8"/>`)
	out.WriteString(`<rect x="-15.5" y="-4.5" width="3.5" height="2.7" rx=".5"/><rect x="-10" y="-2.8" width="3.5" height="2.7" rx=".5"/><rect x="-4.5" y="-1.1" width="3.5" height="2.7" rx=".5"/><rect x="1" y=".6" width="3.5" height="2.7" rx=".5"/>`)
	out.WriteString(`<circle cx="-20" cy="1.5" r="1.4"/><circle cx="-15" cy="3.2" r="1.4"/><circle cx="-10" cy="4.9" r="1.4"/>`)
	out.WriteString(`<path d="M11 -10h9M12 -6h6"/>`)
}

func renderServerIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<rect x="-17" y="-23" width="34" height="46" rx="5" fill="%s" fill-opacity=".08"/>`, color)
	out.WriteString(`<rect x="-17" y="-23" width="34" height="46" rx="5"/>`)
	out.WriteString(`<path d="M-10 -13h20M-10 -3h20M-10 7h20M-10 17h12"/>`)
	out.WriteString(`<circle cx="11" cy="17" r="2"/>`)
}

func renderGenericSwitchIcon(out *bytes.Buffer, color string) {
	fmt.Fprintf(out, `<rect x="-22" y="-15" width="44" height="30" rx="6" fill="%s" fill-opacity=".10"/>`, color)
	out.WriteString(`<rect x="-22" y="-15" width="44" height="30" rx="6"/><path d="M-14 -6h28M-14 3h28M-17 -6h.1M-17 3h.1"/>`)
}

func roleColor(role string) string {
	switch role {
	case "firewall":
		return "#dc2626"
	case "wan-cloud", "internet":
		return "#64748b"
	case "public-cloud":
		return "#f59e0b"
	case "dwdm":
		return "#7c3aed"
	case "edge-router":
		return "#d97706"
	case "core-router":
		return "#7c3aed"
	case "router", "ospf-backbone", "ospf-area-10", "ospf-area-20", "isis-level-2", "isis-level-1", "route-reflector", "rr-client", "external-peer", "core-switch", "distribution-switch", "access-switch", "metro-switch", "wireless":
		return "#0878b9"
	case "endpoint", "users":
		return "#475569"
	case "super-spine":
		return "#7c3aed"
	case "spine":
		return "#2563eb"
	case "leaf":
		return "#0891b2"
	case "server":
		return "#16a34a"
	default:
		return "#475569"
	}
}

func roleNodeFill(role string) string {
	switch role {
	case "edge-router":
		return "#fffbeb"
	case "core-router":
		return "#f5f3ff"
	default:
		return "#ffffff"
	}
}
