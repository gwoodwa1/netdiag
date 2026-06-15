package drawio

import (
	"encoding/xml"
	"fmt"
	"sort"
)

type DoctorReport struct {
	RoundTripSafe bool          `json:"round_trip_safe"`
	Pages         int           `json:"pages"`
	Managed       DoctorManaged `json:"managed"`
	Unmanaged     DoctorIgnored `json:"unmanaged"`
	Problems      []string      `json:"problems,omitempty"`
	Warnings      []string      `json:"warnings,omitempty"`
}

type DoctorManaged struct {
	Nodes  int `json:"nodes"`
	Groups int `json:"groups"`
	Links  int `json:"links"`
	Labels int `json:"labels"`
}

type DoctorIgnored struct {
	Annotations      int `json:"annotations"`
	DecorativeShapes int `json:"decorative_shapes"`
	Connectors       int `json:"connectors"`
}

func Doctor(data []byte) (DoctorReport, error) {
	var file drawioFile
	if err := xml.Unmarshal(data, &file); err != nil {
		return DoctorReport{}, fmt.Errorf("parse draw.io file: %w", err)
	}
	if len(file.Diagrams) == 0 {
		return DoctorReport{}, fmt.Errorf("draw.io file contains no diagram pages")
	}
	report := DoctorReport{Pages: len(file.Diagrams)}
	seen := make(map[string]bool)
	for pageIndex, page := range file.Diagrams {
		graphData, err := decodePage(page.Content)
		if err != nil {
			return DoctorReport{}, fmt.Errorf("decode draw.io page %d: %w", pageIndex+1, err)
		}
		cells, err := extractCells(graphData)
		if err != nil {
			return DoctorReport{}, fmt.Errorf("parse draw.io page %d graph: %w", pageIndex+1, err)
		}
		for _, cell := range cells {
			if cell.NetdiagID == "" && cell.NetdiagKind == "" {
				classifyDoctorUnmanaged(cell, &report)
				continue
			}
			if cell.NetdiagID == "" || cell.NetdiagKind == "" {
				report.Problems = append(report.Problems, "managed cell has incomplete netdiag-id/netdiag-kind metadata")
				continue
			}
			key := cell.NetdiagKind + ":" + cell.NetdiagID
			if seen[key] {
				report.Problems = append(report.Problems, fmt.Sprintf("duplicate %s ID %q", cell.NetdiagKind, cell.NetdiagID))
				continue
			}
			seen[key] = true
			switch cell.NetdiagKind {
			case "node":
				report.Managed.Nodes++
				if cell.Geometry == nil {
					report.Problems = append(report.Problems, fmt.Sprintf("node %q has no geometry", cell.NetdiagID))
				}
			case "group":
				report.Managed.Groups++
				if cell.Geometry == nil {
					report.Problems = append(report.Problems, fmt.Sprintf("group %q has no geometry", cell.NetdiagID))
				}
			case "link":
				report.Managed.Links++
			case "label":
				report.Managed.Labels++
			default:
				report.Warnings = append(report.Warnings, fmt.Sprintf("unknown managed kind %q for ID %q", cell.NetdiagKind, cell.NetdiagID))
			}
		}
	}
	if report.Managed.Nodes+report.Managed.Groups+report.Managed.Links == 0 {
		report.Problems = append(report.Problems, "no netdiag-managed nodes, groups, or links found")
	}
	if report.Unmanaged.Annotations+report.Unmanaged.DecorativeShapes+report.Unmanaged.Connectors > 0 {
		report.Warnings = append(report.Warnings, "unmanaged Draw.io objects are not preserved by regeneration")
	}
	sort.Strings(report.Problems)
	sort.Strings(report.Warnings)
	report.RoundTripSafe = len(report.Problems) == 0
	return report, nil
}

func classifyDoctorUnmanaged(cell extractedCell, report *DoctorReport) {
	switch {
	case cell.Edge == "1":
		report.Unmanaged.Connectors++
	case cell.Vertex == "1" && hasStyleFlag(cell.Style, "text"):
		report.Unmanaged.Annotations++
	case cell.Vertex == "1":
		report.Unmanaged.DecorativeShapes++
	}
}
