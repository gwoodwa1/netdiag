package isis

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gwoodwa1/netdiag/internal/spec"
)

func ToDocumentSet(results []Result) (*spec.Document, error) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Title: "IS-IS discovered topology", Layout: "rows"},
		Nodes:   make(map[string]spec.Node),
	}
	seen := make(map[string]bool)
	for _, result := range results {
		if strings.TrimSpace(result.LocalNode) == "" {
			return nil, fmt.Errorf("local device name is unavailable; provide --local or use a descriptive filename")
		}
		localID := nodeID(result.LocalNode)
		mergeNode(doc.Nodes, localID, spec.Node{
			Label: result.LocalNode, Role: "isis-level-2",
			Metadata: map[string]interface{}{"discovery": "isis", "local": true},
		})
		for _, neighbor := range result.Neighbors {
			if neighbor.SystemID == "" || neighbor.Interface == "" {
				continue
			}
			remoteID := nodeID(neighbor.SystemID)
			metadata := map[string]interface{}{"discovery": "isis", "holdtime": neighbor.Holdtime}
			addMetadata(metadata, "snpa", neighbor.SNPA)
			addMetadata(metadata, "instance", neighbor.Instance)
			addMetadata(metadata, "adjacency_type", neighbor.Type)
			addMetadata(metadata, "ietf_nsf", neighbor.IETFNSF)
			mergeNode(doc.Nodes, remoteID, spec.Node{Label: neighbor.SystemID, Role: roleForType(neighbor.Type), Metadata: metadata})
			remotePort := neighbor.SNPA
			if remotePort == "" {
				remotePort = "isis-adjacency"
			}
			link := spec.Link{
				From:     spec.LinkEndpoint{Node: localID, Port: neighbor.Interface},
				To:       spec.LinkEndpoint{Node: remoteID, Port: remotePort},
				Label:    strings.Trim(strings.Join([]string{neighbor.Instance, neighbor.Type}, " · "), " ·"),
				Protocol: "isis", Status: strings.ToLower(neighbor.State),
			}
			key := linkKey(link)
			if !seen[key] {
				seen[key] = true
				doc.Links = append(doc.Links, link)
			}
		}
	}
	if len(doc.Links) == 0 {
		return nil, fmt.Errorf("no complete IS-IS neighbors found")
	}
	if len(results) == 1 {
		doc.Diagram.Title = "IS-IS topology: " + results[0].LocalNode
	}
	sort.Slice(doc.Links, func(i, j int) bool { return linkKey(doc.Links[i]) < linkKey(doc.Links[j]) })
	return doc, nil
}

func BuildReport(results []Result, doc *spec.Document) Report {
	report := Report{Devices: len(results)}
	for _, result := range results {
		report.Observations += len(result.Neighbors)
	}
	if doc != nil {
		report.Nodes, report.Links = len(doc.Nodes), len(doc.Links)
	}
	if report.Observations > report.Links {
		report.MergedObservations = report.Observations - report.Links
	}
	return report
}

func roleForType(value string) string {
	switch strings.ToUpper(value) {
	case "L1":
		return "isis-level-1"
	default:
		return "isis-level-2"
	}
}

func nodeID(value string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' {
			out.WriteRune(r)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func mergeNode(nodes map[string]spec.Node, id string, incoming spec.Node) {
	existing, ok := nodes[id]
	if !ok {
		nodes[id] = incoming
		return
	}
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]interface{})
	}
	for key, value := range incoming.Metadata {
		existing.Metadata[key] = value
	}
	if existing.Label == "" {
		existing.Label = incoming.Label
	}
	if existing.Role == "" || existing.Role == "router" || incoming.Role == "isis-level-2" {
		existing.Role = incoming.Role
	}
	nodes[id] = existing
}

func linkKey(link spec.Link) string {
	left := link.From.Node + "\x00" + strings.ToLower(link.From.Port)
	right := link.To.Node + "\x00" + strings.ToLower(link.To.Port)
	if right < left {
		left, right = right, left
	}
	return left + "\x01" + right
}

func addMetadata(metadata map[string]interface{}, key, value string) {
	if strings.TrimSpace(value) != "" {
		metadata[key] = value
	}
}
