package svg

import (
	"reflect"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/model"
)

func TestInspectCleanDiagram(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "rows"},
		Nodes: []model.Node{{ID: "a", Role: "router"}, {ID: "b", Role: "router"}},
		Links: []model.Link{{
			From: model.LinkEndpoint{Node: "a", Port: "Eth0/0"},
			To:   model.LinkEndpoint{Node: "b", Port: "Eth0/0"},
		}},
	}
	report, err := Inspect(diagram)
	if err != nil {
		t.Fatal(err)
	}
	if report.Score != 100 || len(report.Findings) != 0 {
		t.Fatalf("unexpected clean report: %+v", report)
	}
}

func TestInspectFindsCrossingLinks(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "ring", InterfaceLabels: "none"},
		Nodes: []model.Node{
			{ID: "a", Role: "router"}, {ID: "b", Role: "router"},
			{ID: "c", Role: "router"}, {ID: "d", Role: "router"},
		},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "a"}, To: model.LinkEndpoint{Node: "c"}},
			{From: model.LinkEndpoint{Node: "b"}, To: model.LinkEndpoint{Node: "d"}},
		},
	}
	report, err := Inspect(diagram)
	if err != nil {
		t.Fatal(err)
	}
	if !containsInspectionFinding(report, "link_crossing") {
		t.Fatalf("crossing was not reported: %+v", report)
	}
	if !report.HasAtLeast(InspectionWarning) || report.HasAtLeast(InspectionError) {
		t.Fatalf("unexpected severity threshold result: %+v", report)
	}
}

func TestInspectIsDeterministic(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "ring", InterfaceLabels: "none"},
		Nodes: []model.Node{{ID: "b", Role: "router"}, {ID: "a", Role: "router"}, {ID: "c", Role: "router"}},
		Links: []model.Link{{From: model.LinkEndpoint{Node: "a"}, To: model.LinkEndpoint{Node: "b"}}},
	}
	first, err := Inspect(diagram)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Inspect(diagram)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("inspection changed across identical runs:\n%+v\n%+v", first, second)
	}
}

func TestInspectEndpointCrowding(t *testing.T) {
	diagram := &model.Diagram{Links: []model.Link{
		{From: model.LinkEndpoint{Node: "core"}, To: model.LinkEndpoint{Node: "a"}},
		{From: model.LinkEndpoint{Node: "core"}, To: model.LinkEndpoint{Node: "b"}},
	}}
	geometry := map[string]endpointGeometry{
		endpointKey(0, true):  {Point: point{X: 100, Y: 100}, Side: "right"},
		endpointKey(0, false): {Point: point{X: 300, Y: 100}, Side: "left"},
		endpointKey(1, true):  {Point: point{X: 100, Y: 110}, Side: "right"},
		endpointKey(1, false): {Point: point{X: 300, Y: 110}, Side: "left"},
	}
	findings := inspectEndpointCrowding(diagram, geometry)
	if len(findings) != 1 || findings[0].Code != "crowded_endpoints" {
		t.Fatalf("unexpected crowded endpoint findings: %+v", findings)
	}
}

func TestInspectFindsOverlappingInterfaceLabels(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "rows"},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "a", Port: "Ethernet0/0"}, To: model.LinkEndpoint{Node: "b", Port: "Ethernet0/0"}},
			{From: model.LinkEndpoint{Node: "c", Port: "Ethernet0/1"}, To: model.LinkEndpoint{Node: "d", Port: "Ethernet0/1"}},
		},
	}
	geometry := map[string]endpointGeometry{
		endpointKey(0, true):  {Point: point{X: 100, Y: 100}, Side: "top"},
		endpointKey(0, false): {Point: point{X: 500, Y: 100}, Side: "top"},
		endpointKey(1, true):  {Point: point{X: 100, Y: 100}, Side: "top"},
		endpointKey(1, false): {Point: point{X: 700, Y: 100}, Side: "top"},
	}
	routes := map[int]linkRoute{
		0: directRoute(point{X: 100, Y: 100}, point{X: 500, Y: 100}, "top", "top", ""),
		1: directRoute(point{X: 100, Y: 100}, point{X: 700, Y: 100}, "top", "top", ""),
	}
	findings := inspectLabels(diagram, routes, geometry, map[string]placedNode{}, 1000, 1000)
	found := false
	for _, finding := range findings {
		found = found || finding.Code == "label_overlap"
	}
	if !found {
		t.Fatalf("label overlap was not reported: %+v", findings)
	}
}

func TestInspectFindsLinkThroughInterfaceLabel(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "rows"},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "a", Port: "Ethernet0/0"}, To: model.LinkEndpoint{Node: "b"}},
			{From: model.LinkEndpoint{Node: "c"}, To: model.LinkEndpoint{Node: "d"}},
		},
	}
	geometry := map[string]endpointGeometry{
		endpointKey(0, true):  {Point: point{X: 100, Y: 100}, Side: "top"},
		endpointKey(0, false): {Point: point{X: 500, Y: 100}, Side: "top"},
		endpointKey(1, true):  {Point: point{X: 50, Y: 88}, Side: "right"},
		endpointKey(1, false): {Point: point{X: 150, Y: 88}, Side: "left"},
	}
	routes := map[int]linkRoute{
		0: directRoute(point{X: 100, Y: 100}, point{X: 500, Y: 100}, "top", "top", ""),
		1: directRoute(point{X: 50, Y: 88}, point{X: 150, Y: 88}, "right", "left", ""),
	}
	findings := inspectLabels(diagram, routes, geometry, map[string]placedNode{}, 1000, 1000)
	found := false
	for _, finding := range findings {
		found = found || finding.Code == "label_link_overlap"
	}
	if !found {
		t.Fatalf("link through interface label was not reported: %+v", findings)
	}
}

func containsInspectionFinding(report InspectionReport, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
