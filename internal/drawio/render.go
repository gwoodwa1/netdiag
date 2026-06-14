package drawio

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

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
}

type point struct {
	As string `xml:"as,attr"`
}

func Render(diagram *model.Diagram) ([]byte, error) {
	cells := []cell{{ID: "0"}, {ID: "1", Parent: "0"}}
	nodeParent, groupCells := layoutGroups(diagram.Groups)
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
		cells = append(cells, cell{
			ID: nodeCellID(node.ID), Value: defaultString(node.Label, node.ID), Style: style,
			Parent: parent, Vertex: "1", Geometry: &geometry{X: x, Y: y, Width: 170, Height: 70, As: "geometry"},
		})
	}

	for index, link := range diagram.Links {
		linkID := fmt.Sprintf("link-%d", index+1)
		sourceLabel := endpointLabel(link.SourceLabel(), link.From.Address)
		targetLabel := endpointLabel(link.TargetLabel(), link.To.Address)
		cells = append(cells, cell{
			ID: linkID, Value: link.MiddleLabel(),
			Style:  "edgeStyle=orthogonalEdgeStyle;rounded=1;html=1;endArrow=none;startArrow=none;jettySize=auto;",
			Parent: "1", Edge: "1", Source: nodeCellID(link.From.Node), Target: nodeCellID(link.To.Node),
			Geometry: &geometry{Relative: "1", As: "geometry"},
		})
		cells = appendEdgeLabel(cells, linkID+"-source", linkID, sourceLabel, -0.8)
		cells = appendEdgeLabel(cells, linkID+"-target", linkID, targetLabel, 0.8)
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

func layoutGroups(groups []model.Group) (map[string]string, []cell) {
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
		cells = append(cells, cell{
			ID: groupCellID(group.ID), Value: defaultString(group.Label, group.ID),
			Style:  "swimlane;html=1;rounded=1;horizontal=1;startSize=32;fillColor=#dbeafe;swimlaneFillColor=#eff6ff;strokeColor=#93c5fd;fontStyle=1;",
			Parent: parent, Vertex: "1", Geometry: &geometry{X: x, Y: y, Width: 650, Height: 360, As: "geometry"},
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

func groupCellID(id string) string { return "group-" + id }
func nodeCellID(id string) string  { return "node-" + id }

func appendEdgeLabel(cells []cell, id, parent, value string, position float64) []cell {
	if value == "" {
		return cells
	}
	return append(cells, cell{
		ID: id, Value: value,
		Style:       "edgeLabel;html=1;align=center;verticalAlign=middle;resizable=0;points=[];labelBackgroundColor=#ffffff;",
		Parent:      parent,
		Vertex:      "1",
		Connectable: "0",
		Geometry:    &geometry{X: position, Relative: "1", As: "geometry", Offset: &point{As: "offset"}},
	})
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
