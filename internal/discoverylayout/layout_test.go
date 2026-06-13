package discoverylayout

import (
	"fmt"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/spec"
)

func TestApplyGroupsLargeTopologyByHostnamePrefix(t *testing.T) {
	doc := &spec.Document{Version: 1, Nodes: make(map[string]spec.Node)}
	for pod := 1; pod <= 4; pod++ {
		for node := 1; node <= 10; node++ {
			id := fmt.Sprintf("n55%d-%02d", pod, node)
			doc.Nodes[id] = spec.Node{Label: fmt.Sprintf("N55%d-%02d", pod, node), Role: "isis-level-2"}
		}
	}
	for index := 0; index < 10; index++ {
		doc.Links = append(doc.Links, spec.Link{
			From:  spec.LinkEndpoint{Node: "n551-01", Port: fmt.Sprintf("Te0/0/0/%d", index)},
			To:    spec.LinkEndpoint{Node: "n552-01", Port: fmt.Sprintf("Te0/0/1/%d", index)},
			Label: "CORE · L2",
		})
	}

	report := Apply(doc)
	if report.Layout != "sites" || report.Grouping != "hostname-prefix" || report.Groups != 4 {
		t.Fatalf("unexpected auto-layout report: %+v", report)
	}
	if len(doc.Groups) != 4 || doc.Diagram.LinkStyle != "orthogonal" || doc.Diagram.InterfaceAt != "ends" {
		t.Fatalf("unexpected auto-layout document: %+v", doc.Diagram)
	}
	if report.SuppressedMiddleLabels != 10 || doc.Links[0].Label != "" {
		t.Fatalf("repeated middle labels were not suppressed: %+v", report)
	}
}

func TestApplyUsesRingForSmallCycle(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"a": {Role: "isis-level-2"}, "b": {Role: "isis-level-2"}, "c": {Role: "isis-level-2"},
		},
		Links: []spec.Link{
			{From: spec.LinkEndpoint{Node: "a"}, To: spec.LinkEndpoint{Node: "b"}},
			{From: spec.LinkEndpoint{Node: "b"}, To: spec.LinkEndpoint{Node: "c"}},
			{From: spec.LinkEndpoint{Node: "c"}, To: spec.LinkEndpoint{Node: "a"}},
		},
	}
	report := Apply(doc)
	if report.Layout != "ring" || doc.Diagram.Layout != "ring" {
		t.Fatalf("small cycle did not select ring layout: %+v", report)
	}
}

func TestApplyUsesBalancedGroupsWithoutHostnamePrefixes(t *testing.T) {
	doc := &spec.Document{Version: 1, Nodes: make(map[string]spec.Node)}
	for index := 1; index <= 25; index++ {
		id := fmt.Sprintf("router%d", index)
		doc.Nodes[id] = spec.Node{Label: id, Role: "router"}
	}
	report := Apply(doc)
	if report.Layout != "sites" || report.Grouping != "balanced" || report.Groups != 3 {
		t.Fatalf("unexpected balanced auto-layout report: %+v", report)
	}
	if len(doc.Groups["cluster-01"].Nodes) != 10 || len(doc.Groups["cluster-03"].Nodes) != 5 {
		t.Fatalf("unexpected balanced group sizes: %+v", doc.Groups)
	}
}

func TestApplyDisambiguatesNormalizedHostnameGroupIDs(t *testing.T) {
	doc := &spec.Document{Version: 1, Nodes: make(map[string]spec.Node)}
	for index := 1; index <= 10; index++ {
		doc.Nodes[fmt.Sprintf("space-%02d", index)] = spec.Node{Label: fmt.Sprintf("LON CORE-%02d", index), Role: "router"}
		doc.Nodes[fmt.Sprintf("dash-%02d", index)] = spec.Node{Label: fmt.Sprintf("lon-core-%02d", index), Role: "router"}
	}
	report := Apply(doc)
	if report.Grouping != "hostname-prefix" || len(doc.Groups) != 2 {
		t.Fatalf("normalized hostname groups collided: report=%+v groups=%+v", report, doc.Groups)
	}
	if doc.Groups["lon-core"] == nil || doc.Groups["lon-core-2"] == nil {
		t.Fatalf("expected deterministic collision suffix: %+v", doc.Groups)
	}
}

func TestApplyGroupsNumberedNetworkRolesBySiteAndPlane(t *testing.T) {
	doc := &spec.Document{Version: 1, Nodes: make(map[string]spec.Node)}
	for _, prefix := range []string{"NYC", "WAS", "PHX", "LAX", "ORL", "SFO", "SAN", "DAL"} {
		doc.Nodes[prefix+"-PE1"] = spec.Node{Label: prefix + "-PE1", Role: "router"}
		doc.Nodes[prefix+"-PE2"] = spec.Node{Label: prefix + "-PE2", Role: "router"}
	}
	for _, prefix := range []string{"CORE-A", "CORE-B"} {
		doc.Nodes[prefix+"-P1"] = spec.Node{Label: prefix + "-P1", Role: "router"}
		doc.Nodes[prefix+"-P2"] = spec.Node{Label: prefix + "-P2", Role: "router"}
	}

	report := Apply(doc)
	if report.Grouping != "hostname-prefix" || len(doc.Groups) != 10 {
		t.Fatalf("numbered network roles were not grouped semantically: report=%+v groups=%+v", report, doc.Groups)
	}
	if doc.Groups["nyc"] == nil || doc.Groups["core-a"] == nil || len(doc.Groups["nyc"].Nodes) != 2 {
		t.Fatalf("missing expected site or plane group: %+v", doc.Groups)
	}
}

func TestApplySelectsHubSpokeForCoreAndSites(t *testing.T) {
	doc := &spec.Document{Version: 1, Nodes: make(map[string]spec.Node)}
	for _, prefix := range []string{"NYC", "WAS", "PHX", "LAX", "ORL", "SFO", "SAN", "DAL"} {
		doc.Nodes[prefix+"-PE1"] = spec.Node{Label: prefix + "-PE1", Role: "edge-router"}
		doc.Nodes[prefix+"-PE2"] = spec.Node{Label: prefix + "-PE2", Role: "edge-router"}
	}
	for _, prefix := range []string{"CORE-A", "CORE-B"} {
		doc.Nodes[prefix+"-P1"] = spec.Node{Label: prefix + "-P1", Role: "core-router"}
		doc.Nodes[prefix+"-P2"] = spec.Node{Label: prefix + "-P2", Role: "core-router"}
	}

	report := Apply(doc)
	if report.Layout != "hub-spoke" || doc.Diagram.Layout != "hub-spoke" {
		t.Fatalf("core-and-site topology did not select hub-spoke: %+v", report)
	}
}
