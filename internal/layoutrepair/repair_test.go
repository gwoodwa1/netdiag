package layoutrepair

import (
	"testing"

	"github.com/gwoodwa1/netdiag/internal/spec"
	"github.com/gwoodwa1/netdiag/internal/svg"
)

func TestImproveRoutesAroundIntermediateNode(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Layout: "rows", LinkStyle: "clean"},
		Nodes: map[string]spec.Node{
			"a": {Role: "router"},
			"m": {Role: "router"},
			"z": {Role: "router"},
		},
		Links: []spec.Link{{
			From: spec.LinkEndpoint{Node: "a", Port: "Ethernet0/0"},
			To:   spec.LinkEndpoint{Node: "z", Port: "Ethernet0/0"},
		}},
	}
	if err := spec.Prepare(doc); err != nil {
		t.Fatal(err)
	}
	improved, report, err := Improve(doc, Options{MaxRounds: 2, MaxCandidates: 20})
	if err != nil {
		t.Fatal(err)
	}
	if report.After.Errors >= report.Before.Errors {
		t.Fatalf("errors did not improve: %+v", report)
	}
	if improved.Diagram.LinkStyle != "orthogonal" {
		t.Fatalf("link style = %q, want orthogonal", improved.Diagram.LinkStyle)
	}
	if doc.Diagram.LinkStyle != "clean" {
		t.Fatal("Improve mutated the input document")
	}
}

func TestImproveLeavesCleanDiagramUnchanged(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Layout: "ring", LinkStyle: "clean"},
		Nodes: map[string]spec.Node{
			"a": {Role: "router"},
			"b": {Role: "router"},
			"c": {Role: "router"},
		},
		Links: []spec.Link{
			{From: spec.LinkEndpoint{Node: "a", Port: "Ethernet0/0"}, To: spec.LinkEndpoint{Node: "b", Port: "Ethernet0/0"}},
			{From: spec.LinkEndpoint{Node: "b", Port: "Ethernet0/1"}, To: spec.LinkEndpoint{Node: "c", Port: "Ethernet0/0"}},
			{From: spec.LinkEndpoint{Node: "c", Port: "Ethernet0/1"}, To: spec.LinkEndpoint{Node: "a", Port: "Ethernet0/1"}},
		},
	}
	if err := spec.Prepare(doc); err != nil {
		t.Fatal(err)
	}
	_, report, err := Improve(doc, Options{MaxRounds: 2, MaxCandidates: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Changes) != 0 || report.Before != report.After {
		t.Fatalf("clean diagram was changed: %+v", report)
	}
}

func TestImproveOrdersNodesByConnectedPeers(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Layout: "rows", LinkStyle: "clean"},
		Nodes: map[string]spec.Node{
			"rr-a": {Role: "route-reflector"},
			"rr-b": {Role: "route-reflector"},
			"c-a":  {Role: "rr-client"},
			"c-b":  {Role: "rr-client"},
		},
		Links: []spec.Link{
			{From: spec.LinkEndpoint{Node: "rr-a", Port: "Ethernet0/0"}, To: spec.LinkEndpoint{Node: "c-b", Port: "Ethernet0/0"}},
			{From: spec.LinkEndpoint{Node: "rr-b", Port: "Ethernet0/0"}, To: spec.LinkEndpoint{Node: "c-a", Port: "Ethernet0/0"}},
		},
	}
	if err := spec.Prepare(doc); err != nil {
		t.Fatal(err)
	}
	improved, report, err := Improve(doc, Options{MaxRounds: 2, MaxCandidates: 30})
	if err != nil {
		t.Fatal(err)
	}
	if report.After.Penalty >= report.Before.Penalty {
		t.Fatalf("peer ordering did not improve the layout: %+v", report)
	}
	if improved.Nodes["rr-b"].Order >= improved.Nodes["rr-a"].Order {
		t.Fatalf("reflectors were not reordered by connected peers: %+v", improved.Nodes)
	}
}

func TestBetterPrioritizesWeightedPenaltyThenErrors(t *testing.T) {
	if better(Score{Penalty: 520, Errors: 1, Warnings: 100}, Score{Penalty: 40, Errors: 2}) {
		t.Fatal("a large warning increase should not win solely because it removes errors")
	}
	if !better(Score{Penalty: 40, Errors: 1, Warnings: 4}, Score{Penalty: 40, Errors: 2}) {
		t.Fatal("fewer errors should win when weighted penalties are equal")
	}
}

func TestProblemLinksPrioritizeUnreadableEndpointLabels(t *testing.T) {
	report := svg.InspectionReport{Findings: []svg.InspectionFinding{
		{Code: "link_through_node", Severity: svg.InspectionError, Links: []int{1}},
		{Code: "link_through_node", Severity: svg.InspectionError, Links: []int{1}},
		{Code: "endpoint_labels_too_close", Severity: svg.InspectionError, Links: []int{9}},
	}}
	links := problemLinkIDs(report, 10)
	if len(links) == 0 || links[0] != 9 {
		t.Fatalf("unreadable endpoint labels were not prioritized: %v", links)
	}
}

func TestTargetedCandidatesGroupUnreadableEndpointLabels(t *testing.T) {
	doc := &spec.Document{Links: []spec.Link{{}, {}, {}}}
	report := svg.InspectionReport{Findings: []svg.InspectionFinding{
		{Code: "endpoint_labels_too_close", Severity: svg.InspectionError, Links: []int{1}},
		{Code: "endpoint_labels_too_close", Severity: svg.InspectionError, Links: []int{3}},
	}}
	candidates := targetedCandidates(doc, report)
	if len(candidates) == 0 {
		t.Fatal("no targeted candidates generated")
	}
	trial := &spec.Document{Links: []spec.Link{{}, {}, {}}}
	candidates[0].apply(trial)
	if trial.Links[0].From.LabelRotation != 90 || trial.Links[0].To.LabelRotation != 90 ||
		trial.Links[2].From.LabelRotation != 90 || trial.Links[2].To.LabelRotation != 90 {
		t.Fatalf("grouped label repair did not rotate all unreadable links: %+v", trial.Links)
	}
}
