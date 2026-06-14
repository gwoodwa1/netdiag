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
	X  float64 `xml:"x,attr,omitempty"`
	Y  float64 `xml:"y,attr,omitempty"`
	As string  `xml:"as,attr,omitempty"`
}

type points struct {
	As     string  `xml:"as,attr"`
	Points []point `xml:"mxPoint"`
}

type Options struct {
	Overrides *layoutoverride.Document
}

func Render(diagram *model.Diagram) ([]byte, error) {
	return RenderWithOptions(diagram, Options{})
}

func RenderWithOptions(diagram *model.Diagram, options Options) ([]byte, error) {
	if err := validateOverrideReferences(diagram, options.Overrides); err != nil {
		return nil, err
	}
	overrides := layoutoverride.Overrides{}
	if options.Overrides != nil {
		overrides = options.Overrides.LayoutOverrides
	}
	cells := []cell{{ID: "0"}, {ID: "1", Parent: "0"}}
	nodeParent, groupCells := layoutGroups(diagram.Groups, overrides.Groups)
	cells = append(cells, groupCells...)

	nodes := append([]model.Node(nil), diagram.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
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
		}
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

func layoutGroups(groups []model.Group, overrides map[string]layoutoverride.Bounds) (map[string]string, []cell) {
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
		Style:       "edgeLabel;html=1;align=center;verticalAlign=middle;resizable=0;points=[];labelBackgroundColor=#ffffff;",
		Parent:      parent, NetdiagID: semanticID, NetdiagKind: "label",
		Vertex:      "1",
		Connectable: "0",
		Geometry:    &geometry{X: position, Relative: "1", As: "geometry", Offset: &point{As: "offset"}},
	})
}

func validateOverrideReferences(diagram *model.Diagram, overrides *layoutoverride.Document) error {
	if overrides == nil {
		return nil
	}
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
		links[link.StableID()] = true
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
		"top": "0.5;"+prefix+"Y=0;", "right": "1;"+prefix+"Y=0.5;",
		"bottom": "0.5;"+prefix+"Y=1;", "left": "0;"+prefix+"Y=0.5;",
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
