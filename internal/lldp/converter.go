package lldp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/spec"
)

func ToDocument(result Result, localNode string) (*spec.Document, error) {
	if strings.TrimSpace(localNode) == "" {
		localNode = result.LocalNode
	}
	if strings.TrimSpace(localNode) == "" {
		return nil, fmt.Errorf("local device name is unavailable; provide --local")
	}
	result.LocalNode = localNode
	return ToDocumentSet([]Result{result})
}

// ToDocumentSet merges LLDP observations from multiple local devices into one
// topology and deduplicates links observed from both ends.
func ToDocumentSet(results []Result) (*spec.Document, error) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Title: "LLDP discovered topology", Layout: "rows"},
		Nodes:   make(map[string]spec.Node),
	}
	seenLinks := make(map[string]bool)
	localCount := 0
	for _, result := range results {
		localNode := strings.TrimSpace(result.LocalNode)
		if localNode == "" {
			return nil, fmt.Errorf("local device name is unavailable; provide --local or use a descriptive filename")
		}
		localCount++
		localID := nodeID(localNode)
		mergeNode(doc.Nodes, localID, spec.Node{
			Label: localNode, Role: "switch",
			Metadata: map[string]interface{}{"discovery": "lldp", "local": true},
		})
		for _, neighbor := range result.Neighbors {
			identity := firstUseful(neighbor.SystemName, neighbor.ChassisID, neighbor.ManagementAddress)
			if identity == "" || neighbor.LocalPort == "" || neighbor.PortID == "" {
				continue
			}
			remoteID := nodeID(identity)
			metadata := map[string]interface{}{"discovery": "lldp"}
			addMetadata(metadata, "chassis_id", neighbor.ChassisID)
			addMetadata(metadata, "management_address", neighbor.ManagementAddress)
			addMetadata(metadata, "system_description", neighbor.SystemDescription)
			addMetadata(metadata, "capabilities", neighbor.Capabilities)
			mergeNode(doc.Nodes, remoteID, spec.Node{Label: identity, Role: inferRole(neighbor), Metadata: metadata})

			link := spec.Link{
				From:     spec.LinkEndpoint{Node: localID, Port: neighbor.LocalPort},
				To:       spec.LinkEndpoint{Node: remoteID, Port: neighbor.PortID},
				Label:    neighbor.PortDescription,
				Protocol: "lldp",
			}
			key := linkKey(link)
			if !seenLinks[key] {
				seenLinks[key] = true
				doc.Links = append(doc.Links, link)
			}
		}
	}
	if len(doc.Links) == 0 {
		return nil, fmt.Errorf("no complete LLDP neighbors found")
	}
	if localCount == 1 {
		for _, result := range results {
			doc.Diagram.Title = "LLDP topology: " + result.LocalNode
		}
	}
	sort.Slice(doc.Links, func(i, j int) bool {
		return linkKey(doc.Links[i]) < linkKey(doc.Links[j])
	})
	return doc, nil
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
	if incoming.Role == "router" || existing.Role == "" || existing.Role == "device" {
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

func inferRole(neighbor Neighbor) string {
	capabilities := strings.FieldsFunc(strings.ToLower(neighbor.Capabilities), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == ';'
	})
	for _, capability := range capabilities {
		if capability == "r" || capability == "router" {
			return "router"
		}
	}
	for _, capability := range capabilities {
		if capability == "b" || capability == "bridge" {
			return "switch"
		}
	}
	value := strings.ToLower(neighbor.SystemDescription)
	switch {
	case strings.Contains(value, "router"):
		return "router"
	case strings.Contains(value, "bridge") || strings.Contains(value, "switch"):
		return "switch"
	case strings.Contains(value, "server") || strings.Contains(value, "station"):
		return "server"
	default:
		return "device"
	}
}

func addMetadata(metadata map[string]interface{}, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}
