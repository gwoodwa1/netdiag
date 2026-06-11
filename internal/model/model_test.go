package model

import (
	"testing"

	"github.com/gwoodwa1/netdiag/internal/spec"
	"gopkg.in/yaml.v3"
)

func TestCompile(t *testing.T) {
	yamlInput := `
version: 1
diagram:
  title: Production Data Center Fabric
  layout: auto
  theme: light
groups:
  dc1:
    label: DC1
    kind: site
    groups:
      rack-a:
        label: Rack A
        kind: rack
        nodes:
          leaf-01: {}
          leaf-02: {}
nodes:
  leaf-01:
    role: leaf
    icon_label: PE
    metadata:
      mgmt_ip: 10.0.0.1
  leaf-02:
    role: leaf
  server-01:
    role: server
links:
  - from: leaf-01:Ethernet1/1
    to:
      node: server-01
      port: eth0
      side: top
    label: 25G
`

	var doc spec.Document
	if err := yaml.Unmarshal([]byte(yamlInput), &doc); err != nil {
		t.Fatalf("failed to unmarshal test yaml: %v", err)
	}

	diag, err := Compile(&doc)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify theme values
	if diag.Theme.Title != "Production Data Center Fabric" {
		t.Errorf("expected Title %q, got %q", "Production Data Center Fabric", diag.Theme.Title)
	}
	if diag.Theme.Layout != "auto" {
		t.Errorf("expected Layout %q, got %q", "auto", diag.Theme.Layout)
	}
	if diag.Theme.Name != "light" {
		t.Errorf("expected Theme Name %q, got %q", "light", diag.Theme.Name)
	}

	// Verify nodes
	if len(diag.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(diag.Nodes))
	}
	// Verify sorted nodes by ID: leaf-01, leaf-02, server-01
	if diag.Nodes[0].ID != "leaf-01" || diag.Nodes[1].ID != "leaf-02" || diag.Nodes[2].ID != "server-01" {
		t.Errorf("nodes not sorted by ID correctly: %+v", diag.Nodes)
	}
	if diag.Nodes[0].Metadata["mgmt_ip"] != "10.0.0.1" {
		t.Errorf("expected mgmt_ip 10.0.0.1, got %v", diag.Nodes[0].Metadata["mgmt_ip"])
	}
	if diag.Nodes[0].IconLabel != "PE" {
		t.Errorf("expected icon label PE, got %q", diag.Nodes[0].IconLabel)
	}

	// Verify groups
	if len(diag.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(diag.Groups))
	}
	// Sort by ID or check specific
	var dc1, rackA Group
	for _, g := range diag.Groups {
		if g.ID == "dc1" {
			dc1 = g
		} else if g.ID == "rack-a" {
			rackA = g
		}
	}
	if dc1.Label != "DC1" || dc1.ParentID != "" || len(dc1.NodeIDs) != 0 {
		t.Errorf("unexpected dc1 group: %+v", dc1)
	}
	if rackA.Label != "Rack A" || rackA.ParentID != "dc1" || len(rackA.NodeIDs) != 2 || rackA.NodeIDs[0] != "leaf-01" || rackA.NodeIDs[1] != "leaf-02" {
		t.Errorf("unexpected rack-a group: %+v", rackA)
	}

	// Verify links
	if len(diag.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(diag.Links))
	}
	l := diag.Links[0]
	if l.From.Node != "leaf-01" || l.From.Port != "Ethernet1/1" || l.From.Side != "" {
		t.Errorf("unexpected link from: %+v", l.From)
	}
	if l.To.Node != "server-01" || l.To.Port != "eth0" || l.To.Side != "top" {
		t.Errorf("unexpected link to: %+v", l.To)
	}
}
