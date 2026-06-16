package svg

import (
	"fmt"
	"math"
	"sort"

	"github.com/gwoodwa1/netdiag/internal/model"
)

type InspectionSeverity string

const (
	InspectionInfo    InspectionSeverity = "info"
	InspectionWarning InspectionSeverity = "warning"
	InspectionError   InspectionSeverity = "error"
)

type InspectionFinding struct {
	Code       string             `json:"code"`
	Severity   InspectionSeverity `json:"severity"`
	Message    string             `json:"message"`
	Nodes      []string           `json:"nodes,omitempty"`
	Links      []int              `json:"links,omitempty"`
	Suggestion string             `json:"suggestion,omitempty"`
}

type InspectionSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

type InspectionReport struct {
	Layout   string              `json:"layout"`
	Width    float64             `json:"width"`
	Height   float64             `json:"height"`
	Score    int                 `json:"score"`
	Summary  InspectionSummary   `json:"summary"`
	Findings []InspectionFinding `json:"findings"`
}

type inspectedLabel struct {
	Link   int
	Node   string
	Source bool
	Box    box
	Center point
}

// Inspect measures the geometry produced by the native renderer. It does not
// parse rendered SVG, so identical diagrams always produce identical reports.
func Inspect(doc *model.Diagram) (InspectionReport, error) {
	roles, byRole := groupNodes(doc)
	layout := layoutDiagram(doc, roles, byRole)
	geometry, err := endpointAttachments(doc, layout.Nodes)
	if err != nil {
		return InspectionReport{}, err
	}
	routes := inspectionRoutes(doc, layout.Nodes, geometry)
	report := InspectionReport{Layout: doc.Theme.Layout, Width: layout.Width, Height: layout.Height, Findings: []InspectionFinding{}}
	report.Findings = append(report.Findings, inspectNodeOverlaps(layout.Nodes)...)
	report.Findings = append(report.Findings, inspectRouteCrossings(doc, routes)...)
	report.Findings = append(report.Findings, inspectRoutesThroughNodes(doc, routes, layout.Nodes)...)
	report.Findings = append(report.Findings, inspectEndpointCrowding(doc, geometry)...)
	report.Findings = append(report.Findings, inspectLabels(doc, routes, geometry, layout.Nodes, layout.Width, layout.Height)...)
	sort.Slice(report.Findings, func(i, j int) bool {
		left, right := report.Findings[i], report.Findings[j]
		if severityRank(left.Severity) != severityRank(right.Severity) {
			return severityRank(left.Severity) > severityRank(right.Severity)
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return left.Message < right.Message
	})
	for _, finding := range report.Findings {
		switch finding.Severity {
		case InspectionError:
			report.Summary.Errors++
		case InspectionWarning:
			report.Summary.Warnings++
		case InspectionInfo:
			report.Summary.Info++
		}
	}
	report.Score = max(0, 100-report.Summary.Errors*20-report.Summary.Warnings*5-report.Summary.Info)
	return report, nil
}

func (report InspectionReport) HasAtLeast(threshold InspectionSeverity) bool {
	for _, finding := range report.Findings {
		if severityRank(finding.Severity) >= severityRank(threshold) {
			return true
		}
	}
	return false
}

func severityRank(severity InspectionSeverity) int {
	switch severity {
	case InspectionError:
		return 3
	case InspectionWarning:
		return 2
	default:
		return 1
	}
}

func inspectionRoutes(doc *model.Diagram, nodes map[string]placedNode, geometry map[string]endpointGeometry) map[int]linkRoute {
	result := make(map[int]linkRoute, len(doc.Links))
	useDiagonal := doc.Theme.Layout == "hub-spoke" && doc.Theme.LinkStyle != "orthogonal"
	if useDiagonal {
		links := make([]routedLink, 0, len(doc.Links))
		for index, link := range doc.Links {
			links = append(links, routedLink{
				Index: index, FromNode: link.From.Node, ToNode: link.To.Node,
				Start: geometry[endpointKey(index, true)].Point, End: geometry[endpointKey(index, false)].Point,
				StartSide: geometry[endpointKey(index, true)].Side, EndSide: geometry[endpointKey(index, false)].Side,
				StartStub: link.From.Stub, EndStub: link.To.Stub,
			})
		}
		clearance := doc.Theme.RouteClearance
		if clearance == 0 {
			clearance = 24
		}
		result = planDiagonalRoutesWithObstacles(links, clearance, nodes)
	} else {
		for index := range doc.Links {
			start := geometry[endpointKey(index, true)]
			end := geometry[endpointKey(index, false)]
			route := directRoute(start.Point, end.Point, start.Side, end.Side, doc.Theme.LinkStyle)
			if doc.Theme.Layout == "sites" || doc.Theme.LinkStyle == "orthogonal" {
				route = orthogonalRoute(start.Point, end.Point, start.Side, end.Side, nodes, index)
			}
			result[index] = route
		}
	}
	bundles, _ := buildBundleVisuals(doc, geometry)
	for index, link := range doc.Links {
		if link.Bundle == "" {
			continue
		}
		start := geometry[endpointKey(index, true)].Point
		end := geometry[endpointKey(index, false)].Point
		visual := bundles[link.Bundle]
		result[index] = routeVia(start, point{X: visual.X, Y: visual.Y}, end, doc.Theme.LinkStyle)
	}
	return result
}

func routeVia(start, via, end point, style string) linkRoute {
	points := []point{start, via, end}
	if style == "orthogonal" {
		points = []point{start, {X: via.X, Y: start.Y}, via, {X: via.X, Y: end.Y}, end}
	} else if style == "clean" {
		direction := 1.0
		if end.Y < start.Y {
			direction = -1
		}
		stub := math.Min(54, math.Abs(end.Y-start.Y)*0.18)
		points = []point{start, {X: start.X, Y: start.Y + direction*stub}, via, {X: end.X, Y: end.Y - direction*stub}, end}
	}
	return linkRoute{Points: points, Path: pathDataVia(start, via, end, style), Label: via}
}

func inspectNodeOverlaps(nodes map[string]placedNode) []InspectionFinding {
	ids := sortedNodeIDs(nodes)
	var findings []InspectionFinding
	for i, leftID := range ids {
		for _, rightID := range ids[i+1:] {
			if boxesOverlap(nodes[leftID].Box, nodes[rightID].Box) {
				findings = append(findings, InspectionFinding{
					Code: "node_overlap", Severity: InspectionError,
					Message: fmt.Sprintf("nodes %s and %s overlap", leftID, rightID),
					Nodes:   []string{leftID, rightID}, Suggestion: "increase layout spacing or separate the nodes into different groups",
				})
			}
		}
	}
	return findings
}

func inspectRouteCrossings(doc *model.Diagram, routes map[int]linkRoute) []InspectionFinding {
	var findings []InspectionFinding
	for left := 0; left < len(doc.Links); left++ {
		for right := left + 1; right < len(doc.Links); right++ {
			if linksShareNode(doc.Links[left], doc.Links[right]) {
				continue
			}
			if count := routeIntersectionCount(routes[left], routes[right]); count > 0 {
				findings = append(findings, InspectionFinding{
					Code: "link_crossing", Severity: InspectionWarning,
					Message:    fmt.Sprintf("link %d (%s) crosses link %d (%s)", left+1, describeLink(doc.Links[left]), right+1, describeLink(doc.Links[right])),
					Links:      []int{left + 1, right + 1},
					Suggestion: "set endpoint sides or positions, add endpoint stubs, or increase route_clearance",
				})
			}
		}
	}
	return findings
}

func inspectRoutesThroughNodes(doc *model.Diagram, routes map[int]linkRoute, nodes map[string]placedNode) []InspectionFinding {
	ids := sortedNodeIDs(nodes)
	var findings []InspectionFinding
	for index, link := range doc.Links {
		samples := sampleRoute(routes[index], 24)
		for _, nodeID := range ids {
			if nodeID == link.From.Node || nodeID == link.To.Node {
				continue
			}
			if polylineIntersectsBox(samples, expandBox(nodes[nodeID].Box, 5)) {
				findings = append(findings, InspectionFinding{
					Code: "link_through_node", Severity: InspectionError,
					Message: fmt.Sprintf("link %d (%s) passes behind node %s, creating an apparent unlabeled endpoint", index+1, describeLink(link), nodeID),
					Nodes:   []string{nodeID}, Links: []int{index + 1},
					Suggestion: "reroute the link around the node using endpoint sides or stubs, or increase layout spacing",
				})
			}
		}
	}
	return findings
}

func inspectEndpointCrowding(doc *model.Diagram, geometry map[string]endpointGeometry) []InspectionFinding {
	type endpoint struct {
		side  string
		point point
	}
	byNode := make(map[string][]endpoint)
	for index, link := range doc.Links {
		from := geometry[endpointKey(index, true)]
		to := geometry[endpointKey(index, false)]
		byNode[link.From.Node] = append(byNode[link.From.Node], endpoint{from.Side, from.Point})
		byNode[link.To.Node] = append(byNode[link.To.Node], endpoint{to.Side, to.Point})
	}
	var findings []InspectionFinding
	nodeIDs := make([]string, 0, len(byNode))
	for nodeID := range byNode {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)
	for _, nodeID := range nodeIDs {
		bySide := make(map[string][]point)
		for _, item := range byNode[nodeID] {
			bySide[item.side] = append(bySide[item.side], item.point)
		}
		for _, side := range []string{"top", "right", "bottom", "left"} {
			points := bySide[side]
			if len(points) < 2 {
				continue
			}
			sort.Slice(points, func(i, j int) bool {
				if side == "top" || side == "bottom" {
					return points[i].X < points[j].X
				}
				return points[i].Y < points[j].Y
			})
			minimum := math.Inf(1)
			for index := 1; index < len(points); index++ {
				minimum = math.Min(minimum, math.Hypot(points[index].X-points[index-1].X, points[index].Y-points[index-1].Y))
			}
			if minimum < 24 {
				findings = append(findings, InspectionFinding{
					Code: "crowded_endpoints", Severity: InspectionWarning,
					Message:    fmt.Sprintf("%s has %d endpoints on its %s side with %.1fpx minimum separation", nodeID, len(points), side, minimum),
					Nodes:      []string{nodeID},
					Suggestion: "move some endpoints to another side, enlarge the node, or set explicit endpoint positions",
				})
			}
		}
	}
	return findings
}

func inspectLabels(doc *model.Diagram, routes map[int]linkRoute, geometry map[string]endpointGeometry, nodes map[string]placedNode, width, height float64) []InspectionFinding {
	labels := inspectionLabels(doc, routes, geometry)
	var findings []InspectionFinding
	for _, label := range labels {
		if label.Box.X < 0 || label.Box.Y < 0 || label.Box.X+label.Box.W > width || label.Box.Y+label.Box.H > height {
			findings = append(findings, InspectionFinding{
				Code: "label_outside_canvas", Severity: InspectionError,
				Message: fmt.Sprintf("interface label for link %d extends outside the canvas", label.Link),
				Nodes:   []string{label.Node}, Links: []int{label.Link},
				Suggestion: "change the endpoint side or increase canvas/layout spacing",
			})
		}
		for nodeID, node := range nodes {
			if nodeID == label.Node {
				continue
			}
			if boxesOverlap(label.Box, node.Box) {
				findings = append(findings, InspectionFinding{
					Code: "label_node_overlap", Severity: InspectionWarning,
					Message: fmt.Sprintf("interface label for link %d overlaps node %s", label.Link, nodeID),
					Nodes:   []string{label.Node, nodeID}, Links: []int{label.Link},
					Suggestion: "rotate the label, add an endpoint stub, or move the endpoint to another side",
				})
			}
		}
		for routeIndex := range doc.Links {
			if routeIndex+1 == label.Link {
				continue
			}
			if polylineIntersectsBox(sampleRoute(routes[routeIndex], 24), label.Box) {
				findings = append(findings, InspectionFinding{
					Code: "label_link_overlap", Severity: InspectionWarning,
					Message: fmt.Sprintf("link %d (%s) passes through an interface label for link %d", routeIndex+1, describeLink(doc.Links[routeIndex]), label.Link),
					Nodes:   []string{label.Node}, Links: []int{routeIndex + 1, label.Link},
					Suggestion: "add an endpoint stub, rotate the label, or move one link to a different route lane",
				})
			}
		}
		if distance := labelDistanceToRoute(label.Center, routes[label.Link-1]); distance > 160 {
			findings = append(findings, InspectionFinding{
				Code: "label_detached_from_route", Severity: InspectionWarning,
				Message: fmt.Sprintf("interface label for link %d is %.1fpx away from its route", label.Link, distance),
				Nodes:   []string{label.Node}, Links: []int{label.Link},
				Suggestion: "reduce label_offset or adjust label_along so the label remains visually attached to the route",
			})
		}
	}
	for left := 0; left < len(labels); left++ {
		for right := left + 1; right < len(labels); right++ {
			if labels[left].Link == labels[right].Link {
				if labelBoxDistance(labels[left].Box, labels[right].Box) < 24 {
					link := doc.Links[labels[left].Link-1]
					findings = append(findings, InspectionFinding{
						Code: "endpoint_labels_too_close", Severity: InspectionError,
						Message: fmt.Sprintf("source and target interface labels for link %d (%s) have less than 24px clearance", labels[left].Link, describeLink(link)),
						Nodes:   []string{link.From.Node, link.To.Node}, Links: []int{labels[left].Link},
						Suggestion: "separate the endpoint positions, add endpoint stubs, or rotate one endpoint label",
					})
				}
				continue
			}
			if boxesOverlap(labels[left].Box, labels[right].Box) {
				findings = append(findings, InspectionFinding{
					Code: "label_overlap", Severity: InspectionWarning,
					Message:    fmt.Sprintf("interface labels for links %d and %d overlap", labels[left].Link, labels[right].Link),
					Links:      []int{labels[left].Link, labels[right].Link},
					Suggestion: "rotate a label, add endpoint stubs, or separate the endpoint positions",
				})
			}
		}
	}
	return findings
}

func inspectionLabels(doc *model.Diagram, routes map[int]linkRoute, geometry map[string]endpointGeometry) []inspectedLabel {
	if doc.Theme.InterfaceLabels == "none" {
		return nil
	}
	degrees := nodeDegrees(doc)
	var labels []inspectedLabel
	for index, link := range doc.Links {
		for _, source := range []bool{true, false} {
			endpoint := link.To
			label := link.TargetLabel()
			if source {
				endpoint = link.From
				label = link.SourceLabel()
			}
			if label == "" {
				continue
			}
			item := geometry[endpointKey(index, source)]
			location, ok := routeEndpointLabelLocation(routes[index], source, degrees[endpoint.Node], item.LabelLane, endpoint)
			if !ok {
				location = fallbackEndpointLabelLocation(item)
			}
			labels = append(labels, inspectedLabel{Link: index + 1, Node: endpoint.Node, Source: source, Center: location, Box: interfaceLabelBox(location, label, endpoint.LabelRotation, doc.Theme.InterfaceLabelStyle)})
		}
	}
	return labels
}

func labelBoxDistance(left, right box) float64 {
	dx := math.Max(0, math.Max(left.X-(right.X+right.W), right.X-(left.X+left.W)))
	dy := math.Max(0, math.Max(left.Y-(right.Y+right.H), right.Y-(left.Y+left.H)))
	return math.Hypot(dx, dy)
}

func fallbackEndpointLabelLocation(endpoint endpointGeometry) point {
	location := endpoint.Point
	location.Y -= 12
	switch endpoint.Side {
	case "bottom":
		location.Y = endpoint.Point.Y + 25
	case "left":
		location.X = endpoint.Point.X - horizontalLabelOffset
	case "right":
		location.X = endpoint.Point.X + horizontalLabelOffset
	}
	return location
}

func labelDistanceToRoute(location point, route linkRoute) float64 {
	points := sampleRoute(route, 48)
	if len(points) == 0 {
		return 0
	}
	minimum := math.Inf(1)
	for index := 1; index < len(points); index++ {
		minimum = math.Min(minimum, distancePointToSegment(location, points[index-1], points[index]))
	}
	return minimum
}

func distancePointToSegment(value, start, end point) float64 {
	dx, dy := end.X-start.X, end.Y-start.Y
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return math.Hypot(value.X-start.X, value.Y-start.Y)
	}
	position := ((value.X-start.X)*dx + (value.Y-start.Y)*dy) / lengthSquared
	position = math.Max(0, math.Min(1, position))
	projection := point{X: start.X + position*dx, Y: start.Y + position*dy}
	return math.Hypot(value.X-projection.X, value.Y-projection.Y)
}

func interfaceLabelBox(location point, label string, rotation int, style model.InterfaceLabelStyle) box {
	const size = 11.0
	width := math.Max(38, float64(len([]rune(label)))*size*0.61+style.PaddingX*2)
	height := size + style.PaddingY*2
	centerY := location.Y - size/2
	if rotation == 90 || rotation == 270 {
		width, height = height, width
	}
	return box{X: location.X - width/2, Y: centerY - height/2, W: width, H: height}
}

func linksShareNode(left, right model.Link) bool {
	return left.From.Node == right.From.Node || left.From.Node == right.To.Node ||
		left.To.Node == right.From.Node || left.To.Node == right.To.Node
}

func describeLink(link model.Link) string {
	return link.From.Node + " -> " + link.To.Node
}

func polylineIntersectsBox(points []point, obstacle box) bool {
	for index := 1; index < len(points); index++ {
		if generalSegmentIntersectsBox(points[index-1], points[index], obstacle) {
			return true
		}
	}
	return false
}

func generalSegmentIntersectsBox(start, end point, obstacle box) bool {
	if pointInsideBox(start, obstacle) || pointInsideBox(end, obstacle) {
		return true
	}
	topLeft := point{obstacle.X, obstacle.Y}
	topRight := point{obstacle.X + obstacle.W, obstacle.Y}
	bottomLeft := point{obstacle.X, obstacle.Y + obstacle.H}
	bottomRight := point{obstacle.X + obstacle.W, obstacle.Y + obstacle.H}
	return segmentsCross(start, end, topLeft, topRight) ||
		segmentsCross(start, end, topRight, bottomRight) ||
		segmentsCross(start, end, bottomRight, bottomLeft) ||
		segmentsCross(start, end, bottomLeft, topLeft)
}

func pointInsideBox(value point, target box) bool {
	return value.X > target.X && value.X < target.X+target.W && value.Y > target.Y && value.Y < target.Y+target.H
}

func boxesOverlap(left, right box) bool {
	return left.X < right.X+right.W && left.X+left.W > right.X &&
		left.Y < right.Y+right.H && left.Y+left.H > right.Y
}

func sortedNodeIDs(nodes map[string]placedNode) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
