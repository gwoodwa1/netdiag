package d2backend

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/model"
)

func TestSourceCoversNetworkHardCases(t *testing.T) {
	diagram := &model.Diagram{
		Groups: []model.Group{
			{ID: "site", Label: "Site"},
			{ID: "rack", Label: "Rack", ParentID: "site", NodeIDs: []string{"a"}},
		},
		Nodes: []model.Node{
			{ID: "a", Label: "Router A", Role: "router"},
			{ID: "b", Label: "Router B", Role: "router"},
		},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "a", Port: "Ethernet0/0"}, To: model.LinkEndpoint{Node: "b", Port: "Ethernet0/1"}, Label: "Primary"},
			{From: model.LinkEndpoint{Node: "a", Port: "Ethernet0/2"}, To: model.LinkEndpoint{Node: "b", Port: "Ethernet0/3"}, Label: "Backup"},
		},
	}
	source := Source(diagram)
	for _, want := range []string{"site:", "rack:", "site.rack.a -> b", `source-arrowhead.label: "Eth0/0"`, `target-arrowhead.label: "Eth0/1"`, `"Primary"`, `"Backup"`} {
		if !strings.Contains(source, want) {
			t.Fatalf("D2 source missing %q:\n%s", want, source)
		}
	}
}

func TestELKOutputIsDeterministic(t *testing.T) {
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "a", Role: "router"}, {ID: "b", Role: "router"}},
		Links: []model.Link{{From: model.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: model.LinkEndpoint{Node: "b", Port: "Eth0/1"}, Label: "10G"}},
	}
	first, err := Render(diagram, Options{Layout: "elk"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(diagram, Options{Layout: "elk"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("D2/ELK output changed across identical renders")
	}
}
