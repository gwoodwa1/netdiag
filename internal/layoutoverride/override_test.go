package layoutoverride

import "testing"

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
