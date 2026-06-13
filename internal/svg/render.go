package svg

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/gwoodwa1/netdiag/internal/icons"
	"github.com/gwoodwa1/netdiag/internal/model"
	"github.com/gwoodwa1/netdiag/internal/spec"
)

const (
	canvasWidth   = 2400.0
	diagramLeft   = 470.0
	diagramRight  = 145.0
	headerHeight  = 165.0
	rowHeight     = 320.0
	rowInset      = 32.0
	rowBandHeight = 250.0
	nodeHeight    = 82.0
	rowLinkGap    = 300.0
)

type point struct {
	X float64
	Y float64
}

type placedNode struct {
	ID   string
	Node model.Node
	Box  box
}

type box struct {
	X float64
	Y float64
	W float64
	H float64
}

type attachment struct {
	LinkIndex int
	Source    bool
	Port      string
	PeerX     float64
	PeerY     float64
	Side      string
}

type endpointGeometry struct {
	Point     point
	Side      string
	LabelLane int
}

type bundleVisual struct {
	Bundle string
	Label  string
	Tags   []string
	Color  string
	X      float64
	Y      float64
	Count  int
}

type Options struct {
	IconDir string
}

func Render(doc *model.Diagram) ([]byte, error) {
	return RenderWithOptions(doc, Options{})
}

func RenderWithOptions(doc *model.Diagram, options Options) ([]byte, error) {
	roles, byRole := groupNodes(doc)
	layout := layoutDiagram(doc, roles, byRole)
	iconPack := icons.NewPack(options.IconDir)
	premium := doc.Theme.Name == "premium"

	var out bytes.Buffer
	fmt.Fprintf(&out, `<svg class="theme-%s" xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img" shape-rendering="geometricPrecision">`, escapeID(defaultTheme(doc.Theme.Name)), layout.Width, layout.Height, layout.Width, layout.Height)
	renderDefinitions(&out, premium, doc.Theme.Name)
	if premium {
		out.WriteString(`<rect width="100%" height="100%" fill="url(#canvasGradient)"/><rect width="100%" height="100%" fill="url(#technicalGrid)"/>`)
	} else {
		out.WriteString(`<rect width="100%" height="100%" fill="#f8fafc"/>`)
	}
	renderTitle(&out, doc, layout.Width)
	if doc.Theme.Layout == "ring" {
		renderRingBackground(&out, doc, premium)
	} else if doc.Theme.Layout == "sites" || doc.Theme.Layout == "hub-spoke" {
		renderSiteBackgrounds(&out, layout.Groups, premium)
	} else {
		renderRowBackgrounds(&out, roles, layout.Width, premium)
		renderRowHeadings(&out, roles, byRole)
	}
	if err := renderLinks(&out, doc, layout.Nodes); err != nil {
		return nil, err
	}
	renderBundleLegend(&out, doc)
	renderNodes(&out, layout.Nodes, iconPack, premium)
	out.WriteString(`</svg>`)
	return out.Bytes(), nil
}

func renderDefinitions(out *bytes.Buffer, premium bool, theme string) {
	out.WriteString(`<defs><filter id="shadow" x="-20%" y="-20%" width="140%" height="150%"><feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="#0f172a" flood-opacity=".14"/></filter>`)
	if premium {
		out.WriteString(`<linearGradient id="canvasGradient" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#f8fbff"/><stop offset="1" stop-color="#eef4fa"/></linearGradient>`)
		out.WriteString(`<linearGradient id="deviceCardGradient" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ffffff"/><stop offset=".58" stop-color="#f8fafc"/><stop offset="1" stop-color="#e8eef5"/></linearGradient>`)
		out.WriteString(`<linearGradient id="siteGradient" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#eff6ff" stop-opacity=".96"/><stop offset="1" stop-color="#dbeafe" stop-opacity=".72"/></linearGradient>`)
		out.WriteString(`<linearGradient id="titleGradient" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="#0f172a"/><stop offset=".72" stop-color="#172554"/><stop offset="1" stop-color="#0c4a6e"/></linearGradient>`)
		out.WriteString(`<pattern id="technicalGrid" width="32" height="32" patternUnits="userSpaceOnUse"><path d="M32 0H0V32" fill="none" stroke="#94a3b8" stroke-width=".55" stroke-opacity=".12"/><circle cx="0" cy="0" r="1" fill="#64748b" fill-opacity=".16"/></pattern>`)
		out.WriteString(`<filter id="deviceShadow" x="-25%" y="-30%" width="150%" height="175%"><feDropShadow dx="0" dy="2" stdDeviation="2" flood-color="#0f172a" flood-opacity=".12"/><feDropShadow dx="0" dy="8" stdDeviation="10" flood-color="#0f172a" flood-opacity=".15"/></filter>`)
		out.WriteString(`<filter id="portGlow" x="-150%" y="-150%" width="400%" height="400%"><feGaussianBlur stdDeviation="2.2" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`)
		out.WriteString(`<style>.premium-link{paint-order:stroke;}.label-mask+text{paint-order:stroke fill;stroke:#fff;stroke-width:2.2px;stroke-linejoin:round}.node-title{paint-order:stroke fill;stroke:#fff;stroke-width:1.4px;stroke-linejoin:round}</style>`)
	}
	if css := themeCSS(theme); css != "" {
		fmt.Fprintf(out, `<style>%s</style>`, css)
	}
	out.WriteString(`</defs>`)
}

func groupNodes(doc *model.Diagram) ([]string, map[string][]string) {
	byRole := make(map[string][]string)
	nodesByID := make(map[string]model.Node)
	for _, node := range doc.Nodes {
		byRole[node.Role] = append(byRole[node.Role], node.ID)
		nodesByID[node.ID] = node
	}
	for role := range byRole {
		sort.SliceStable(byRole[role], func(i, j int) bool {
			leftID, rightID := byRole[role][i], byRole[role][j]
			leftOrder, rightOrder := nodesByID[leftID].Order, nodesByID[rightID].Order
			if leftOrder == 0 {
				leftOrder = int(^uint(0) >> 1)
			}
			if rightOrder == 0 {
				rightOrder = int(^uint(0) >> 1)
			}
			if leftOrder == rightOrder {
				return leftID < rightID
			}
			return leftOrder < rightOrder
		})
	}

	preferred := []string{
		"users", "internet", "public-cloud", "wan-cloud", "dwdm",
		"ospf-backbone", "ospf-area-10", "ospf-area-20",
		"isis-level-2", "isis-level-1",
		"route-reflector", "rr-client", "external-peer",
		"edge-router", "router", "core-router", "firewall", "core-switch",
		"distribution-switch", "access-switch", "wireless",
		"metro-switch",
		"super-spine", "spine", "leaf", "server", "endpoint",
	}
	var roles []string
	for _, role := range preferred {
		if len(byRole[role]) > 0 {
			roles = append(roles, role)
		}
	}
	var rest []string
	for role := range byRole {
		found := false
		for _, existing := range roles {
			found = found || role == existing
		}
		if !found {
			rest = append(rest, role)
		}
	}
	sort.Strings(rest)
	return append(roles, rest...), byRole
}

func placeNodes(doc *model.Diagram, roles []string, byRole map[string][]string) map[string]placedNode {
	if doc.Theme.Layout == "ring" {
		return placeRingNodes(doc, roles, byRole)
	}
	return placeRowNodes(doc, roles, byRole, rowLayoutWidth(doc, roles, byRole))
}

func placeRowNodes(doc *model.Diagram, roles []string, byRole map[string][]string, width float64) map[string]placedNode {
	result := make(map[string]placedNode)
	nodesByID := make(map[string]model.Node)
	for _, n := range doc.Nodes {
		nodesByID[n.ID] = n
	}
	for row, rowRole := range roles {
		ids := byRole[rowRole]
		spacing := (width - diagramLeft - diagramRight) / float64(len(ids))
		for column, id := range ids {
			width := nodeWidth(rowRole)
			x := diagramLeft + spacing*(float64(column)+0.5) - width/2
			y := headerHeight + float64(row)*rowHeight + 112
			result[id] = placedNode{ID: id, Node: nodesByID[id], Box: box{X: x, Y: y, W: width, H: nodeHeight}}
		}
	}
	return result
}

func rowLayoutWidth(doc *model.Diagram, roles []string, byRole map[string][]string) float64 {
	nodesByID := make(map[string]model.Node)
	for _, node := range doc.Nodes {
		nodesByID[node.ID] = node
	}
	width := canvasWidth
	for _, role := range roles {
		ids := byRole[role]
		if len(ids) < 2 {
			continue
		}
		maxNodeWidth := 0.0
		for _, id := range ids {
			maxNodeWidth = math.Max(maxNodeWidth, nodeWidth(nodesByID[id].Role))
		}
		required := diagramLeft + diagramRight + float64(len(ids))*(maxNodeWidth+rowLinkGap)
		width = math.Max(width, required)
	}
	return width
}

func placeRingNodes(doc *model.Diagram, roles []string, byRole map[string][]string) map[string]placedNode {
	var ids []string
	for _, role := range roles {
		ids = append(ids, byRole[role]...)
	}
	result := make(map[string]placedNode)
	nodesByID := make(map[string]model.Node)
	for _, n := range doc.Nodes {
		nodesByID[n.ID] = n
	}
	centerX, centerY := 1435.0, 690.0
	radiusX, radiusY := 700.0, 385.0
	for index, id := range ids {
		node := nodesByID[id]
		width := nodeWidth(node.Role)
		angle := -math.Pi/2 + 2*math.Pi*float64(index)/float64(len(ids))
		x := centerX + radiusX*math.Cos(angle) - width/2
		y := centerY + radiusY*math.Sin(angle) - nodeHeight/2
		result[id] = placedNode{ID: id, Node: node, Box: box{X: x, Y: y, W: width, H: nodeHeight}}
	}
	return result
}

func nodeWidth(role string) float64 {
	switch role {
	case "super-spine", "spine":
		return 380
	case "leaf", "core-switch", "distribution-switch", "access-switch", "metro-switch":
		return 300
	case "server", "router", "edge-router", "core-router", "firewall", "wan-cloud", "public-cloud", "dwdm":
		return 280
	default:
		return 240
	}
}

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

func renderLinks(out *bytes.Buffer, doc *model.Diagram, nodes map[string]placedNode) error {
	geometry, err := endpointAttachments(doc, nodes)
	if err != nil {
		return err
	}
	bundleVisuals, err := buildBundleVisuals(doc, geometry)
	if err != nil {
		return err
	}

	out.WriteString(`<g id="links">`)
	premium := doc.Theme.Name == "premium"
	for index, link := range doc.Links {
		from := link.From
		to := link.To
		startGeometry := geometry[endpointKey(index, true)]
		endGeometry := geometry[endpointKey(index, false)]
		start, end := startGeometry.Point, endGeometry.Point
		color, dash := linkStyle(link.Style)
		strokeWidth := 2.2
		if link.Bundle != "" {
			strokeWidth = 3
		}
		visual := doc.ResolveLinkStyle(link)
		if visual.Color != "" {
			color = visual.Color
		}
		if visual.Pattern != "" {
			dash = linkPattern(visual.Pattern)
		}
		if visual.Width > 0 {
			strokeWidth = visual.Width
		}
		color = escape(color)

		useOrthogonalRoute := doc.Theme.Layout == "sites" || doc.Theme.Layout == "hub-spoke" || doc.Theme.LinkStyle == "orthogonal"
		route := directRoute(start, end, startGeometry.Side, endGeometry.Side, doc.Theme.LinkStyle)
		if useOrthogonalRoute {
			route = orthogonalRoute(start, end, startGeometry.Side, endGeometry.Side, nodes, index)
		}
		path := route.Path
		if link.Bundle != "" {
			visual := bundleVisuals[link.Bundle]
			path = pathDataVia(start, point{X: visual.X, Y: visual.Y}, end, doc.Theme.LinkStyle)
		}
		fmt.Fprintf(out, `<g id="link-%d" data-netdiag-kind="link">`, index+1)
		if premium {
			fmt.Fprintf(out, `<path class="link-underlay" d="%s" fill="none" stroke="#ffffff" stroke-width="%.1f" stroke-linecap="round" stroke-linejoin="round" opacity=".88"/>`, path, strokeWidth+3.8)
		}
		className := ""
		if premium {
			className = ` class="premium-link"`
		}
		fmt.Fprintf(out, `<path%s d="%s" fill="none" stroke="%s" stroke-width="%.1f" stroke-linecap="round" stroke-linejoin="round" %s opacity=".86"/>`, className, path, color, strokeWidth, dash)
		renderPortMarker(out, start, color, premium)
		renderPortMarker(out, end, color, premium)
		if doc.Theme.InterfaceLabels != "none" {
			renderEndpointLabel(out, start, link.SourceLabel(), startGeometry.Side, startGeometry.LabelLane, doc.Theme.InterfaceLabelStyle)
			renderEndpointLabel(out, end, link.TargetLabel(), endGeometry.Side, endGeometry.LabelLane, doc.Theme.InterfaceLabelStyle)
		}
		renderEndpointAddress(out, start, from.Address, startGeometry.Side, startGeometry.LabelLane, color)
		renderEndpointAddress(out, end, to.Address, endGeometry.Side, endGeometry.LabelLane, color)
		if link.MiddleLabel() != "" && link.Bundle == "" {
			if useOrthogonalRoute {
				renderRouteLabel(out, route.Label, route.LabelHorizontal, link.MiddleLabel(), color, index)
			} else {
				renderCenterLabel(out, start, end, startGeometry.Side, endGeometry.Side, link.MiddleLabel(), color, index)
			}
		}
		if link.Bundle == "" {
			if useOrthogonalRoute {
				renderRouteTags(out, route.Label, link.Tags(), color, index)
			} else {
				renderLinkTags(out, start, end, link.Tags(), color, index)
			}
		}
		out.WriteString(`</g>`)
	}
	bundleIDs := make([]string, 0, len(bundleVisuals))
	for id := range bundleVisuals {
		bundleIDs = append(bundleIDs, id)
	}
	sort.Strings(bundleIDs)
	for _, id := range bundleIDs {
		renderBundleMarker(out, bundleVisuals[id].X, bundleVisuals[id].Y, bundleVisuals[id])
	}
	out.WriteString(`</g>`)
	return nil
}

func buildBundleVisuals(doc *model.Diagram, geometry map[string]endpointGeometry) (map[string]*bundleVisual, error) {
	visuals := make(map[string]*bundleVisual)
	for index, link := range doc.Links {
		if link.Bundle == "" {
			continue
		}
		start := geometry[endpointKey(index, true)].Point
		end := geometry[endpointKey(index, false)].Point
		color, _ := linkStyle(link.Style)
		if custom := doc.ResolveLinkStyle(link); custom.Color != "" {
			color = custom.Color
		}
		color = escape(color)
		visual := visuals[link.Bundle]
		if visual == nil {
			visual = &bundleVisual{
				Bundle: link.Bundle,
				Label:  link.MiddleLabel(),
				Tags:   link.Tags(),
				Color:  color,
			}
			visuals[link.Bundle] = visual
		}
		visual.X += (start.X + end.X) / 2
		visual.Y += (start.Y + end.Y) / 2
		visual.Count++
	}
	for _, visual := range visuals {
		visual.X /= float64(visual.Count)
		visual.Y /= float64(visual.Count)
	}
	return visuals, nil
}

func computeDefaultSides(fromCenter, toCenter point, layout string) (string, string) {
	fromSide, toSide := "bottom", "top"
	if layout == "ring" && math.Abs(toCenter.X-fromCenter.X) > math.Abs(toCenter.Y-fromCenter.Y) {
		fromSide, toSide = "right", "left"
		if toCenter.X < fromCenter.X {
			fromSide, toSide = "left", "right"
		}
	} else if math.Abs(toCenter.Y-fromCenter.Y) < 1 {
		fromSide, toSide = "right", "left"
		if toCenter.X < fromCenter.X {
			fromSide, toSide = "left", "right"
		}
	} else if toCenter.Y < fromCenter.Y {
		fromSide, toSide = "top", "bottom"
	}
	return fromSide, toSide
}

func endpointAttachments(doc *model.Diagram, nodes map[string]placedNode) (map[string]endpointGeometry, error) {
	attachments := make(map[string][]attachment)
	rootGroups := nodeRootGroups(doc)
	for index, link := range doc.Links {
		from := link.From
		to := link.To
		fromNode, toNode := nodes[from.Node], nodes[to.Node]
		fromCenter := point{X: fromNode.Box.X + fromNode.Box.W/2, Y: fromNode.Box.Y + fromNode.Box.H/2}
		toCenter := point{X: toNode.Box.X + toNode.Box.W/2, Y: toNode.Box.Y + toNode.Box.H/2}

		fromSide := from.Side
		toSide := to.Side
		defaultFromSide, defaultToSide := computeDefaultSides(fromCenter, toCenter, doc.Theme.Layout)
		if (doc.Theme.Layout == "sites" || doc.Theme.Layout == "hub-spoke") && rootGroups[from.Node] != rootGroups[to.Node] {
			deltaX := math.Abs(toCenter.X - fromCenter.X)
			deltaY := math.Abs(toCenter.Y - fromCenter.Y)
			if deltaX >= deltaY {
				defaultFromSide, defaultToSide = "right", "left"
				if toCenter.X < fromCenter.X {
					defaultFromSide, defaultToSide = "left", "right"
				}
			}
		}
		if fromSide == "" {
			fromSide = defaultFromSide
		}
		if toSide == "" {
			toSide = defaultToSide
		}
		attachments[from.Node] = append(attachments[from.Node], attachment{index, true, from.Port, toCenter.X, toCenter.Y, fromSide})
		attachments[to.Node] = append(attachments[to.Node], attachment{index, false, to.Port, fromCenter.X, fromCenter.Y, toSide})
	}

	result := make(map[string]endpointGeometry)
	for nodeID, items := range attachments {
		node := nodes[nodeID]
		bySide := make(map[string][]attachment)
		for _, item := range items {
			bySide[item.Side] = append(bySide[item.Side], item)
		}
		for side, sideItems := range bySide {
			sort.SliceStable(sideItems, func(i, j int) bool {
				if side == "left" || side == "right" {
					if sideItems[i].PeerY == sideItems[j].PeerY {
						return sideItems[i].Port < sideItems[j].Port
					}
					return sideItems[i].PeerY < sideItems[j].PeerY
				}
				if sideItems[i].PeerX == sideItems[j].PeerX {
					return sideItems[i].Port < sideItems[j].Port
				}
				return sideItems[i].PeerX < sideItems[j].PeerX
			})
			for slot, item := range sideItems {
				x := node.Box.X
				y := node.Box.Y + 18
				if side == "top" || side == "bottom" {
					x += 22
					if len(sideItems) > 1 {
						x += (node.Box.W - 44) * float64(slot) / float64(len(sideItems)-1)
					} else {
						x = node.Box.X + node.Box.W/2
					}
					y = node.Box.Y
					if side == "bottom" {
						y += node.Box.H
					}
				} else {
					if len(sideItems) > 1 {
						y += (node.Box.H - 36) * float64(slot) / float64(len(sideItems)-1)
					} else {
						y = node.Box.Y + node.Box.H/2
					}
					if side == "right" {
						x += node.Box.W
					}
				}
				lane := 0
				if (side == "left" || side == "right") && len(sideItems) > 1 {
					lane = slot
				}
				result[endpointKey(item.LinkIndex, item.Source)] = endpointGeometry{Point: point{x, y}, Side: side, LabelLane: lane}
			}
		}
	}
	return result, nil
}

func endpointKey(index int, source bool) string {
	return fmt.Sprintf("%d:%t", index, source)
}

func pathData(start, end point, startSide, endSide, style string) string {
	horizontal := (startSide == "left" || startSide == "right") && (endSide == "left" || endSide == "right")
	if style == "orthogonal" {
		if horizontal {
			mid := (start.X + end.X) / 2
			return fmt.Sprintf("M %.1f %.1f H %.1f V %.1f H %.1f", start.X, start.Y, mid, end.Y, end.X)
		}
		mid := (start.Y + end.Y) / 2
		return fmt.Sprintf("M %.1f %.1f V %.1f H %.1f V %.1f", start.X, start.Y, mid, end.X, end.Y)
	}
	if style == "clean" {
		if horizontal {
			direction := 1.0
			if end.X < start.X {
				direction = -1
			}
			stub := math.Min(54, math.Abs(end.X-start.X)*0.22)
			return fmt.Sprintf("M %.1f %.1f H %.1f L %.1f %.1f H %.1f", start.X, start.Y, start.X+direction*stub, end.X-direction*stub, end.Y, end.X)
		}
		direction := 1.0
		if end.Y < start.Y {
			direction = -1
		}
		stub := math.Min(54, math.Abs(end.Y-start.Y)*0.22)
		startStubY := start.Y + direction*stub
		endStubY := end.Y - direction*stub
		return fmt.Sprintf("M %.1f %.1f V %.1f L %.1f %.1f V %.1f", start.X, start.Y, startStubY, end.X, endStubY, end.Y)
	}
	return fmt.Sprintf("M %.1f %.1f L %.1f %.1f", start.X, start.Y, end.X, end.Y)
}

func pathDataVia(start, via, end point, style string) string {
	if style == "orthogonal" {
		return fmt.Sprintf("M %.1f %.1f H %.1f V %.1f H %.1f V %.1f", start.X, start.Y, via.X, via.Y, end.X, end.Y)
	}
	if style == "clean" {
		direction := 1.0
		if end.Y < start.Y {
			direction = -1
		}
		stub := math.Min(54, math.Abs(end.Y-start.Y)*0.18)
		return fmt.Sprintf(
			"M %.1f %.1f V %.1f L %.1f %.1f L %.1f %.1f V %.1f",
			start.X, start.Y,
			start.Y+direction*stub,
			via.X, via.Y,
			end.X, end.Y-direction*stub,
			end.Y,
		)
	}
	return fmt.Sprintf("M %.1f %.1f L %.1f %.1f L %.1f %.1f", start.X, start.Y, via.X, via.Y, end.X, end.Y)
}

func renderEndpointLabel(out *bytes.Buffer, endpoint point, label, side string, lane int, style model.InterfaceLabelStyle) {
	x := endpoint.X
	y := endpoint.Y - 12
	if side == "bottom" {
		y = endpoint.Y + 25
	} else if side == "left" {
		x = endpoint.X - horizontalLabelOffset
		y = endpoint.Y - 12
	} else if side == "right" {
		x = endpoint.X + horizontalLabelOffset
		y = endpoint.Y - 12
	}
	renderInterfaceLabel(out, x, y, label, style)
}

func renderEndpointAddress(out *bytes.Buffer, endpoint point, address, side string, lane int, color string) {
	if address == "" {
		return
	}
	x := endpoint.X
	y := endpoint.Y - 33
	if side == "bottom" {
		y = endpoint.Y + 46
	} else if side == "left" {
		x = endpoint.X - horizontalLabelOffset
		y = endpoint.Y + 17
	} else if side == "right" {
		x = endpoint.X + horizontalLabelOffset
		y = endpoint.Y + 17
	}
	renderLabel(out, x, y, address, color, "middle", 10, false)
}

const horizontalLabelOffset = 55.0

func renderInterfaceLabel(out *bytes.Buffer, x, y float64, label string, style model.InterfaceLabelStyle) {
	fill := style.Fill
	if fill == "" {
		fill = "#ffffff"
	}
	color := style.Color
	if color == "" {
		color = "#334155"
	}
	border := style.Border
	if border == "" {
		border = "#cbd5e1"
	}
	radius := style.Radius
	paddingX := style.PaddingX
	paddingY := style.PaddingY
	const size = 11
	width := math.Max(38, float64(len([]rune(label)))*size*0.61+paddingX*2)
	height := size + paddingY*2
	fmt.Fprintf(out, `<rect class="interface-label-badge" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" fill="%s" stroke="%s" stroke-width="1"/>`, x-width/2, y-size-paddingY, width, height, radius, escape(fill), escape(border))
	fmt.Fprintf(out, `<text class="interface-label-text" x="%.1f" y="%.1f" text-anchor="middle" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="%d" font-weight="650">%s</text>`, x, y, escape(color), size, escape(label))
}

func renderPortMarker(out *bytes.Buffer, endpoint point, color string, premium bool) {
	filter := ""
	if premium {
		filter = ` filter="url(#portGlow)"`
	}
	fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="3.2" fill="#ffffff" stroke="%s" stroke-width="2"%s/>`, endpoint.X, endpoint.Y, color, filter)
}

func renderCenterLabel(out *bytes.Buffer, start, end point, startSide, endSide, label, color string, index int) {
	positions := []float64{0.42, 0.5, 0.58}
	t := positions[index%len(positions)]
	x := start.X + (end.X-start.X)*t
	y := start.Y + (end.Y-start.Y)*t - 9
	horizontal := (startSide == "left" || startSide == "right") && (endSide == "left" || endSide == "right")
	if horizontal {
		y = start.Y + (end.Y-start.Y)*t + 31
	}
	renderLabel(out, x, y, label, color, "middle", 11, false)
}

func renderRouteLabel(out *bytes.Buffer, location point, horizontal bool, label, color string, index int) {
	offsets := []float64{-18, -36, 18}
	offset := offsets[index%len(offsets)]
	if horizontal {
		location.Y += offset
	} else {
		location.X += offset
	}
	renderLabel(out, location.X, location.Y, label, color, "middle", 11, false)
}

func renderLinkTags(out *bytes.Buffer, start, end point, tags []string, color string, index int) {
	if len(tags) == 0 {
		return
	}
	positions := []float64{0.42, 0.5, 0.58}
	t := positions[index%len(positions)]
	x := start.X + (end.X-start.X)*t
	y := start.Y + (end.Y-start.Y)*t + 14

	widths := make([]float64, len(tags))
	totalWidth := 0.0
	for i, tag := range tags {
		widths[i] = math.Max(35, float64(len([]rune(tag)))*6.1+14)
		totalWidth += widths[i]
	}
	totalWidth += float64(len(tags)-1) * 4
	cursor := x - totalWidth/2
	for i, tag := range tags {
		width := widths[i]
		fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="%.1f" height="18" rx="9" fill="%s" fill-opacity=".12" stroke="%s" stroke-opacity=".42"/>`, cursor, y-13, width, color, color)
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="middle" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="9" font-weight="700">%s</text>`, cursor+width/2, y, color, escape(tag))
		cursor += width + 4
	}
}

func renderRouteTags(out *bytes.Buffer, location point, tags []string, color string, index int) {
	if len(tags) == 0 {
		return
	}
	renderLinkTags(out, location, location, tags, color, index)
}

func renderBundleMarker(out *bytes.Buffer, x, y float64, visual *bundleVisual) {
	fmt.Fprintf(out, `<g class="bundle-marker" filter="url(#shadow)">`)
	fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="22" fill="#ffffff" stroke="%s" stroke-width="3"/>`, x, y, visual.Color)
	fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="14" fill="%s" fill-opacity=".13"/>`, x, y, visual.Color)
	fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="middle" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="10" font-weight="800">%s</text>`, x, y+3.5, visual.Color, escape(spec.DisplayPort(visual.Bundle)))
	out.WriteString(`</g>`)
}

func renderBundleLegend(out *bytes.Buffer, doc *model.Diagram) {
	type legendBundle struct {
		Name  string
		Label string
		Tags  []string
		Count int
	}
	bundles := make(map[string]*legendBundle)
	for _, link := range doc.Links {
		if link.Bundle == "" {
			continue
		}
		entry := bundles[link.Bundle]
		if entry == nil {
			entry = &legendBundle{Name: link.Bundle, Label: link.MiddleLabel(), Tags: link.Tags()}
			bundles[link.Bundle] = entry
		}
		entry.Count++
	}
	if len(bundles) == 0 {
		return
	}
	ids := make([]string, 0, len(bundles))
	for id := range bundles {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	const (
		x         = 78.0
		y         = 1010.0
		width     = 332.0
		rowHeight = 47.0
	)
	out.WriteString(`<g id="bundle-legend">`)
	fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#475569" font-family="Inter,Segoe UI,sans-serif" font-size="11" font-weight="800" letter-spacing="1.5">BUNDLE DETAILS</text>`, x, y)
	fmt.Fprintf(out, `<path d="M%.1f %.1f H%.1f" stroke="#cbd5e1"/>`, x, y+12, x+width)
	for i, id := range ids {
		entry := bundles[id]
		rowY := y + 34 + float64(i)*rowHeight
		detail := fmt.Sprintf("%dx%s", entry.Count, entry.Label)
		if entry.Label == "" {
			detail = fmt.Sprintf("%d links", entry.Count)
		}
		tags := make([]string, 0, len(entry.Tags))
		vlan := ""
		for _, tag := range entry.Tags {
			if tag == entry.Name {
				continue
			}
			if strings.HasPrefix(tag, "VLAN ") {
				vlan = tag
				continue
			}
			tags = append(tags, tag)
		}
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#15803d" font-family="ui-monospace,SFMono-Regular,monospace" font-size="11" font-weight="800">%s</text>`, x, rowY, escape(spec.DisplayPort(entry.Name)))
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#334155" font-family="ui-monospace,SFMono-Regular,monospace" font-size="8.5" font-weight="650">%s · %s</text>`, x+48, rowY, escape(detail), escape(strings.Join(tags, " · ")))
		if vlan != "" {
			fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#64748b" font-family="ui-monospace,SFMono-Regular,monospace" font-size="8.5" font-weight="600">%s</text>`, x+48, rowY+15, escape(vlan))
		}
	}
	out.WriteString(`</g>`)
}

func renderLabel(out *bytes.Buffer, x, y float64, label, color, anchor string, size int, strong bool) {
	width := math.Max(38, float64(len([]rune(label)))*float64(size)*0.61+18)
	weight := 500
	if strong {
		weight = 650
	}
	fmt.Fprintf(out, `<rect class="label-mask" x="%.1f" y="%.1f" width="%.1f" height="%d" rx="5" fill="#f8fafc" stroke="#f8fafc" stroke-width="4"/>`, x-width/2, y-float64(size)-5, width, size+12)
	fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="%s" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="%d" font-weight="%d">%s</text>`, x, y, anchor, color, size, weight, escape(label))
}

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

func linkStyle(style string) (string, string) {
	switch style {
	case "ospf":
		return "#2563eb", ""
	case "isis":
		return "#7c3aed", ""
	case "ibgp":
		return "#0891b2", `stroke-dasharray="8 4"`
	case "ebgp":
		return "#ea580c", `stroke-dasharray="4 4"`
	case "ring":
		return "#0f766e", ""
	case "wan":
		return "#7c3aed", ""
	case "dwdm":
		return "#c026d3", `stroke-dasharray="10 5"`
	case "internet":
		return "#64748b", `stroke-dasharray="6 5"`
	case "security":
		return "#dc2626", ""
	case "peer":
		return "#7c3aed", `stroke-dasharray="7 6"`
	case "server":
		return "#16a34a", ""
	default:
		return "#334155", ""
	}
}

func linkPattern(pattern string) string {
	switch pattern {
	case "dashed":
		return `stroke-dasharray="8 5"`
	case "dotted":
		return `stroke-dasharray="2 5"`
	default:
		return ""
	}
}

func defaultTheme(theme string) string {
	if theme == "" {
		return "light"
	}
	return theme
}

func themeCSS(theme string) string {
	switch theme {
	case "nord":
		return `.theme-nord [fill="#f8fafc"],.theme-nord [fill="#ffffff"]{fill:#3b4252!important}.theme-nord [fill="#eff6ff"],.theme-nord [fill="#f1f5f9"],.theme-nord [fill="#f5f3ff"],.theme-nord [fill="#ecfeff"],.theme-nord [fill="#f0fdf4"]{fill:#434c5e!important}.theme-nord [fill="#0f172a"]{fill:#2e3440!important}.theme-nord [fill="#334155"],.theme-nord [fill="#475569"],.theme-nord [fill="#64748b"]{fill:#d8dee9!important}.theme-nord [stroke="#e2e8f0"],.theme-nord [stroke="#cbd5e1"],.theme-nord [stroke="#dbeafe"]{stroke:#4c566a!important}.theme-nord text{fill:#eceff4}.theme-nord .label-mask{fill:#3b4252!important;stroke:#3b4252!important}`
	case "dracula":
		return `.theme-dracula [fill="#f8fafc"],.theme-dracula [fill="#ffffff"]{fill:#282a36!important}.theme-dracula [fill="#eff6ff"],.theme-dracula [fill="#f1f5f9"],.theme-dracula [fill="#f5f3ff"],.theme-dracula [fill="#ecfeff"],.theme-dracula [fill="#f0fdf4"]{fill:#343746!important}.theme-dracula [fill="#0f172a"]{fill:#191a21!important}.theme-dracula [fill="#334155"],.theme-dracula [fill="#475569"],.theme-dracula [fill="#64748b"]{fill:#f8f8f2!important}.theme-dracula [stroke="#e2e8f0"],.theme-dracula [stroke="#cbd5e1"],.theme-dracula [stroke="#dbeafe"]{stroke:#6272a4!important}.theme-dracula text{fill:#f8f8f2}.theme-dracula .label-mask{fill:#282a36!important;stroke:#282a36!important}`
	default:
		return ""
	}
}

func escape(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}

func escapeID(value string) string {
	var out strings.Builder
	dash := false
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.' {
			out.WriteRune(char)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		result = "item"
	}
	if result != value {
		sum := sha256.Sum256([]byte(value))
		result = fmt.Sprintf("%s-%x", result, sum[:4])
	}
	return result
}
