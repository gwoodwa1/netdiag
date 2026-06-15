package svg

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/model"
)

func renderTitle(out *bytes.Buffer, doc *model.Diagram, width float64) {
	fill := "#0f172a"
	if doc.Theme.Name == "premium" {
		fill = "url(#titleGradient)"
	}
	fmt.Fprintf(out, `<rect x="0" y="0" width="%.0f" height="112" fill="%s"/>`, width, fill)
	fmt.Fprintf(out, `<text x="70" y="56" fill="#f8fafc" font-family="Inter,Segoe UI,sans-serif" font-size="30" font-weight="700">%s</text>`, escape(doc.Theme.Title))
	fmt.Fprintf(out, `<text x="70" y="84" fill="#94a3b8" font-family="Inter,Segoe UI,sans-serif" font-size="15">%s</text>`, escape(doc.Theme.Subtitle))
	fmt.Fprintf(out, `<text x="%.0f" y="66" text-anchor="end" fill="#38bdf8" font-family="ui-monospace,SFMono-Regular,monospace" font-size="14">%s</text>`, width-70, escape(strings.ToUpper(doc.Theme.Badge)))
}

func renderSiteBackgrounds(out *bytes.Buffer, groups []placedGroup, premium bool) {
	out.WriteString(`<g id="site-backgrounds">`)
	for _, group := range groups {
		b := group.Box
		if group.Depth == 0 {
			fmt.Fprintf(out, `<g id="group-%s" class="site site-%s" data-netdiag-kind="group">`, escapeID(group.ID), escapeID(group.ID))
			fill := "#eff6ff"
			if premium {
				fill = "url(#siteGradient)"
			}
			fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="24" fill="%s" stroke="#93c5fd" stroke-width="2"/>`, b.X, b.Y, b.W, b.H, fill)
			fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="%.1f" height="58" rx="24" fill="#dbeafe"/>`, b.X, b.Y, b.W)
			fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#1e3a8a" font-family="Inter,Segoe UI,sans-serif" font-size="16" font-weight="800">%s</text>`, b.X+26, b.Y+35, escape(group.Label))
			fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="end" fill="#64748b" font-family="ui-monospace,SFMono-Regular,monospace" font-size="10" font-weight="700">%s</text>`, b.X+b.W-24, b.Y+34, escape(strings.ToUpper(group.Kind)))
			out.WriteString(`</g>`)
			continue
		}
		fmt.Fprintf(out, `<g id="group-%s" class="site-subgroup site-subgroup-%s" data-netdiag-kind="group">`, escapeID(group.ID), escapeID(group.ID))
		fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="14" fill="#ffffff" fill-opacity=".28" stroke="#94a3b8" stroke-dasharray="7 6"/>`, b.X, b.Y, b.W, b.H)
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#475569" font-family="ui-monospace,SFMono-Regular,monospace" font-size="10" font-weight="750">%s</text>`, b.X+14, b.Y+18, escape(strings.ToUpper(group.Label)))
		out.WriteString(`</g>`)
	}
	out.WriteString(`</g>`)
}

func renderRowBackgrounds(out *bytes.Buffer, roles []string, width float64, premium bool) {
	for row, role := range roles {
		y := headerHeight + float64(row)*rowHeight + rowInset
		opacity := ""
		if premium {
			opacity = ` fill-opacity=".88"`
		}
		fmt.Fprintf(out, `<rect x="42" y="%.1f" width="%.1f" height="%.1f" rx="20" fill="%s"%s stroke="#e2e8f0"/>`, y, width-84, rowBandHeight, roleFill(role), opacity)
	}
}

func renderRowHeadings(out *bytes.Buffer, roles []string, byRole map[string][]string) {
	out.WriteString(`<g id="row-headings">`)
	for row, role := range roles {
		y := headerHeight + float64(row)*rowHeight + rowInset
		fmt.Fprintf(out, `<g class="row-heading row-heading-%s">`, escapeID(role))
		fmt.Fprintf(out, `<path d="M78 %.1f H410" stroke="#cbd5e1" stroke-width="1"/>`, y+53)
		fmt.Fprintf(out, `<text x="78" y="%.1f" fill="#64748b" font-family="Inter,Segoe UI,sans-serif" font-size="12" font-weight="700" letter-spacing="1.8">%s LAYER · %d DEVICES</text>`, y+38, escape(strings.ToUpper(strings.ReplaceAll(role, "-", " "))), len(byRole[role]))
		out.WriteString(`</g>`)
	}
	out.WriteString(`</g>`)
}

func renderRingBackground(out *bytes.Buffer, doc *model.Diagram, premium bool) {
	out.WriteString(`<g id="ring-background">`)
	fill := "#eff6ff"
	if premium {
		fill = "url(#siteGradient)"
	}
	fmt.Fprintf(out, `<rect x="42" y="197" width="2316" height="950" rx="24" fill="%s" stroke="#dbeafe"/>`, fill)
	out.WriteString(`<ellipse cx="1435" cy="690" rx="700" ry="385" fill="none" stroke="#cbd5e1" stroke-width="2" stroke-dasharray="8 8"/>`)
	fmt.Fprintf(out, `<text x="1435" y="675" text-anchor="middle" fill="#334155" font-family="Inter,Segoe UI,sans-serif" font-size="23" font-weight="750">%s</text>`, escape(strings.ToUpper(doc.Theme.Badge)))
	fmt.Fprintf(out, `<text x="1435" y="705" text-anchor="middle" fill="#64748b" font-family="ui-monospace,SFMono-Regular,monospace" font-size="13">%d-NODE RESILIENT RING</text>`, len(doc.Nodes))
	out.WriteString(`<text x="78" y="235" fill="#64748b" font-family="Inter,Segoe UI,sans-serif" font-size="12" font-weight="700" letter-spacing="1.8">RING TOPOLOGY</text>`)
	out.WriteString(`<path d="M78 250 H410" stroke="#cbd5e1" stroke-width="1"/>`)
	out.WriteString(`</g>`)
}

func roleFill(role string) string {
	switch role {
	case "firewall":
		return "#fef2f2"
	case "wan-cloud", "internet":
		return "#f1f5f9"
	case "public-cloud":
		return "#fffbeb"
	case "dwdm":
		return "#f5f3ff"
	case "router", "edge-router", "core-router", "ospf-backbone", "ospf-area-10", "ospf-area-20", "isis-level-2", "isis-level-1", "route-reflector", "rr-client", "external-peer", "core-switch", "distribution-switch", "access-switch", "metro-switch", "wireless":
		return "#eff6ff"
	case "endpoint", "users":
		return "#f8fafc"
	case "super-spine":
		return "#f5f3ff"
	case "spine":
		return "#eff6ff"
	case "leaf":
		return "#ecfeff"
	case "server":
		return "#f0fdf4"
	default:
		return "#f8fafc"
	}
}
