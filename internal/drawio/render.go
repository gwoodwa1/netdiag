package drawio

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/layoutoverride"
	"github.com/gwoodwa1/netdiag/internal/model"
)

type cell struct {
	XMLName     xml.Name  `xml:"mxCell"`
	ID          string    `xml:"id,attr"`
	Value       string    `xml:"value,attr,omitempty"`
	Style       string    `xml:"style,attr,omitempty"`
	Parent      string    `xml:"parent,attr,omitempty"`
	Vertex      string    `xml:"vertex,attr,omitempty"`
	Edge        string    `xml:"edge,attr,omitempty"`
	Source      string    `xml:"source,attr,omitempty"`
	Target      string    `xml:"target,attr,omitempty"`
	Connectable string    `xml:"connectable,attr,omitempty"`
	NetdiagID   string    `xml:"netdiag-id,attr,omitempty"`
	NetdiagKind string    `xml:"netdiag-kind,attr,omitempty"`
	Geometry    *geometry `xml:"mxGeometry,omitempty"`
}

type geometry struct {
	X        float64 `xml:"x,attr,omitempty"`
	Y        float64 `xml:"y,attr,omitempty"`
	Width    float64 `xml:"width,attr,omitempty"`
	Height   float64 `xml:"height,attr,omitempty"`
	Relative string  `xml:"relative,attr,omitempty"`
	As       string  `xml:"as,attr"`
	Offset   *point  `xml:"mxPoint,omitempty"`
	Points   *points `xml:"Array,omitempty"`
}

type point struct {
	X  float64 `xml:"x,attr"`
	Y  float64 `xml:"y,attr"`
	As string  `xml:"as,attr,omitempty"`
}

type points struct {
	As     string  `xml:"as,attr"`
	Points []point `xml:"mxPoint"`
}

type Options struct {
	Overrides *layoutoverride.Document
	Report    *LayoutReport
}

type LayoutReport struct {
	Preserved    LayoutPreserved   `json:"preserved"`
	AutoPlaced   []LayoutPlacement `json:"auto_placed,omitempty"`
	AutoRouted   []string          `json:"auto_routed,omitempty"`
	IgnoredStale []string          `json:"ignored_stale_overrides,omitempty"`
}

type LayoutPreserved struct {
	Nodes  int `json:"nodes"`
	Groups int `json:"groups"`
	Links  int `json:"links"`
}

type LayoutPlacement struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Near string `json:"near,omitempty"`
}

func Render(diagram *model.Diagram) ([]byte, error) {
	return RenderWithOptions(diagram, Options{})
}

func RenderWithOptions(diagram *model.Diagram, options Options) ([]byte, error) {
	if options.Overrides != nil {
		if err := layoutoverride.Validate(options.Overrides); err != nil {
			return nil, fmt.Errorf("invalid layout overrides: %w", err)
		}
	}
	if err := validateOverrideReferences(diagram, options.Overrides); err != nil {
		return nil, err
	}
	return renderWithOptions(diagram, options)
}

func RenderWithLayoutReport(diagram *model.Diagram, options Options) ([]byte, LayoutReport, error) {
	if options.Overrides != nil {
		if err := layoutoverride.Validate(options.Overrides); err != nil {
			return nil, LayoutReport{}, fmt.Errorf("invalid layout overrides: %w", err)
		}
	}
	overrides, report, err := reconcileOverrides(diagram, options.Overrides)
	if err != nil {
		return nil, LayoutReport{}, err
	}
	options.Overrides = overrides
	options.Report = &report
	result, err := renderWithOptions(diagram, options)
	sort.Strings(report.AutoRouted)
	return result, report, err
}

func renderWithOptions(diagram *model.Diagram, options Options) ([]byte, error) {
	overrides := layoutoverride.Overrides{}
	if options.Overrides != nil {
		overrides = options.Overrides.LayoutOverrides
	}
	cells := []cell{{ID: "0"}, {ID: "1", Parent: "0"}}
	nodeParent, groupCells := layoutGroups(diagram.Groups, overrides.Groups, options.Report)
	cells = append(cells, groupCells...)

	nodes := append([]model.Node(nil), diagram.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	neighbors := nodeNeighbors(diagram.Links)
	placed := overriddenNodePlacements(nodes, nodeParent, overrides.Nodes)
	anchors := placementIDs(placed)
	groupSlots := make(map[string]int)
	ungrouped := 0
	for _, node := range nodes {
		parent := nodeParent[node.ID]
		x, y := 80.0+float64(ungrouped%5)*240, 100.0+float64(ungrouped/5)*150
		if parent == "" {
			parent = "1"
			ungrouped++
		} else {
			slot := groupSlots[parent]
			x, y = 35+float64(slot%3)*205, 70+float64(slot/3)*115
			groupSlots[parent]++
		}
		visual := styleForRole(node.Role)
		stroke := visual.Stroke
		if node.Color != "" {
			stroke = node.Color
		}
		style := fmt.Sprintf("shape=%s;whiteSpace=wrap;html=1;rounded=1;fillColor=%s;strokeColor=%s;fontStyle=1;", visual.Shape, visual.Fill, stroke)
		width, height := 170.0, 70.0
		if override, ok := overrides.Nodes[node.ID]; ok {
			x, y, width, height = applyBounds(x, y, width, height, override)
			style = applyBoundsStyle(style, override)
		} else if nearX, nearY, nearID, ok := placeNearManagedNeighbor(node.ID, parent, width, height, neighbors, placed, anchors); ok {
			x, y = nearX, nearY
			reportAutoPlaced(options.Report, "node", node.ID, nearID)
		} else {
			reportAutoPlaced(options.Report, "node", node.ID, "")
		}
		placed[node.ID] = nodePlacement{Parent: parent, X: x, Y: y, Width: width, Height: height}
		cells = append(cells, cell{
			ID: nodeCellID(node.ID), Value: defaultString(node.Label, node.ID), Style: style,
			Parent: parent, Vertex: "1", NetdiagID: node.ID, NetdiagKind: "node",
			Geometry: &geometry{X: x, Y: y, Width: width, Height: height, As: "geometry"},
		})
	}

	for _, link := range diagram.Links {
		stableID := link.StableID()
		linkID := linkCellID(stableID)
		sourceLabel := endpointLabel(link.SourceLabel(), link.From.Address)
		targetLabel := endpointLabel(link.TargetLabel(), link.To.Address)
		style := "edgeStyle=orthogonalEdgeStyle;rounded=1;html=1;endArrow=none;startArrow=none;jettySize=auto;"
		linkGeometry := &geometry{Relative: "1", As: "geometry"}
		if override, ok := overrides.Links[stableID]; ok {
			style = applyLinkOverride(style, override)
			linkGeometry.Points = overridePoints(override.Waypoints)
		} else if options.Report != nil {
			options.Report.AutoRouted = append(options.Report.AutoRouted, stableID)
		}
		cells = append(cells, cell{
			ID: linkID, Value: link.MiddleLabel(),
			Style: style, NetdiagID: stableID, NetdiagKind: "link",
			Parent: "1", Edge: "1", Source: nodeCellID(link.From.Node), Target: nodeCellID(link.To.Node),
			Geometry: linkGeometry,
		})
		cells = appendEdgeLabel(cells, labelCellID(stableID, "source"), linkID, "label:"+stableID+":source", sourceLabel, -0.8)
		cells = appendEdgeLabel(cells, labelCellID(stableID, "target"), linkID, "label:"+stableID+":target", targetLabel, 0.8)
	}

	var graph bytes.Buffer
	graph.WriteString(`<mxGraphModel dx="1422" dy="794" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1600" pageHeight="1200" math="0" shadow="0"><root>`)
	encoder := xml.NewEncoder(&graph)
	for _, item := range cells {
		if err := encoder.Encode(item); err != nil {
			return nil, err
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	graph.WriteString(`</root></mxGraphModel>`)

	title := defaultString(diagram.Theme.Title, "Network Diagram")
	var out bytes.Buffer
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&out, `<mxfile host="netdiag" modified="2026-01-01T00:00:00.000Z" agent="netdiag" version="24.7.17" type="device"><diagram id="netdiag-page-1" name="%s">`, escapeAttr(title))
	out.Write(graph.Bytes())
	out.WriteString(`</diagram></mxfile>`)
	return out.Bytes(), nil
}

type nodePlacement struct {
	Parent              string
	X, Y, Width, Height float64
}

func nodeNeighbors(links []model.Link) map[string][]string {
	result := make(map[string][]string)
	for _, link := range links {
		result[link.From.Node] = append(result[link.From.Node], link.To.Node)
		result[link.To.Node] = append(result[link.To.Node], link.From.Node)
	}
	for id := range result {
		sort.Strings(result[id])
	}
	return result
}

func overriddenNodePlacements(nodes []model.Node, nodeParent map[string]string, overrides map[string]layoutoverride.Bounds) map[string]nodePlacement {
	result := make(map[string]nodePlacement)
	for _, node := range nodes {
		override, ok := overrides[node.ID]
		if !ok || override.X == nil || override.Y == nil {
			continue
		}
		parent := nodeParent[node.ID]
		if parent == "" {
			parent = "1"
		}
		_, _, width, height := applyBounds(0, 0, 170, 70, override)
		result[node.ID] = nodePlacement{Parent: parent, X: *override.X, Y: *override.Y, Width: width, Height: height}
	}
	return result
}

func placementIDs(placed map[string]nodePlacement) map[string]bool {
	result := make(map[string]bool, len(placed))
	for id := range placed {
		result[id] = true
	}
	return result
}

func placeNearManagedNeighbor(id, parent string, width, height float64, neighbors map[string][]string, placed map[string]nodePlacement, anchors map[string]bool) (float64, float64, string, bool) {
	const maxAttempts = 8
	for _, neighborID := range neighbors[id] {
		if !anchors[neighborID] {
			continue
		}
		neighbor, ok := placed[neighborID]
		if !ok || neighbor.Parent != parent {
			continue
		}
		x, y := neighbor.X, neighbor.Y+neighbor.Height+80
		for attempts := 0; attempts < maxAttempts; attempts++ {
			if !placementOverlaps(parent, x, y, width, height, placed) {
				return x, y, neighborID, true
			}
			x += width + 70
		}
	}
	return 0, 0, "", false
}

func placementOverlaps(parent string, x, y, width, height float64, placed map[string]nodePlacement) bool {
	const gap = 20.0
	for _, item := range placed {
		if item.Parent != parent {
			continue
		}
		if x < item.X+item.Width+gap && x+width+gap > item.X && y < item.Y+item.Height+gap && y+height+gap > item.Y {
			return true
		}
	}
	return false
}

func layoutGroups(groups []model.Group, overrides map[string]layoutoverride.Bounds, report *LayoutReport) (map[string]string, []cell) {
	nodeParent := make(map[string]string)
	var cells []cell
	sorted := append([]model.Group(nil), groups...)
	parentByID := make(map[string]string, len(groups))
	for _, group := range groups {
		parentByID[group.ID] = group.ParentID
	}
	sort.Slice(sorted, func(i, j int) bool {
		leftDepth := groupDepth(sorted[i].ID, parentByID)
		rightDepth := groupDepth(sorted[j].ID, parentByID)
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return sorted[i].ID < sorted[j].ID
	})
	groupSlots := make(map[string]int)
	for _, group := range sorted {
		parent := "1"
		slot := groupSlots[parent]
		x, y := 40.0+float64(slot%3)*700, 50.0+float64(slot/3)*430
		if group.ParentID != "" {
			parent = groupCellID(group.ParentID)
			slot = groupSlots[parent]
			x, y = 25+float64(slot%2)*310, 60+float64(slot/2)*170
		}
		groupSlots[parent]++
		width, height := 650.0, 360.0
		override := overrides[group.ID]
		if _, ok := overrides[group.ID]; !ok {
			reportAutoPlaced(report, "group", group.ID, "")
		}
		x, y, width, height = applyBounds(x, y, width, height, override)
		style := applyBoundsStyle("swimlane;html=1;rounded=1;horizontal=1;startSize=32;fillColor=#dbeafe;swimlaneFillColor=#eff6ff;strokeColor=#93c5fd;fontStyle=1;", override)
		cells = append(cells, cell{
			ID: groupCellID(group.ID), Value: defaultString(group.Label, group.ID),
			Style: style, NetdiagID: group.ID, NetdiagKind: "group",
			Parent: parent, Vertex: "1", Geometry: &geometry{X: x, Y: y, Width: width, Height: height, As: "geometry"},
		})
		for _, nodeID := range group.NodeIDs {
			nodeParent[nodeID] = groupCellID(group.ID)
		}
	}
	return nodeParent, cells
}

func reportAutoPlaced(report *LayoutReport, kind, id, near string) {
	if report != nil {
		report.AutoPlaced = append(report.AutoPlaced, LayoutPlacement{Kind: kind, ID: id, Near: near})
	}
}

func groupDepth(id string, parentByID map[string]string) int {
	depth := 0
	seen := make(map[string]bool)
	for parent := parentByID[id]; parent != "" && !seen[parent]; parent = parentByID[parent] {
		seen[parent] = true
		depth++
	}
	return depth
}

func groupCellID(id string) string { return safeCellID("group", id) }
func nodeCellID(id string) string  { return safeCellID("node", id) }
func linkCellID(id string) string  { return safeCellID("link", id) }
func labelCellID(id, position string) string {
	return safeCellID("label", id+"-"+position)
}

func appendEdgeLabel(cells []cell, id, parent, semanticID, value string, position float64) []cell {
	if value == "" {
		return cells
	}
	return append(cells, cell{
		ID: id, Value: value,
		Style:  "edgeLabel;html=1;align=center;verticalAlign=middle;resizable=0;points=[];labelBackgroundColor=#ffffff;",
		Parent: parent, NetdiagID: semanticID, NetdiagKind: "label",
		Vertex:      "1",
		Connectable: "0",
		Geometry:    &geometry{X: position, Relative: "1", As: "geometry", Offset: &point{As: "offset"}},
	})
}

func reconcileOverrides(diagram *model.Diagram, overrides *layoutoverride.Document) (*layoutoverride.Document, LayoutReport, error) {
	var report LayoutReport
	nodes, groups, links, err := topologyIDs(diagram)
	if err != nil {
		return nil, report, err
	}
	if overrides == nil {
		return nil, report, nil
	}
	result := &layoutoverride.Document{
		Version: overrides.Version,
		LayoutOverrides: layoutoverride.Overrides{
			Nodes:  make(map[string]layoutoverride.Bounds),
			Groups: make(map[string]layoutoverride.Bounds),
			Links:  make(map[string]layoutoverride.Link),
		},
	}
	for id, value := range overrides.LayoutOverrides.Nodes {
		if nodes[id] {
			result.LayoutOverrides.Nodes[id] = value
			report.Preserved.Nodes++
		} else {
			report.IgnoredStale = append(report.IgnoredStale, "node "+id)
		}
	}
	for id, value := range overrides.LayoutOverrides.Groups {
		if groups[id] {
			result.LayoutOverrides.Groups[id] = value
			report.Preserved.Groups++
		} else {
			report.IgnoredStale = append(report.IgnoredStale, "group "+id)
		}
	}
	for id, value := range overrides.LayoutOverrides.Links {
		if links[id] {
			result.LayoutOverrides.Links[id] = value
			report.Preserved.Links++
		} else {
			report.IgnoredStale = append(report.IgnoredStale, "link "+id)
		}
	}
	sort.Strings(report.IgnoredStale)
	return result, report, nil
}

func FormatLayoutReport(report LayoutReport) string {
	var result strings.Builder
	fmt.Fprintf(&result, "Preserved:\n- %d nodes\n- %d links\n- %d groups\n\n", report.Preserved.Nodes, report.Preserved.Links, report.Preserved.Groups)
	result.WriteString("Auto-placed:\n")
	if len(report.AutoPlaced) == 0 {
		result.WriteString("- none\n")
	} else {
		for _, placement := range report.AutoPlaced {
			if placement.Near != "" {
				fmt.Fprintf(&result, "- %s %s near %s\n", placement.Kind, placement.ID, placement.Near)
			} else {
				fmt.Fprintf(&result, "- %s %s using generated placement\n", placement.Kind, placement.ID)
			}
		}
	}
	result.WriteString("\nAuto-routed:\n")
	if len(report.AutoRouted) == 0 {
		result.WriteString("- none\n")
	} else {
		for _, id := range report.AutoRouted {
			fmt.Fprintf(&result, "- %s\n", id)
		}
	}
	result.WriteString("\nIgnored stale overrides:\n")
	if len(report.IgnoredStale) == 0 {
		result.WriteString("- none\n")
	} else {
		for _, id := range report.IgnoredStale {
			fmt.Fprintf(&result, "- %s\n", id)
		}
	}
	return result.String()
}

func validateOverrideReferences(diagram *model.Diagram, overrides *layoutoverride.Document) error {
	nodes := make(map[string]bool)
	groups := make(map[string]bool)
	links := make(map[string]bool)
	for _, node := range diagram.Nodes {
		nodes[node.ID] = true
	}
	for _, group := range diagram.Groups {
		groups[group.ID] = true
	}
	for _, link := range diagram.Links {
		stableID := link.StableID()
		if links[stableID] {
			return fmt.Errorf("links resolve to duplicate stable ID %q; assign explicit unique link IDs", stableID)
		}
		links[stableID] = true
	}
	if overrides == nil {
		return nil
	}
	for id := range overrides.LayoutOverrides.Nodes {
		if !nodes[id] {
			return fmt.Errorf("layout overrides reference unknown node %q", id)
		}
	}
	for id := range overrides.LayoutOverrides.Groups {
		if !groups[id] {
			return fmt.Errorf("layout overrides reference unknown group %q", id)
		}
	}
	for id := range overrides.LayoutOverrides.Links {
		if !links[id] {
			return fmt.Errorf("layout overrides reference unknown link %q", id)
		}
	}
	return nil
}

func applyBounds(x, y, width, height float64, override layoutoverride.Bounds) (float64, float64, float64, float64) {
	if override.X != nil {
		x = *override.X
	}
	if override.Y != nil {
		y = *override.Y
	}
	if override.Width != nil {
		width = *override.Width
	}
	if override.Height != nil {
		height = *override.Height
	}
	return x, y, width, height
}

func applyBoundsStyle(style string, override layoutoverride.Bounds) string {
	if override.Locked {
		style += "locked=1;"
	}
	if override.Style != "" {
		style += "netdiagStyle=" + override.Style + ";"
	}
	return style
}

func applyLinkOverride(style string, override layoutoverride.Link) string {
	switch override.Style {
	case "straight":
		style = strings.ReplaceAll(style, "edgeStyle=orthogonalEdgeStyle;", "edgeStyle=none;")
	case "curved":
		style = strings.ReplaceAll(style, "edgeStyle=orthogonalEdgeStyle;", "edgeStyle=none;curved=1;")
	}
	style += sideStyle("exit", override.SourceSide)
	style += sideStyle("entry", override.TargetSide)
	if override.Locked {
		style += "locked=1;"
	}
	return style
}

func sideStyle(prefix, side string) string {
	values := map[string]string{
		"top": "0.5;" + prefix + "Y=0;", "right": "1;" + prefix + "Y=0.5;",
		"bottom": "0.5;" + prefix + "Y=1;", "left": "0;" + prefix + "Y=0.5;",
	}
	if value := values[side]; value != "" {
		return prefix + "X=" + value + prefix + "Perimeter=0;"
	}
	return ""
}

func overridePoints(values []layoutoverride.Point) *points {
	if len(values) == 0 {
		return nil
	}
	result := &points{As: "points", Points: make([]point, len(values))}
	for index, value := range values {
		result.Points[index] = point{X: value.X, Y: value.Y}
	}
	return result
}

func safeCellID(kind, semanticID string) string {
	safe := true
	for _, value := range semanticID {
		if !((value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '-' || value == '_' || value == '.') {
			safe = false
			break
		}
	}
	if safe && semanticID != "" {
		return kind + "-" + semanticID
	}
	sum := sha256.Sum256([]byte(semanticID))
	return fmt.Sprintf("%s-%x", kind, sum[:8])
}

func endpointLabel(label, address string) string {
	if address == "" {
		return label
	}
	return strings.TrimSpace(label + " " + address)
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func escapeAttr(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
