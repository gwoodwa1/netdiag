package svg

import (
	"bytes"
	"fmt"
	"math"
	"sort"

	"github.com/gwoodwa1/netdiag/internal/icons"
	"github.com/gwoodwa1/netdiag/internal/model"
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
	Pinned    bool
	Position  *float64
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

type renderer struct {
	out       bytes.Buffer
	iconPack  *icons.Pack
	premium   bool
	themeName string
}

func Render(doc *model.Diagram) ([]byte, error) {
	return RenderWithOptions(doc, Options{})
}

func RenderWithOptions(doc *model.Diagram, options Options) ([]byte, error) {
	roles, byRole := groupNodes(doc)
	layout := layoutDiagram(doc, roles, byRole)
	renderer := renderer{
		iconPack:  icons.NewPack(options.IconDir),
		premium:   doc.Theme.Name == "premium",
		themeName: doc.Theme.Name,
	}
	if err := renderer.renderDocument(doc, roles, byRole, layout); err != nil {
		return nil, err
	}
	return renderer.out.Bytes(), nil
}

func (renderer *renderer) renderDocument(doc *model.Diagram, roles []string, byRole map[string][]string, layout layoutResult) error {
	fmt.Fprintf(&renderer.out, `<svg class="theme-%s" xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img" shape-rendering="geometricPrecision">`, escapeID(defaultTheme(renderer.themeName)), layout.Width, layout.Height, layout.Width, layout.Height)
	renderDefinitions(&renderer.out, renderer.premium, renderer.themeName)
	if renderer.premium {
		renderer.out.WriteString(`<rect width="100%" height="100%" fill="url(#canvasGradient)"/><rect width="100%" height="100%" fill="url(#technicalGrid)"/>`)
	} else {
		renderer.out.WriteString(`<rect width="100%" height="100%" fill="#f8fafc"/>`)
	}
	renderTitle(&renderer.out, doc, layout.Width)
	if doc.Theme.Layout == "ring" {
		renderRingBackground(&renderer.out, doc, renderer.premium)
	} else if doc.Theme.Layout == "sites" || doc.Theme.Layout == "hub-spoke" {
		renderSiteBackgrounds(&renderer.out, layout.Groups, renderer.premium)
	} else {
		renderRowBackgrounds(&renderer.out, roles, layout.Width, renderer.premium)
		renderRowHeadings(&renderer.out, roles, byRole)
	}
	var linkAnnotations bytes.Buffer
	if err := renderLinks(&renderer.out, &linkAnnotations, doc, layout.Nodes, layout.Width, layout.Height); err != nil {
		return err
	}
	renderNodes(&renderer.out, layout.Nodes, renderer.iconPack, renderer.premium)
	renderer.out.WriteString(`<g id="link-annotations" pointer-events="none">`)
	renderer.out.Write(linkAnnotations.Bytes())
	renderer.out.WriteString(`</g>`)
	renderBundleLegend(&renderer.out, doc)
	renderer.out.WriteString(`</svg>`)
	return nil
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
			node := nodesByID[id]
			width := nodeBoxWidth(node)
			height := nodeBoxHeight(node)
			x := diagramLeft + spacing*(float64(column)+0.5) - width/2
			y := headerHeight + float64(row)*rowHeight + 112
			result[id] = placedNode{ID: id, Node: node, Box: box{X: x, Y: y, W: width, H: height}}
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
			maxNodeWidth = math.Max(maxNodeWidth, nodeBoxWidth(nodesByID[id]))
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
		width := nodeBoxWidth(node)
		height := nodeBoxHeight(node)
		angle := -math.Pi/2 + 2*math.Pi*float64(index)/float64(len(ids))
		x := centerX + radiusX*math.Cos(angle) - width/2
		y := centerY + radiusY*math.Sin(angle) - height/2
		result[id] = placedNode{ID: id, Node: node, Box: box{X: x, Y: y, W: width, H: height}}
	}
	return result
}

func nodeBoxWidth(node model.Node) float64 {
	if node.Width > 0 {
		return node.Width
	}
	return nodeWidth(node.Role)
}

func nodeBoxHeight(node model.Node) float64 {
	if node.Height > 0 {
		return node.Height
	}
	return nodeHeight
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

func renderLinks(out, annotations *bytes.Buffer, doc *model.Diagram, nodes map[string]placedNode, canvasWidth, canvasHeight float64) error {
	geometry, err := endpointAttachments(doc, nodes)
	if err != nil {
		return err
	}
	useDiagonalRoutes := doc.Theme.Layout == "hub-spoke" && doc.Theme.LinkStyle != "orthogonal"
	diagonalRoutes := make(map[int]linkRoute)
	if useDiagonalRoutes {
		links := make([]routedLink, 0, len(doc.Links))
		for index, link := range doc.Links {
			links = append(links, routedLink{
				Index:     index,
				FromNode:  link.From.Node,
				ToNode:    link.To.Node,
				Start:     geometry[endpointKey(index, true)].Point,
				End:       geometry[endpointKey(index, false)].Point,
				StartSide: geometry[endpointKey(index, true)].Side,
				EndSide:   geometry[endpointKey(index, false)].Side,
				StartStub: link.From.Stub,
				EndStub:   link.To.Stub,
			})
		}
		clearance := doc.Theme.RouteClearance
		if clearance == 0 {
			clearance = 24
		}
		diagonalRoutes = planDiagonalRoutesWithObstacles(links, clearance, nodes)
	}
	bundleVisuals, err := buildBundleVisuals(doc, geometry)
	if err != nil {
		return err
	}

	out.WriteString(`<g id="links">`)
	premium := doc.Theme.Name == "premium"
	degrees := nodeDegrees(doc)
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

		useDiagonalRoute := useDiagonalRoutes
		useOrthogonalRoute := doc.Theme.Layout == "sites" || doc.Theme.LinkStyle == "orthogonal"
		route := directRoute(start, end, startGeometry.Side, endGeometry.Side, doc.Theme.LinkStyle)
		if useDiagonalRoute {
			route = diagonalRoutes[index]
		} else if useOrthogonalRoute {
			route = orthogonalRoute(start, end, startGeometry.Side, endGeometry.Side, nodes, index)
		}
		path := route.Path
		if link.Bundle != "" {
			visual := bundleVisuals[link.Bundle]
			path = pathDataVia(start, point{X: visual.X, Y: visual.Y}, end, doc.Theme.LinkStyle)
		}
		fmt.Fprintf(out, `<g id="link-%d" data-netdiag-kind="link">`, index+1)
		if premium || useDiagonalRoute {
			underlayWidth := strokeWidth + 3.8
			underlayOpacity := .88
			if useDiagonalRoute {
				underlayWidth = strokeWidth + 7.5
				underlayOpacity = .96
			}
			fmt.Fprintf(out, `<path class="link-underlay" d="%s" fill="none" stroke="#ffffff" stroke-width="%.1f" stroke-linecap="round" stroke-linejoin="round" opacity="%.2f"/>`, path, underlayWidth, underlayOpacity)
		}
		className := ""
		if premium {
			className = ` class="premium-link"`
		}
		fmt.Fprintf(out, `<path%s d="%s" fill="none" stroke="%s" stroke-width="%.1f" stroke-linecap="round" stroke-linejoin="round" %s opacity=".86"/>`, className, path, color, strokeWidth, dash)
		renderPortMarker(out, start, color, premium)
		renderPortMarker(out, end, color, premium)
		fmt.Fprintf(annotations, `<g id="link-annotation-%d" class="link-annotation">`, index+1)
		if doc.Theme.InterfaceLabels != "none" {
			if location, ok := routeEndpointLabelLocation(route, true, degrees[from.Node], startGeometry.LabelLane, from); ok {
				location = clampInterfaceLabelLocation(location, link.SourceLabel(), from.LabelRotation, doc.Theme.InterfaceLabelStyle, canvasWidth, canvasHeight)
				renderRotatedInterfaceLabel(annotations, location.X, location.Y, link.SourceLabel(), from.LabelRotation, doc.Theme.InterfaceLabelStyle)
			}
			if location, ok := routeEndpointLabelLocation(route, false, degrees[to.Node], endGeometry.LabelLane, to); ok {
				location = clampInterfaceLabelLocation(location, link.TargetLabel(), to.LabelRotation, doc.Theme.InterfaceLabelStyle, canvasWidth, canvasHeight)
				renderRotatedInterfaceLabel(annotations, location.X, location.Y, link.TargetLabel(), to.LabelRotation, doc.Theme.InterfaceLabelStyle)
			}
		}
		renderEndpointAddress(annotations, start, from.Address, startGeometry.Side, startGeometry.LabelLane, color)
		renderEndpointAddress(annotations, end, to.Address, endGeometry.Side, endGeometry.LabelLane, color)
		if link.MiddleLabel() != "" && link.Bundle == "" {
			if useOrthogonalRoute || useDiagonalRoute {
				renderRouteLabel(annotations, route.Label, route.LabelHorizontal, link.MiddleLabel(), color, index)
			} else {
				renderCenterLabel(annotations, start, end, startGeometry.Side, endGeometry.Side, link.MiddleLabel(), color, index)
			}
		}
		if link.Bundle == "" {
			if useOrthogonalRoute || useDiagonalRoute {
				renderRouteTags(annotations, route.Label, link.Tags(), color, index)
			} else {
				renderLinkTags(annotations, start, end, link.Tags(), color, index)
			}
		}
		annotations.WriteString(`</g>`)
		out.WriteString(`</g>`)
	}
	bundleIDs := make([]string, 0, len(bundleVisuals))
	for id := range bundleVisuals {
		bundleIDs = append(bundleIDs, id)
	}
	sort.Strings(bundleIDs)
	for _, id := range bundleIDs {
		renderBundleMarker(annotations, bundleVisuals[id].X, bundleVisuals[id].Y, bundleVisuals[id])
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
		attachments[from.Node] = append(attachments[from.Node], attachment{index, true, from.Port, toCenter.X, toCenter.Y, fromSide, from.Side != "", from.Position})
		attachments[to.Node] = append(attachments[to.Node], attachment{index, false, to.Port, fromCenter.X, fromCenter.Y, toSide, to.Side != "", to.Position})
	}

	result := make(map[string]endpointGeometry)
	for nodeID, items := range attachments {
		node := nodes[nodeID]
		if doc.Theme.Layout == "hub-spoke" && node.Node.Role == "edge-router" {
			items = spreadAttachmentSides(node, items)
		}
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
				if item.Position != nil {
					if side == "top" || side == "bottom" {
						x = node.Box.X + node.Box.W**item.Position
						y = node.Box.Y
						if side == "bottom" {
							y += node.Box.H
						}
					} else {
						y = node.Box.Y + node.Box.H**item.Position
						if side == "right" {
							x += node.Box.W
						}
					}
					result[endpointKey(item.LinkIndex, item.Source)] = endpointGeometry{Point: point{x, y}, Side: side}
					continue
				}
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
	clearance := doc.Theme.EndpointClearance
	if clearance == 0 {
		clearance = 44
	}
	enforceEndpointClearance(result, attachments, nodes, clearance)
	return result, nil
}

func enforceEndpointClearance(result map[string]endpointGeometry, attachments map[string][]attachment, nodes map[string]placedNode, clearance float64) {
	if clearance <= 0 {
		return
	}
	for nodeID, items := range attachments {
		bySide := make(map[string][]attachment)
		for _, item := range items {
			bySide[item.Side] = append(bySide[item.Side], item)
		}
		for side, sideItems := range bySide {
			if len(sideItems) < 2 {
				continue
			}
			sort.SliceStable(sideItems, func(i, j int) bool {
				a := result[endpointKey(sideItems[i].LinkIndex, sideItems[i].Source)].Point
				b := result[endpointKey(sideItems[j].LinkIndex, sideItems[j].Source)].Point
				if side == "top" || side == "bottom" {
					return a.X < b.X
				}
				return a.Y < b.Y
			})
			node := nodes[nodeID]
			minimum, maximum := node.Box.X+16, node.Box.X+node.Box.W-16
			if side == "left" || side == "right" {
				minimum, maximum = node.Box.Y+14, node.Box.Y+node.Box.H-14
			}
			effective := math.Min(clearance, (maximum-minimum)/float64(len(sideItems)-1))
			offsets := make([]float64, len(sideItems))
			for index, item := range sideItems {
				geometry := result[endpointKey(item.LinkIndex, item.Source)]
				offsets[index] = geometry.Point.X
				if side == "left" || side == "right" {
					offsets[index] = geometry.Point.Y
				}
				offsets[index] = math.Max(minimum, math.Min(maximum, offsets[index]))
				if index > 0 {
					offsets[index] = math.Max(offsets[index], offsets[index-1]+effective)
				}
			}
			if offsets[len(offsets)-1] > maximum {
				offsets[len(offsets)-1] = maximum
				for index := len(offsets) - 2; index >= 0; index-- {
					offsets[index] = math.Min(offsets[index], offsets[index+1]-effective)
				}
			}
			for index, item := range sideItems {
				key := endpointKey(item.LinkIndex, item.Source)
				geometry := result[key]
				if side == "top" || side == "bottom" {
					geometry.Point.X = offsets[index]
				} else {
					geometry.Point.Y = offsets[index]
				}
				result[key] = geometry
			}
		}
	}
}

func spreadAttachmentSides(node placedNode, items []attachment) []attachment {
	if len(items) < 2 || len(items) > 4 {
		return items
	}
	center := point{X: node.Box.X + node.Box.W/2, Y: node.Box.Y + node.Box.H/2}
	sides := []string{"top", "right", "bottom", "left"}
	used := make(map[string]bool)
	result := append([]attachment(nil), items...)
	for _, item := range result {
		if item.Pinned {
			used[item.Side] = true
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Pinned != result[j].Pinned {
			return result[i].Pinned
		}
		return math.Atan2(result[i].PeerY-center.Y, result[i].PeerX-center.X) <
			math.Atan2(result[j].PeerY-center.Y, result[j].PeerX-center.X)
	})
	for index := range result {
		if result[index].Pinned {
			continue
		}
		bestSide := result[index].Side
		bestScore := math.Inf(-1)
		for _, side := range sides {
			if used[side] {
				continue
			}
			score := sideAlignment(center, point{X: result[index].PeerX, Y: result[index].PeerY}, side)
			if score > bestScore {
				bestSide, bestScore = side, score
			}
		}
		result[index].Side = bestSide
		used[bestSide] = true
	}
	return result
}

func sideAlignment(origin, peer point, side string) float64 {
	dx, dy := peer.X-origin.X, peer.Y-origin.Y
	length := math.Hypot(dx, dy)
	if length == 0 {
		return 0
	}
	switch side {
	case "top":
		return -dy / length
	case "right":
		return dx / length
	case "bottom":
		return dy / length
	case "left":
		return -dx / length
	default:
		return math.Inf(-1)
	}
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
