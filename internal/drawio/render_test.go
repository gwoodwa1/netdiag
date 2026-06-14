package drawio

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/layoutoverride"
	"github.com/gwoodwa1/netdiag/internal/model"
)

func TestRenderProducesEditableDrawIOGraph(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Title: `Core & Edge`},
		Groups: []model.Group{
			{ID: "site", Label: "London", NodeIDs: []string{"pe-1", "fw-1"}},
			{ID: "rack", Label: "Rack A", ParentID: "site"},
		},
		Nodes: []model.Node{
			{ID: "pe-1", Label: "PE-1", Role: "edge-router"},
			{ID: "fw-1", Label: "FW-1", Role: "firewall"},
		},
		Links: []model.Link{{
			From:   model.LinkEndpoint{Node: "pe-1", Port: "Te0/0/0/1", Address: "192.0.2.1/31"},
			To:     model.LinkEndpoint{Node: "fw-1", Port: "Eth1/1", Address: "192.0.2.0/31"},
			Labels: model.LinkLabels{Middle: "100G"},
		}},
	}

	got, err := Render(diagram)
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(got, new(interface{})); err != nil {
		t.Fatalf("invalid draw.io XML: %v", err)
	}
	stableID := diagram.Links[0].StableID()
	for _, want := range []string{
		`<mxfile `,
		`name="Core &amp; Edge"`,
		`id="group-site"`,
		`netdiag-id="site"`,
		`id="group-rack"`,
		`id="node-pe-1"`,
		`netdiag-kind="node"`,
		`shape=mxgraph.cisco19.router`,
		`source="node-pe-1"`,
		`target="node-fw-1"`,
		`id="` + linkCellID(stableID) + `"`,
		`netdiag-id="` + stableID + `"`,
		`id="` + labelCellID(stableID, "source") + `"`,
		`Te0/0/0/1 192.0.2.1/31`,
		`id="` + labelCellID(stableID, "target") + `"`,
		`Eth1/1 192.0.2.0/31`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("draw.io output does not contain %q", want)
		}
	}
	if strings.Index(string(got), `id="group-site"`) > strings.Index(string(got), `id="group-rack"`) {
		t.Error("parent group must be emitted before nested group")
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "b", Role: "switch"}, {ID: "a", Role: "router"}},
	}
	first, err := Render(diagram)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(diagram)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("draw.io output changed between identical renders")
	}
}

func TestRenderUsesStableAndExplicitLinkIDs(t *testing.T) {
	first := model.Link{
		From: model.LinkEndpoint{Node: "a", Port: "Eth1"},
		To:   model.LinkEndpoint{Node: "b", Port: "Eth2"},
	}
	reversed := model.Link{From: first.To, To: first.From}
	if first.StableID() != reversed.StableID() {
		t.Fatal("automatic link ID changed when link direction changed")
	}

	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{{ID: "core-link", From: first.From, To: first.To}},
	}
	got, err := Render(diagram)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="link-core-link"`, `netdiag-id="core-link"`, `netdiag-kind="link"`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("draw.io output does not contain %q", want)
		}
	}
}

func TestRenderAppliesLayoutOverrides(t *testing.T) {
	x, y, width := 20.0, 30.0, 220.0
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{{
			ID: "core-link", From: model.LinkEndpoint{Node: "a"}, To: model.LinkEndpoint{Node: "b"},
		}},
	}
	overrides := &layoutoverride.Document{
		Version: 1,
		LayoutOverrides: layoutoverride.Overrides{
			Nodes: map[string]layoutoverride.Bounds{"a": {X: &x, Y: &y, Width: &width, Locked: true}},
			Links: map[string]layoutoverride.Link{"core-link": {
				SourceSide: "right", TargetSide: "left", Style: "straight", Locked: true,
				Waypoints: []layoutoverride.Point{{X: 0, Y: 125}, {X: 400, Y: 125}},
			}},
		},
	}
	got, err := RenderWithOptions(diagram, Options{Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`id="node-a"`, `x="20" y="30" width="220"`, `locked=1;`,
		`edgeStyle=none;`, `exitX=1;exitY=0.5;exitPerimeter=0;`,
		`entryX=0;entryY=0.5;entryPerimeter=0;`,
		`<Array as="points"><mxPoint x="0" y="125"></mxPoint><mxPoint x="400" y="125"></mxPoint></Array>`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("draw.io output does not contain %q\n%s", want, got)
		}
	}
}

func TestRenderRejectsUnknownOverrideReference(t *testing.T) {
	diagram := &model.Diagram{Nodes: []model.Node{{ID: "a"}}}
	overrides := &layoutoverride.Document{
		Version:         1,
		LayoutOverrides: layoutoverride.Overrides{Nodes: map[string]layoutoverride.Bounds{"missing": {}}},
	}
	if _, err := RenderWithOptions(diagram, Options{Overrides: overrides}); err == nil {
		t.Fatal("unknown override node was accepted")
	}
}

func TestRenderRejectsDuplicateAutomaticLinkIDs(t *testing.T) {
	link := model.Link{From: model.LinkEndpoint{Node: "a"}, To: model.LinkEndpoint{Node: "b"}}
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{link, link},
	}
	if _, err := Render(diagram); err == nil {
		t.Fatal("duplicate automatic link IDs were accepted")
	}
}

func TestRenderValidatesOverrides(t *testing.T) {
	diagram := &model.Diagram{Nodes: []model.Node{{ID: "a"}}}
	overrides := &layoutoverride.Document{
		Version:         1,
		LayoutOverrides: layoutoverride.Overrides{Links: map[string]layoutoverride.Link{"link": {SourceSide: "east"}}},
	}
	if _, err := RenderWithOptions(diagram, Options{Overrides: overrides}); err == nil {
		t.Fatal("invalid overrides were accepted")
	}
}

func TestStyleForUnknownRoleUsesGenericShape(t *testing.T) {
	style := styleForRole("unknown-role")
	if style.Shape != "rectangle" {
		t.Fatalf("shape = %q, want rectangle", style.Shape)
	}
}
