package layoutoverride

import (
	"testing"

	"github.com/gwoodwa1/netdiag/internal/constraint"
)

func TestValidateAcceptsSupportedOverrides(t *testing.T) {
	width := 300.0
	doc := &Document{
		Version: 1,
		LayoutOverrides: Overrides{
			Nodes: map[string]Bounds{"core-a": {Width: &width, Locked: true}},
			Links: map[string]Link{"core-link": {
				SourceSide: "right", TargetSide: "left", Style: "orthogonal",
				Waypoints: []Point{{X: 500, Y: 200}},
			}},
		},
	}
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnsupportedSide(t *testing.T) {
	doc := &Document{
		Version:         1,
		LayoutOverrides: Overrides{Links: map[string]Link{"core-link": {SourceSide: "east"}}},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("unsupported side was accepted")
	}
}

func TestApplyConstraintsAddsManualIntent(t *testing.T) {
	x, width := 0.0, 320.0
	doc := &Document{Version: 1, LayoutOverrides: Overrides{
		Nodes: map[string]Bounds{"core-a": {X: &x, Width: &width, Locked: true}},
		Links: map[string]Link{"core-link": {SourceSide: "right", Waypoints: []Point{{X: 10, Y: 20}}, Locked: true}},
	}}
	base := constraint.Set{Ports: []constraint.Port{{LinkID: "core-link", Endpoint: constraint.Source, NodeID: "core-a", Side: "top"}}}
	got := doc.ApplyConstraints(base)
	if len(got.Geometry) != 1 || !got.Geometry[0].X.Set || got.Geometry[0].X.Value != 0 || !got.Geometry[0].Locked {
		t.Fatalf("unexpected geometry constraint: %+v", got.Geometry)
	}
	if got.Ports[0].Side != "right" || got.Ports[0].NodeID != "core-a" {
		t.Fatalf("unexpected port constraint: %+v", got.Ports[0])
	}
	if len(got.Routes) != 1 || got.Routes[0].Waypoints[0] != (constraint.Point{X: 10, Y: 20}) {
		t.Fatalf("unexpected route constraint: %+v", got.Routes)
	}
}
