package drawio

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
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

type extractedCell struct {
	Style       string             `xml:"style,attr"`
	Vertex      string             `xml:"vertex,attr"`
	Edge        string             `xml:"edge,attr"`
	NetdiagID   string             `xml:"netdiag-id,attr"`
	NetdiagKind string             `xml:"netdiag-kind,attr"`
	Geometry    *extractedGeometry `xml:"mxGeometry"`
}

type extractedObject struct {
	NetdiagID   string        `xml:"netdiag-id,attr"`
	NetdiagKind string        `xml:"netdiag-kind,attr"`
	Cell        extractedCell `xml:"mxCell"`
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
	Points []extractedPoint `xml:"mxPoint"`
}

type extractedPoint struct {
	X float64 `xml:"x,attr"`
	Y float64 `xml:"y,attr"`
}

type ExtractionReport struct {
	Managed  ExtractionManaged
	Ignored  ExtractionIgnored
	Warnings []string
}

type ExtractionManaged struct {
	Nodes  int
	Groups int
	Links  int
}

type ExtractionIgnored struct {
	Annotations      int
	DecorativeShapes int
	Connectors       int
	UnknownManaged   int
}

func ExtractOverrides(data []byte, diagram *model.Diagram) (*layoutoverride.Document, error) {
	result, _, err := extractOverrides(data, diagram, true)
	return result, err
}

func ExtractOverridesWithReport(data []byte, diagram *model.Diagram) (*layoutoverride.Document, ExtractionReport, error) {
	return extractOverrides(data, diagram, false)
}

func extractOverrides(data []byte, diagram *model.Diagram, strict bool) (*layoutoverride.Document, ExtractionReport, error) {
	var report ExtractionReport
	var file drawioFile
	if err := xml.Unmarshal(data, &file); err != nil {
		return nil, report, fmt.Errorf("parse draw.io file: %w", err)
	}
	if len(file.Diagrams) == 0 {
		return nil, report, fmt.Errorf("draw.io file contains no diagram pages")
	}

	nodes, groups, links, err := topologyIDs(diagram)
	if err != nil {
		return nil, report, err
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
			return nil, report, fmt.Errorf("decode draw.io page %d: %w", pageIndex+1, err)
		}
		cells, err := extractCells(graphData)
		if err != nil {
			return nil, report, fmt.Errorf("parse draw.io page %d graph: %w", pageIndex+1, err)
		}
		for _, cell := range cells {
			if cell.NetdiagID == "" || cell.NetdiagKind == "" {
				classifyIgnoredCell(cell, &report)
				continue
			}
			key := cell.NetdiagKind + ":" + cell.NetdiagID
			if seen[key] {
				return nil, report, fmt.Errorf("draw.io file contains duplicate %s ID %q", cell.NetdiagKind, cell.NetdiagID)
			}
			seen[key] = true
			switch cell.NetdiagKind {
			case "node":
				if !nodes[cell.NetdiagID] {
					if strict {
						return nil, report, fmt.Errorf("draw.io file references unknown node %q", cell.NetdiagID)
					}
					reportUnknownManaged(&report, "node", cell.NetdiagID)
					continue
				}
				bounds, err := extractBounds(cell)
				if err != nil {
					return nil, report, err
				}
				result.LayoutOverrides.Nodes[cell.NetdiagID] = bounds
				report.Managed.Nodes++
			case "group":
				if !groups[cell.NetdiagID] {
					if strict {
						return nil, report, fmt.Errorf("draw.io file references unknown group %q", cell.NetdiagID)
					}
					reportUnknownManaged(&report, "group", cell.NetdiagID)
					continue
				}
				bounds, err := extractBounds(cell)
				if err != nil {
					return nil, report, err
				}
				result.LayoutOverrides.Groups[cell.NetdiagID] = bounds
				report.Managed.Groups++
			case "link":
				if !links[cell.NetdiagID] {
					if strict {
						return nil, report, fmt.Errorf("draw.io file references unknown link %q", cell.NetdiagID)
					}
					reportUnknownManaged(&report, "link", cell.NetdiagID)
					continue
				}
				result.LayoutOverrides.Links[cell.NetdiagID] = extractLink(cell)
				report.Managed.Links++
			}
		}
	}
	if err := layoutoverride.Validate(result); err != nil {
		return nil, report, fmt.Errorf("validate extracted layout overrides: %w", err)
	}
	if !strict {
		appendMissingWarnings(&report, "node", nodes, result.LayoutOverrides.Nodes)
		appendMissingWarnings(&report, "group", groups, result.LayoutOverrides.Groups)
		appendMissingWarnings(&report, "link", links, result.LayoutOverrides.Links)
		sort.Strings(report.Warnings)
	}
	return result, report, nil
}

func FormatExtractionReport(report ExtractionReport) string {
	return fmt.Sprintf(`Managed objects:
- %d nodes extracted
- %d groups extracted
- %d links extracted

Ignored objects:
- %d unmanaged annotations
- %d decorative shapes
- %d manually added connectors without netdiag-id
- %d managed objects not present in source

Warnings:
%s`, report.Managed.Nodes, report.Managed.Groups, report.Managed.Links,
		report.Ignored.Annotations, report.Ignored.DecorativeShapes, report.Ignored.Connectors, report.Ignored.UnknownManaged,
		formatWarnings(report.Warnings))
}

func classifyIgnoredCell(cell extractedCell, report *ExtractionReport) {
	switch {
	case cell.Edge == "1":
		report.Ignored.Connectors++
	case cell.Vertex == "1" && hasStyleFlag(cell.Style, "text"):
		report.Ignored.Annotations++
	case cell.Vertex == "1":
		report.Ignored.DecorativeShapes++
	}
}

func reportUnknownManaged(report *ExtractionReport, kind, id string) {
	report.Ignored.UnknownManaged++
	report.Warnings = append(report.Warnings, fmt.Sprintf("%s %s exists in draw.io but source topology no longer contains it", kind, id))
}

func appendMissingWarnings[T any](report *ExtractionReport, kind string, expected map[string]bool, extracted map[string]T) {
	for id := range expected {
		if _, ok := extracted[id]; !ok {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s %s exists in source but was not found in draw.io", kind, id))
		}
	}
}

func formatWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return "- none\n"
	}
	var result strings.Builder
	for _, warning := range warnings {
		fmt.Fprintf(&result, "- %s\n", warning)
	}
	return result.String()
}

func hasStyleFlag(style, flag string) bool {
	for _, entry := range strings.Split(style, ";") {
		if entry == flag {
			return true
		}
	}
	return false
}

func extractCells(data []byte) ([]extractedCell, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var cells []extractedCell
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return cells, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "mxCell":
			var cell extractedCell
			if err := decoder.DecodeElement(&cell, &start); err != nil {
				return nil, err
			}
			cells = append(cells, cell)
		case "object", "UserObject":
			var object extractedObject
			if err := decoder.DecodeElement(&object, &start); err != nil {
				return nil, err
			}
			if object.Cell.NetdiagID == "" {
				object.Cell.NetdiagID = object.NetdiagID
			}
			if object.Cell.NetdiagKind == "" {
				object.Cell.NetdiagKind = object.NetdiagKind
			}
			cells = append(cells, object.Cell)
		}
	}
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
	}
	if cell.Geometry != nil && cell.Geometry.Points != nil && cell.Geometry.Points.As == "points" {
		result.Waypoints = make([]layoutoverride.Point, len(cell.Geometry.Points.Points))
		for index, point := range cell.Geometry.Points.Points {
			result.Waypoints[index] = layoutoverride.Point{X: point.X, Y: point.Y}
		}
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
