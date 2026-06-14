package drawio

import (
	"encoding/xml"
	"strings"
	"testing"

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
	for _, want := range []string{
		`<mxfile `,
		`name="Core &amp; Edge"`,
		`id="group-site"`,
		`id="group-rack"`,
		`id="node-pe-1"`,
		`shape=mxgraph.cisco19.router`,
		`source="node-pe-1"`,
		`target="node-fw-1"`,
		`id="link-1-source"`,
		`Te0/0/0/1 192.0.2.1/31`,
		`id="link-1-target"`,
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

func TestStyleForUnknownRoleUsesGenericShape(t *testing.T) {
	style := styleForRole("unknown-role")
	if style.Shape != "rectangle" {
		t.Fatalf("shape = %q, want rectangle", style.Shape)
	}
}
