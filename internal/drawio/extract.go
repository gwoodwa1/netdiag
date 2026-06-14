package drawio

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/layoutoverride"
	"github.com/gwoodwa1/netdiag/internal/model"
)

type drawioFile struct {
	Diagrams []drawioPage `xml:"diagram"`
}

type drawioPage struct {
	Content string `xml:",innerxml"`
}

type extractedGraph struct {
	Cells []extractedCell `xml:"root>mxCell"`
}

type extractedCell struct {
	Style       string             `xml:"style,attr"`
	NetdiagID   string             `xml:"netdiag-id,attr"`
	NetdiagKind string             `xml:"netdiag-kind,attr"`
	Geometry    *extractedGeometry `xml:"mxGeometry"`
}

type extractedGeometry struct {
	X      float64          `xml:"x,attr"`
	Y      float64          `xml:"y,attr"`
	Width  float64          `xml:"width,attr"`
	Height float64          `xml:"height,attr"`
	Points *extractedPoints `xml:"Array"`
}

type extractedPoints struct {
	As     string           `xml:"as,attr"`
	Points []layoutoverride.Point `xml:"mxPoint"`
}

func ExtractOverrides(data []byte, diagram *model.Diagram) (*layoutoverride.Document, error) {
	var file drawioFile
	if err := xml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse draw.io file: %w", err)
	}
	if len(file.Diagrams) == 0 {
		return nil, fmt.Errorf("draw.io file contains no diagram pages")
	}

	nodes, groups, links, err := topologyIDs(diagram)
	if err != nil {
		return nil, err
	}
	result := &layoutoverride.Document{
		Version: 1,
		LayoutOverrides: layoutoverride.Overrides{
			Nodes:  make(map[string]layoutoverride.Bounds),
			Groups: make(map[string]layoutoverride.Bounds),
			Links:  make(map[string]layoutoverride.Link),
		},
	}
	seen := make(map[string]bool)
	for pageIndex, page := range file.Diagrams {
		graphData, err := decodePage(page.Content)
		if err != nil {
			return nil, fmt.Errorf("decode draw.io page %d: %w", pageIndex+1, err)
		}
		var graph extractedGraph
		if err := xml.Unmarshal(graphData, &graph); err != nil {
			return nil, fmt.Errorf("parse draw.io page %d graph: %w", pageIndex+1, err)
		}
		for _, cell := range graph.Cells {
			if cell.NetdiagID == "" || cell.NetdiagKind == "" {
				continue
			}
			key := cell.NetdiagKind + ":" + cell.NetdiagID
			if seen[key] {
				return nil, fmt.Errorf("draw.io file contains duplicate %s ID %q", cell.NetdiagKind, cell.NetdiagID)
			}
			seen[key] = true
			switch cell.NetdiagKind {
			case "node":
				if !nodes[cell.NetdiagID] {
					return nil, fmt.Errorf("draw.io file references unknown node %q", cell.NetdiagID)
				}
				bounds, err := extractBounds(cell)
				if err != nil {
					return nil, err
				}
				result.LayoutOverrides.Nodes[cell.NetdiagID] = bounds
			case "group":
				if !groups[cell.NetdiagID] {
					return nil, fmt.Errorf("draw.io file references unknown group %q", cell.NetdiagID)
				}
				bounds, err := extractBounds(cell)
				if err != nil {
					return nil, err
				}
				result.LayoutOverrides.Groups[cell.NetdiagID] = bounds
			case "link":
				if !links[cell.NetdiagID] {
					return nil, fmt.Errorf("draw.io file references unknown link %q", cell.NetdiagID)
				}
				result.LayoutOverrides.Links[cell.NetdiagID] = extractLink(cell)
			}
		}
	}
	if err := layoutoverride.Validate(result); err != nil {
		return nil, fmt.Errorf("validate extracted layout overrides: %w", err)
	}
	return result, nil
}

func topologyIDs(diagram *model.Diagram) (map[string]bool, map[string]bool, map[string]bool, error) {
	nodes := make(map[string]bool, len(diagram.Nodes))
	groups := make(map[string]bool, len(diagram.Groups))
	links := make(map[string]bool, len(diagram.Links))
	for _, node := range diagram.Nodes {
		nodes[node.ID] = true
	}
	for _, group := range diagram.Groups {
		groups[group.ID] = true
	}
	for _, link := range diagram.Links {
		id := link.StableID()
		if links[id] {
			return nil, nil, nil, fmt.Errorf("links resolve to duplicate stable ID %q; assign explicit unique link IDs", id)
		}
		links[id] = true
	}
	return nodes, groups, links, nil
}

func decodePage(content string) ([]byte, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "<mxGraphModel") {
		return []byte(trimmed), nil
	}
	encoded := strings.TrimSpace(strings.ReplaceAll(trimmed, "\n", ""))
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("page is neither raw mxGraphModel XML nor compressed Draw.io data: %w", err)
	}
	reader := flate.NewReader(bytes.NewReader(compressed))
	inflated, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	decoded, err := url.QueryUnescape(string(inflated))
	if err != nil {
		return nil, err
	}
	return []byte(decoded), nil
}

func extractBounds(cell extractedCell) (layoutoverride.Bounds, error) {
	if cell.Geometry == nil {
		return layoutoverride.Bounds{}, fmt.Errorf("draw.io %s %q has no geometry", cell.NetdiagKind, cell.NetdiagID)
	}
	return layoutoverride.Bounds{
		X:      floatPointer(cell.Geometry.X),
		Y:      floatPointer(cell.Geometry.Y),
		Width:  floatPointer(cell.Geometry.Width),
		Height: floatPointer(cell.Geometry.Height),
		Locked: styleValues(cell.Style)["locked"] == "1",
	}, nil
}

func extractLink(cell extractedCell) layoutoverride.Link {
	values := styleValues(cell.Style)
	result := layoutoverride.Link{
		SourceSide: constraintSide(values["exitX"], values["exitY"]),
		TargetSide: constraintSide(values["entryX"], values["entryY"]),
		Locked:     values["locked"] == "1",
	}
	switch {
	case values["curved"] == "1":
		result.Style = "curved"
	case values["edgeStyle"] == "none":
		result.Style = "straight"
	default:
		result.Style = "orthogonal"
	}
	if cell.Geometry != nil && cell.Geometry.Points != nil && cell.Geometry.Points.As == "points" {
		result.Waypoints = cell.Geometry.Points.Points
	}
	return result
}

func styleValues(style string) map[string]string {
	result := make(map[string]string)
	for _, entry := range strings.Split(style, ";") {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func constraintSide(x, y string) string {
	switch x + "," + y {
	case "0.5,0":
		return "top"
	case "1,0.5":
		return "right"
	case "0.5,1":
		return "bottom"
	case "0,0.5":
		return "left"
	default:
		return ""
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
