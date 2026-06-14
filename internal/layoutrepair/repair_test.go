package layoutrepair

import (
	"testing"

	"github.com/gwoodwa1/netdiag/internal/spec"
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

func TestBetterPrioritizesErrorsThenWarnings(t *testing.T) {
	if !better(Score{Errors: 1, Warnings: 100}, Score{Errors: 2}) {
		t.Fatal("fewer errors should win despite additional warnings")
	}
	if better(Score{Errors: 1, Warnings: 5}, Score{Errors: 1, Warnings: 4}) {
		t.Fatal("more warnings should not win when errors are equal")
	}
}
