package drawio

import (
	"encoding/xml"
	"fmt"
	"reflect"
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

func TestRenderUsesNodeSizeHints(t *testing.T) {
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "hub", Label: "Hub", Role: "core-router", Width: 520, Height: 150}},
	}
	got, err := Render(diagram)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="node-hub"`, `width="520" height="150"`} {
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

func TestRenderWithLayoutReportReconcilesTopologyDrift(t *testing.T) {
	x, y := 400.0, 200.0
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "core-a"}, {ID: "edge-new"}},
		Links: []model.Link{{
			ID: "new-link", From: model.LinkEndpoint{Node: "core-a"}, To: model.LinkEndpoint{Node: "edge-new"},
		}},
	}
	overrides := &layoutoverride.Document{
		Version: 1,
		LayoutOverrides: layoutoverride.Overrides{
			Nodes: map[string]layoutoverride.Bounds{
				"core-a":   {X: &x, Y: &y},
				"old-edge": {X: &x, Y: &y},
			},
			Links: map[string]layoutoverride.Link{"old-link": {}},
		},
	}
	if _, err := RenderWithOptions(diagram, Options{Overrides: overrides}); err == nil {
		t.Fatal("strict render accepted stale overrides")
	}
	_, report, err := RenderWithLayoutReport(diagram, Options{Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	if report.Preserved.Nodes != 1 || report.Preserved.Links != 0 {
		t.Fatalf("unexpected preserved counts: %+v", report.Preserved)
	}
	if !reflect.DeepEqual(report.IgnoredStale, []string{"link old-link", "node old-edge"}) {
		t.Fatalf("unexpected stale overrides: %+v", report.IgnoredStale)
	}
	if !reflect.DeepEqual(report.AutoPlaced, []LayoutPlacement{{Kind: "node", ID: "edge-new", Near: "core-a"}}) {
		t.Fatalf("unexpected auto placement: %+v", report.AutoPlaced)
	}
	if !reflect.DeepEqual(report.AutoRouted, []string{"new-link"}) {
		t.Fatalf("unexpected auto routes: %+v", report.AutoRouted)
	}
	formatted := FormatLayoutReport(report)
	for _, want := range []string{"- node edge-new near core-a", "- new-link", "- node old-edge"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("layout report does not contain %q:\n%s", want, formatted)
		}
	}
}

func TestPlaceNearManagedNeighborUsesSortedNeighborAndFallsBackWhenFull(t *testing.T) {
	neighbors := map[string][]string{"new": {"core-a", "core-b"}}
	placed := map[string]nodePlacement{
		"core-a": {Parent: "1", X: 100, Y: 100, Width: 170, Height: 70},
		"core-b": {Parent: "1", X: 900, Y: 100, Width: 170, Height: 70},
	}
	anchors := placementIDs(placed)
	x, y, near, ok := placeNearManagedNeighbor("new", "1", 170, 70, neighbors, placed, anchors)
	if !ok || near != "core-a" || x != 100 || y != 250 {
		t.Fatalf("unexpected multiple-neighbor placement: x=%v y=%v near=%q ok=%v", x, y, near, ok)
	}

	for index := 0; index < 8; index++ {
		id := fmt.Sprintf("block-a-%d", index)
		placed[id] = nodePlacement{Parent: "1", X: 100 + float64(index)*240, Y: 250, Width: 170, Height: 70}
		id = fmt.Sprintf("block-b-%d", index)
		placed[id] = nodePlacement{Parent: "1", X: 900 + float64(index)*240, Y: 250, Width: 170, Height: 70}
	}
	if _, _, _, ok := placeNearManagedNeighbor("new", "1", 170, 70, neighbors, placed, anchors); ok {
		t.Fatal("placement unexpectedly succeeded when candidate area was full")
	}
	if _, _, _, ok := placeNearManagedNeighbor("new", "group-site", 170, 70, neighbors, placed, anchors); ok {
		t.Fatal("placement unexpectedly used a neighbor from another group")
	}
}

func TestLayoutReportPlacesNewNodeInsideResizedGroupAndReportsNoNeighborFallback(t *testing.T) {
	groupX, groupY, groupWidth, groupHeight := 50.0, 60.0, 1100.0, 700.0
	coreX, coreY := 120.0, 140.0
	diagram := &model.Diagram{
		Groups: []model.Group{{ID: "site", NodeIDs: []string{"core", "edge"}}, {ID: "empty-site", NodeIDs: []string{"isolated"}}},
		Nodes:  []model.Node{{ID: "core"}, {ID: "edge"}, {ID: "isolated"}},
		Links: []model.Link{{
			ID: "edge-link", From: model.LinkEndpoint{Node: "core"}, To: model.LinkEndpoint{Node: "edge"},
		}},
	}
	overrides := &layoutoverride.Document{
		Version: 1,
		LayoutOverrides: layoutoverride.Overrides{
			Groups: map[string]layoutoverride.Bounds{"site": {
				X: &groupX, Y: &groupY, Width: &groupWidth, Height: &groupHeight,
			}},
			Nodes: map[string]layoutoverride.Bounds{"core": {X: &coreX, Y: &coreY}},
		},
	}
	rendered, report, err := RenderWithLayoutReport(diagram, Options{Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := ExtractOverrides(rendered, diagram)
	if err != nil {
		t.Fatal(err)
	}
	site := extracted.LayoutOverrides.Groups["site"]
	if *site.Width != groupWidth || *site.Height != groupHeight {
		t.Fatalf("resized group was not preserved: %+v", site)
	}
	edge := extracted.LayoutOverrides.Nodes["edge"]
	if *edge.X != coreX || *edge.Y != coreY+70+80 {
		t.Fatalf("new grouped node was not placed near its positioned neighbor: %+v", edge)
	}
	wantPlacements := []LayoutPlacement{
		{Kind: "group", ID: "empty-site"},
		{Kind: "node", ID: "edge", Near: "core"},
		{Kind: "node", ID: "isolated"},
	}
	if !reflect.DeepEqual(report.AutoPlaced, wantPlacements) {
		t.Fatalf("unexpected grouped/fallback placements: %+v", report.AutoPlaced)
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
	if _, _, err := RenderWithLayoutReport(diagram, Options{}); err == nil {
		t.Fatal("layout-report render accepted duplicate automatic link IDs")
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
