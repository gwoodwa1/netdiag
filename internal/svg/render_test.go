package svg

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/model"
	"github.com/gwoodwa1/netdiag/internal/spec"
)

func TestRenderUsesCustomIconAndFallsBackToBuiltIn(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"custom":   {Role: "edge-router", IconLabel: "PE"},
			"custom-2": {Role: "edge-router"},
			"fallback": {Role: "firewall"},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	custom := `<svg viewBox="0 0 100 100"><path id="custom-router-marker" d="M0 0h100v100z"/></svg>`
	if err := os.WriteFile(filepath.Join(dir, "router.svg"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "firewall.svg"), []byte(`<svg><script/></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RenderWithOptions(diag, Options{IconDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	if !strings.Contains(got, "custom-device-icon") || !strings.Contains(got, "custom-router-marker") {
		t.Fatal("render missing custom canonical router icon")
	}
	if !strings.Contains(got, "netdiag-icon-custom-custom-router-marker") || !strings.Contains(got, "netdiag-icon-custom-2-custom-router-marker") {
		t.Fatal("repeated custom icons must have instance-specific internal IDs")
	}
	if !strings.Contains(got, `class="device-icon-label"`) || !strings.Contains(got, ">PE</text>") {
		t.Fatal("render missing custom icon label")
	}
	if !strings.Contains(got, `device-icon-firewall`) || !strings.Contains(got, `stroke="#dc2626"`) {
		t.Fatal("invalid custom firewall icon did not fall back to built-in")
	}
}

func TestPremiumThemeAddsFidelityEffects(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Theme: "premium"},
		Nodes: map[string]spec.Node{
			"a": {Role: "router"},
			"b": {Role: "router"},
		},
		Links: []spec.Link{{From: spec.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: spec.LinkEndpoint{Node: "b", Port: "Eth0/0"}}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	for _, expected := range []string{`id="technicalGrid"`, `id="deviceCardGradient"`, `id="deviceShadow"`, `id="portGlow"`, `class="link-underlay"`, `fill="url(#deviceCardGradient)"`} {
		if !strings.Contains(got, expected) {
			t.Fatalf("premium render missing %q", expected)
		}
	}
}

func TestDefaultThemeOmitsPremiumEffects(t *testing.T) {
	doc := &spec.Document{Version: 1, Nodes: map[string]spec.Node{"router": {Role: "router"}}}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), `id="technicalGrid"`) || strings.Contains(string(result), `fill="url(#deviceCardGradient)"`) {
		t.Fatal("default render unexpectedly contains premium effects")
	}
}

func TestNamedThemeAndLinkRulesRender(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{
			Theme: "nord",
			LinkStyles: spec.LinkStyleRules{
				Protocol: map[string]spec.VisualStyle{"ospf": {Color: "#00ff00", Pattern: "solid", Width: 3}},
				Status:   map[string]spec.VisualStyle{"inactive": {Color: "#888888", Pattern: "dashed"}},
			},
		},
		Nodes: map[string]spec.Node{"a": {Role: "router"}, "b": {Role: "router"}},
		Links: []spec.Link{{From: spec.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: spec.LinkEndpoint{Node: "b", Port: "Eth0/0"}, Protocol: "ospf", Status: "inactive"}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	for _, want := range []string{`class="theme-nord"`, `.theme-nord`, `stroke="#888888"`, `stroke-width="3.0"`, `stroke-dasharray="8 5"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("render missing %q", want)
		}
	}
}

func TestRenderIncludesEndpointLabels(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Title: "Test"},
		Nodes: map[string]spec.Node{
			"spine-01": {Label: "Spine 01", Role: "spine"},
			"leaf-01":  {Label: "Leaf 01", Role: "leaf"},
		},
		Links: []spec.Link{{
			From:   spec.LinkEndpoint{Node: "spine-01", Port: "Ethernet1/1"},
			To:     spec.LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/49"},
			Bundle: "Port-Channel10",
			LACP:   true,
			Trunk:  &spec.Trunk{Encapsulation: "dot1q", AllowedVLANs: []string{"10", "20"}},
		}},
	}

	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	svg := string(result)
	for _, expected := range []string{"Spine 01", "Leaf 01", "Eth1/1", "Eth1/49", "LACP", "Po10", "TRUNK", "DOT1Q", "VLAN 10,20", "BUNDLE DETAILS"} {
		if !strings.Contains(svg, expected) {
			t.Fatalf("render missing %q", expected)
		}
	}
	for _, expected := range []string{"device-icon-spine", "device-icon-leaf"} {
		if !strings.Contains(svg, expected) {
			t.Fatalf("render missing %q", expected)
		}
	}
	if !strings.Contains(svg, `class="row-heading row-heading-leaf"`) {
		t.Fatal("render missing leaf-layer heading")
	}
	if strings.Index(svg, `id="row-headings"`) > strings.Index(svg, `id="links"`) {
		t.Fatal("gutter headings should render before links without masking them")
	}
}

func TestRenderIncludesStructuredLabelsAndAddresses(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes:   map[string]spec.Node{"a": {Role: "router"}, "b": {Role: "router"}},
		Links: []spec.Link{{
			From:   spec.LinkEndpoint{Node: "a", Port: "Ethernet0/0", Address: "10.10.10.1/30"},
			To:     spec.LinkEndpoint{Node: "b", Port: "Ethernet0/1", Address: "10.10.10.2/30"},
			Labels: &spec.LinkLabels{Source: "WAN-A", Middle: "CKT-1001", Target: "WAN-B"},
		}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"WAN-A", "10.10.10.1/30", "CKT-1001", "WAN-B", "10.10.10.2/30"} {
		if !strings.Contains(string(result), expected) {
			t.Fatalf("render missing %q", expected)
		}
	}
}

func TestRenderPlacesLinkAnnotationsAboveNodes(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes:   map[string]spec.Node{"a": {Role: "router"}, "b": {Role: "router"}},
		Links: []spec.Link{{
			From:   spec.LinkEndpoint{Node: "a", Port: "Ethernet0/0", Address: "10.0.0.1/30"},
			To:     spec.LinkEndpoint{Node: "b", Port: "Ethernet0/1", Address: "10.0.0.2/30"},
			Labels: &spec.LinkLabels{Middle: "WAN"},
			Bundle: "Port-Channel10",
		}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	links := strings.Index(got, `id="links"`)
	nodes := strings.Index(got, `id="nodes"`)
	annotations := strings.Index(got, `id="link-annotations"`)
	legend := strings.Index(got, `id="bundle-legend"`)
	if links < 0 || nodes < 0 || annotations < 0 || legend < 0 || !(links < nodes && nodes < annotations && annotations < legend) {
		t.Fatalf("unexpected SVG layer order: links=%d nodes=%d annotations=%d legend=%d", links, nodes, annotations, legend)
	}
	if !strings.Contains(got, `<g id="link-annotations" pointer-events="none">`) {
		t.Fatal("link annotations should not steal interactive node or link clicks")
	}
	if strings.Index(got, `class="interface-label-badge"`) < annotations {
		t.Fatal("interface label rendered before the annotation layer")
	}
}

func TestNodesStayOutsideHeadingGutter(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"leaf-01": {Role: "leaf"},
			"leaf-02": {Role: "leaf"},
			"leaf-03": {Role: "leaf"},
			"leaf-04": {Role: "leaf"},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	roles, byRole := groupNodes(diag)
	nodes := placeNodes(diag, roles, byRole)
	for id, node := range nodes {
		if node.Box.X < diagramLeft {
			t.Fatalf("node %s starts at %.1f inside heading gutter", id, node.Box.X)
		}
	}
}

func TestNodeOrderControlsPlacementWithinRole(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"alpha":   {Role: "router", Order: 20},
			"charlie": {Role: "router", Order: 10},
			"bravo":   {Role: "router"},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	roles, byRole := groupNodes(diag)
	got := byRole["router"]
	want := []string{"charlie", "alpha", "bravo"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("placement order = %v, want %v", got, want)
		}
	}
	nodes := placeNodes(diag, roles, byRole)
	if !(nodes["charlie"].Box.X < nodes["alpha"].Box.X && nodes["alpha"].Box.X < nodes["bravo"].Box.X) {
		t.Fatal("expected ordered nodes to be placed left-to-right")
	}
}

func TestRingLayoutPlacesNodesAroundCenter(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Layout: "ring"},
		Nodes: map[string]spec.Node{
			"north": {Role: "router", Order: 10},
			"east":  {Role: "router", Order: 20},
			"south": {Role: "router", Order: 30},
			"west":  {Role: "router", Order: 40},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	roles, byRole := groupNodes(diag)
	nodes := placeNodes(diag, roles, byRole)
	if !(nodes["north"].Box.Y < nodes["east"].Box.Y && nodes["east"].Box.X > nodes["north"].Box.X) {
		t.Fatal("expected ordered nodes to follow the ring clockwise from north")
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `id="ring-background"`) {
		t.Fatal("expected ring background")
	}
}

func TestSiteLayoutPlacesNodesInsideSiteBoundaries(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Layout: "sites", LinkStyle: "orthogonal"},
		Groups: map[string]*spec.Group{
			"west": {Label: "West Site", Kind: "site", Nodes: map[string]interface{}{"west-pe": nil}},
			"east": {Label: "East Site", Kind: "site", Nodes: map[string]interface{}{"east-pe": nil}},
		},
		Nodes: map[string]spec.Node{"west-pe": {Role: "router"}, "east-pe": {Role: "router"}},
		Links: []spec.Link{{From: spec.LinkEndpoint{Node: "west-pe", Port: "Eth0/0"}, To: spec.LinkEndpoint{Node: "east-pe", Port: "Eth0/0"}}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	roles, byRole := groupNodes(diag)
	layout := layoutDiagram(diag, roles, byRole)
	if len(layout.Groups) != 2 {
		t.Fatalf("got %d site boundaries, want 2", len(layout.Groups))
	}
	for _, group := range layout.Groups {
		node := layout.Nodes[group.ID+"-pe"]
		if node.Box.X < group.Box.X || node.Box.X+node.Box.W > group.Box.X+group.Box.W {
			t.Fatalf("node %s is outside site %s", node.ID, group.ID)
		}
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `id="site-backgrounds"`) {
		t.Fatal("site-aware render missing site backgrounds")
	}
	if !strings.Contains(string(result), " H ") || strings.Contains(string(result), " L ") {
		t.Fatal("site-aware render did not use orthogonal link segments")
	}
}

func TestSiteLayoutLeavesLabelGutterBetweenSameRoleNodes(t *testing.T) {
	diagram := &model.Diagram{
		Theme:  model.Theme{Layout: "sites"},
		Groups: []model.Group{{ID: "core", Kind: "core", NodeIDs: []string{"p1", "p2"}}},
		Nodes:  []model.Node{{ID: "p1", Role: "router"}, {ID: "p2", Role: "router"}},
	}
	layout := placeSiteLayout(diagram)
	gap := layout.Nodes["p2"].Box.X - (layout.Nodes["p1"].Box.X + layout.Nodes["p1"].Box.W)
	if gap < siteLinkGap {
		t.Fatalf("same-role site node gap = %.1f, want at least %.1f", gap, siteLinkGap)
	}
}

func TestSiteLayoutExpandsHighDegreeCoreRouters(t *testing.T) {
	diagram := &model.Diagram{
		Theme:  model.Theme{Layout: "sites"},
		Groups: []model.Group{{ID: "core", Kind: "core", NodeIDs: []string{"p1", "p2"}}},
		Nodes:  []model.Node{{ID: "p1", Role: "core-router"}, {ID: "p2", Role: "core-router"}},
	}
	for index := 0; index < 9; index++ {
		peer := fmt.Sprintf("pe-%d", index)
		diagram.Nodes = append(diagram.Nodes, model.Node{ID: peer, Role: "edge-router"})
		diagram.Links = append(diagram.Links, model.Link{
			From: model.LinkEndpoint{Node: "p1", Port: fmt.Sprintf("Hu0/%d", index)},
			To:   model.LinkEndpoint{Node: peer, Port: "Hu0/0"},
		})
	}
	layout := placeSiteLayout(diagram)
	if got := layout.Nodes["p1"].Box.W; got <= nodeWidth("core-router") {
		t.Fatalf("high-degree core router width = %.1f, want greater than base width", got)
	}
	if got := layout.Nodes["p1"].Box.H; got <= nodeHeight {
		t.Fatalf("high-degree core router height = %.1f, want greater than base height", got)
	}
	if layout.Width <= canvasWidth {
		t.Fatalf("high-degree topology canvas width = %.1f, want greater than %.1f", layout.Width, canvasWidth)
	}
}

func TestNodeSizeHintsControlNativeLayout(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "sites"},
		Groups: []model.Group{
			{ID: "core", Kind: "core", NodeIDs: []string{"hub"}},
			{ID: "edge", Kind: "site", NodeIDs: []string{"spoke"}},
		},
		Nodes: []model.Node{
			{ID: "hub", Role: "core-router", Width: 520, Height: 150},
			{ID: "spoke", Role: "edge-router", Width: 320, Height: 110},
		},
		Links: []model.Link{{
			From: model.LinkEndpoint{Node: "hub", Port: "Hu0/0/0/0"},
			To:   model.LinkEndpoint{Node: "spoke", Port: "Hu0/0/0/0"},
		}},
	}
	layout := placeSiteLayout(diagram)
	if got := layout.Nodes["hub"].Box; got.W != 520 || got.H != 150 {
		t.Fatalf("hub box = %.1fx%.1f, want 520x150", got.W, got.H)
	}
	if got := layout.Nodes["spoke"].Box; got.W != 320 || got.H != 110 {
		t.Fatalf("spoke box = %.1fx%.1f, want 320x110", got.W, got.H)
	}
}

func TestHubSpokeLayoutPlacesCoreBetweenSpokes(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Groups: []model.Group{
			{ID: "core", NodeIDs: []string{"p1", "p2"}},
			{ID: "left", NodeIDs: []string{"left-pe1", "left-pe2"}},
			{ID: "right", NodeIDs: []string{"right-pe1", "right-pe2"}},
		},
		Nodes: []model.Node{
			{ID: "p1", Role: "core-router"}, {ID: "p2", Role: "core-router"},
			{ID: "left-pe1", Role: "edge-router"}, {ID: "left-pe2", Role: "edge-router"},
			{ID: "right-pe1", Role: "edge-router"}, {ID: "right-pe2", Role: "edge-router"},
		},
	}
	layout := placeHubSpokeLayout(diagram)
	core := layout.Nodes["p1"].Box
	top := layout.Nodes["left-pe1"].Box
	bottom := layout.Nodes["right-pe1"].Box
	if !(top.Y < core.Y && core.Y < bottom.Y) {
		t.Fatalf("core was not placed between spokes: top=%+v core=%+v bottom=%+v", top, core, bottom)
	}
}

func TestHubSpokeLayoutUsesOversizedDefaultCards(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Groups: []model.Group{
			{ID: "core", NodeIDs: []string{"p1", "p2"}},
			{ID: "site", NodeIDs: []string{"pe1", "pe2"}},
		},
		Nodes: []model.Node{
			{ID: "p1", Role: "core-router"},
			{ID: "p2", Role: "core-router"},
			{ID: "pe1", Role: "edge-router"},
			{ID: "pe2", Role: "edge-router"},
		},
	}
	layout := placeHubSpokeLayout(diagram)
	if got := layout.Nodes["p1"].Box; got.W != hubSpokeHubNodeWidth || got.H != hubSpokeHubNodeHeight {
		t.Fatalf("hub box = %.1fx%.1f, want %.1fx%.1f", got.W, got.H, hubSpokeHubNodeWidth, hubSpokeHubNodeHeight)
	}
	if got := layout.Nodes["pe1"].Box; got.W != hubSpokeSpokeNodeWidth || got.H != hubSpokeSpokeNodeHeight {
		t.Fatalf("spoke box = %.1fx%.1f, want %.1fx%.1f", got.W, got.H, hubSpokeSpokeNodeWidth, hubSpokeSpokeNodeHeight)
	}
}

func TestHubSpokeLayoutSeparatesCorePlanes(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Groups: []model.Group{
			{ID: "core-a", NodeIDs: []string{"p1"}},
			{ID: "core-b", NodeIDs: []string{"p2"}},
			{ID: "site-a", NodeIDs: []string{"pe1"}},
			{ID: "site-b", NodeIDs: []string{"pe2"}},
		},
		Nodes: []model.Node{
			{ID: "p1", Role: "core-router"},
			{ID: "p2", Role: "core-router"},
			{ID: "pe1", Role: "edge-router"},
			{ID: "pe2", Role: "edge-router"},
		},
	}
	layout := placeHubSpokeLayout(diagram)
	var coreA, coreB box
	for _, group := range layout.Groups {
		switch group.ID {
		case "core-a":
			coreA = group.Box
		case "core-b":
			coreB = group.Box
		}
	}
	if got := coreB.Y - (coreA.Y + coreA.H); got < hubSpokeCoreGroupGap {
		t.Fatalf("core plane gap = %.1f, want at least %.1f", got, hubSpokeCoreGroupGap)
	}
	if layout.Height != hubSpokeCanvasHeight {
		t.Fatalf("hub-spoke canvas height = %.1f, want %.1f", layout.Height, hubSpokeCanvasHeight)
	}
}

func TestHubSpokeLayoutAllowsExplicitCardSizeOverride(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Groups: []model.Group{
			{ID: "core", NodeIDs: []string{"p1"}},
			{ID: "site", NodeIDs: []string{"pe1"}},
		},
		Nodes: []model.Node{
			{ID: "p1", Role: "core-router", Width: 900, Height: 260},
			{ID: "pe1", Role: "edge-router", Width: 480, Height: 170},
		},
	}
	layout := placeHubSpokeLayout(diagram)
	if got := layout.Nodes["p1"].Box; got.W != 900 || got.H != 260 {
		t.Fatalf("hub override box = %.1fx%.1f, want 900x260", got.W, got.H)
	}
	if got := layout.Nodes["pe1"].Box; got.W != 480 || got.H != 170 {
		t.Fatalf("spoke override box = %.1fx%.1f, want 480x170", got.W, got.H)
	}
}

func TestHubSpokeLayoutReservesRoutingSpaceBetweenEveryDualPEPair(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Groups: []model.Group{
			{ID: "core", NodeIDs: []string{"p1", "p2"}},
		},
		Nodes: []model.Node{
			{ID: "p1", Role: "core-router"},
			{ID: "p2", Role: "core-router"},
		},
	}
	for site := 1; site <= 8; site++ {
		groupID := fmt.Sprintf("site-%d", site)
		pe1 := groupID + "-pe1"
		pe2 := groupID + "-pe2"
		diagram.Groups = append(diagram.Groups, model.Group{ID: groupID, NodeIDs: []string{pe1, pe2}})
		diagram.Nodes = append(diagram.Nodes,
			model.Node{ID: pe1, Role: "edge-router"},
			model.Node{ID: pe2, Role: "edge-router"},
		)
		if site < 8 {
			diagram.Links = append(diagram.Links,
				model.Link{From: model.LinkEndpoint{Node: pe1}, To: model.LinkEndpoint{Node: "p1"}},
				model.Link{From: model.LinkEndpoint{Node: pe1}, To: model.LinkEndpoint{Node: "p2"}},
				model.Link{From: model.LinkEndpoint{Node: pe2}, To: model.LinkEndpoint{Node: "p1"}},
				model.Link{From: model.LinkEndpoint{Node: pe2}, To: model.LinkEndpoint{Node: "p2"}},
			)
		}
	}

	layout := placeHubSpokeLayout(diagram)
	for site := 1; site <= 8; site++ {
		pe1 := layout.Nodes[fmt.Sprintf("site-%d-pe1", site)].Box
		pe2 := layout.Nodes[fmt.Sprintf("site-%d-pe2", site)].Box
		gap := pe2.X - (pe1.X + pe1.W)
		if gap < hubSpokePEGap {
			t.Fatalf("site %d PE routing gap = %.1f, want at least %.1f", site, gap, hubSpokePEGap)
		}
	}
}

func TestHubSpokeLayoutExpandsCanvasForManySpokeSites(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Groups: []model.Group{
			{ID: "core", NodeIDs: []string{"p1", "p2"}},
		},
		Nodes: []model.Node{
			{ID: "p1", Role: "core-router"},
			{ID: "p2", Role: "core-router"},
		},
	}
	for site := 1; site <= 9; site++ {
		groupID := fmt.Sprintf("site-%d", site)
		pe1 := groupID + "-pe1"
		pe2 := groupID + "-pe2"
		diagram.Groups = append(diagram.Groups, model.Group{ID: groupID, NodeIDs: []string{pe1, pe2}})
		diagram.Nodes = append(diagram.Nodes,
			model.Node{ID: pe1, Role: "edge-router"},
			model.Node{ID: pe2, Role: "edge-router"},
		)
	}

	layout := placeHubSpokeLayout(diagram)
	if layout.Width <= hubSpokeCanvasWidth {
		t.Fatalf("hub-spoke canvas width = %.1f, want wider than %.1f for many spoke sites", layout.Width, hubSpokeCanvasWidth)
	}

	var topRow []placedGroup
	for _, group := range layout.Groups {
		if strings.HasPrefix(group.ID, "site-") && group.Box.Y < hubSpokeCanvasHeight/2 {
			topRow = append(topRow, group)
		}
	}
	sort.Slice(topRow, func(i, j int) bool { return topRow[i].Box.X < topRow[j].Box.X })
	if len(topRow) != 5 {
		t.Fatalf("top row site count = %d, want 5", len(topRow))
	}
	for i := 1; i < len(topRow); i++ {
		gap := topRow[i].Box.X - (topRow[i-1].Box.X + topRow[i-1].Box.W)
		if gap < hubSpokeSiteGroupGap {
			t.Fatalf("site group gap %d = %.1f, want at least %.1f", i, gap, hubSpokeSiteGroupGap)
		}
	}
}

func TestDiagonalRouteAndEndpointLabelsFollowLineGeometry(t *testing.T) {
	route := diagonalRoute(point{X: 100, Y: 100}, point{X: 500, Y: 300}, 0)
	if route.Path != "M 100.0 100.0 Q 300.0 200.0 500.0 300.0" {
		t.Fatalf("unexpected diagonal route: %s", route.Path)
	}
	var out bytes.Buffer
	renderRouteEndpointLabel(&out, route, "Hu0/0/0/0", true, 2, 0, model.InterfaceLabelStyle{})
	renderRouteEndpointLabel(&out, route, "Hu0/0/0/1", false, 2, 0, model.InterfaceLabelStyle{})
	got := out.String()
	if !strings.Contains(got, `x="144.0"`) || !strings.Contains(got, `x="440.0"`) {
		t.Fatalf("route labels did not follow diagonal geometry: %s", got)
	}
}

func TestHighDegreeDiagonalEndpointLabelMovesAwayFromHub(t *testing.T) {
	route := diagonalRoute(point{X: 100, Y: 100}, point{X: 500, Y: 300}, 0)
	var out bytes.Buffer
	renderRouteEndpointLabel(&out, route, "Hu0/0/0/0", true, 9, 0, model.InterfaceLabelStyle{})
	if !strings.Contains(out.String(), `x="204.0"`) {
		t.Fatalf("high-degree endpoint label did not move along route: %s", out.String())
	}
}

func TestRouteEndpointLabelPlacementHints(t *testing.T) {
	route := diagonalRoute(point{X: 100, Y: 100}, point{X: 500, Y: 300}, 0)
	along, offset := 0.5, 0.0
	location, ok := routeEndpointLabelLocation(route, true, 2, 0, model.LinkEndpoint{LabelAlong: &along, LabelOffset: &offset})
	if !ok {
		t.Fatal("expected label location")
	}
	if location.X != 300 || location.Y != 200 {
		t.Fatalf("label location = %+v, want {300 200}", location)
	}
	offset = 40
	nudged, ok := routeEndpointLabelLocation(route, true, 2, 0, model.LinkEndpoint{LabelAlong: &along, LabelOffset: &offset})
	if !ok {
		t.Fatal("expected nudged label location")
	}
	if math.Hypot(nudged.X-location.X, nudged.Y-location.Y) < 39 {
		t.Fatalf("label offset did not nudge away from route: base=%+v nudged=%+v", location, nudged)
	}
}

func TestDiagonalRouteLanesSeparateCurves(t *testing.T) {
	start, end := point{X: 100, Y: 100}, point{X: 500, Y: 300}
	first := diagonalRoute(start, end, 1)
	second := diagonalRoute(start, end, 2)
	if first.Points[1] == second.Points[1] || first.Points[1] == pointAlongLine(start, end, 0.5) {
		t.Fatalf("diagonal lanes were not separated: first=%+v second=%+v", first.Points, second.Points)
	}
}

func TestRoutedDiagonalRouteUsesRequestedStraightStub(t *testing.T) {
	route := routedDiagonalRoute(routedLink{
		Start:     point{X: 100, Y: 100},
		End:       point{X: 500, Y: 500},
		StartSide: "bottom",
		StartStub: 120,
	}, 0)
	if got := route.Points[1]; got != (point{X: 100, Y: 220}) {
		t.Fatalf("stub endpoint = %+v, want {100 220}", got)
	}
	if !strings.Contains(route.Path, "M 100.0 100.0 L 100.0 220.0 Q") {
		t.Fatalf("route does not leave straight before diagonal: %s", route.Path)
	}
}

func TestStubbedRouteEndpointLabelUsesStraightSection(t *testing.T) {
	route := routedDiagonalRoute(routedLink{
		Start:     point{X: 100, Y: 100},
		End:       point{X: 500, Y: 500},
		StartSide: "bottom",
		StartStub: 120,
	}, 0)
	location, ok := routeStubLabelPoint(route, true)
	if !ok || location != (point{X: 100, Y: 166}) {
		t.Fatalf("source stub label point = %+v, %t; want {100 166}, true", location, ok)
	}
	if _, ok := routeStubLabelPoint(route, false); ok {
		t.Fatal("non-stubbed target unexpectedly received a stub label point")
	}
}

func TestStubbedRouteEndpointLabelDefaultsToStubCenterline(t *testing.T) {
	route := routedDiagonalRoute(routedLink{
		Start:     point{X: 100, Y: 100},
		End:       point{X: 500, Y: 500},
		StartSide: "bottom",
		StartStub: 120,
	}, 0)
	location, ok := routeEndpointLabelLocation(route, true, 2, 0, model.LinkEndpoint{})
	if !ok || location != (point{X: 100, Y: 166}) {
		t.Fatalf("source stub label location = %+v, %t; want {100 166}, true", location, ok)
	}
	offset := 20.0
	nudged, ok := routeEndpointLabelLocation(route, true, 2, 0, model.LinkEndpoint{LabelOffset: &offset})
	if !ok || nudged == location {
		t.Fatalf("explicit label_offset was not honored: base=%+v nudged=%+v ok=%t", location, nudged, ok)
	}
}

func TestDetouredRouteEndpointLabelStillUsesEndpointStub(t *testing.T) {
	route := linkRoute{Points: []point{
		{X: 100, Y: 100},
		{X: 100, Y: 220},
		{X: 260, Y: 260},
		{X: 380, Y: 380},
		{X: 500, Y: 380},
		{X: 500, Y: 500},
	}}
	source, ok := routeEndpointLabelLocation(route, true, 2, 0, model.LinkEndpoint{})
	if !ok || source != (point{X: 100, Y: 166}) {
		t.Fatalf("source detoured stub label = %+v, %t; want {100 166}, true", source, ok)
	}
	target, ok := routeEndpointLabelLocation(route, false, 2, 0, model.LinkEndpoint{})
	if !ok || target != (point{X: 500, Y: 434}) {
		t.Fatalf("target detoured stub label = %+v, %t; want {500 434}, true", target, ok)
	}
}

func TestRotatedEndpointLabelRotatesCompleteBadge(t *testing.T) {
	var out bytes.Buffer
	renderRotatedInterfaceLabel(&out, 100, 200, "Hu0/0/0/0", 90, model.InterfaceLabelStyle{})
	got := out.String()
	if !strings.Contains(got, `class="interface-label-rotation" transform="rotate(90 100.0 194.5)"`) {
		t.Fatalf("rotated label missing expected group transform: %s", got)
	}
	if !strings.Contains(got, `interface-label-badge`) || !strings.Contains(got, `interface-label-text`) {
		t.Fatalf("rotation did not include complete badge: %s", got)
	}
}

func TestPlanDiagonalRoutesReducesGlobalCrossings(t *testing.T) {
	links := []routedLink{
		{Index: 0, FromNode: "a", ToNode: "d", Start: point{X: 0, Y: 0}, End: point{X: 400, Y: 400}},
		{Index: 1, FromNode: "b", ToNode: "c", Start: point{X: 0, Y: 400}, End: point{X: 400, Y: 0}},
	}
	initial := routeIntersectionCount(diagonalRoute(links[0].Start, links[0].End, 0), diagonalRoute(links[1].Start, links[1].End, 0))
	routes := planDiagonalRoutes(links)
	planned := routeIntersectionCount(routes[0], routes[1])
	if initial == 0 || planned >= initial {
		t.Fatalf("global planner did not reduce crossings: initial=%d planned=%d routes=%+v", initial, planned, routes)
	}
}

func TestDiagonalPlannerAvoidsUnrelatedNodeBox(t *testing.T) {
	links := []routedLink{{
		Index: 0, FromNode: "a", ToNode: "b",
		Start: point{X: 0, Y: 100}, End: point{X: 500, Y: 100},
	}}
	nodes := map[string]placedNode{
		"a":      {Box: box{X: -80, Y: 60, W: 80, H: 80}},
		"middle": {Box: box{X: 220, Y: 60, W: 60, H: 80}},
		"b":      {Box: box{X: 500, Y: 60, W: 80, H: 80}},
	}
	routes := planDiagonalRoutesWithObstacles(links, 24, nodes)
	route := routes[0]
	if routeIntersectsObstacle(route, expandBox(nodes["middle"].Box, 24)) {
		t.Fatalf("diagonal route still crosses unrelated node: %+v", route)
	}
	if strings.Contains(route.Path, " Q ") || len(route.Points) < 3 {
		t.Fatalf("obstructed diagonal route did not receive waypoint detour: %+v", route)
	}
}

func TestDiagonalPlannerKeepsClearRouteCurved(t *testing.T) {
	links := []routedLink{{
		Index: 0, FromNode: "a", ToNode: "b",
		Start: point{X: 0, Y: 100}, End: point{X: 500, Y: 100},
	}}
	nodes := map[string]placedNode{
		"a": {Box: box{X: -80, Y: 60, W: 80, H: 80}},
		"b": {Box: box{X: 500, Y: 60, W: 80, H: 80}},
	}
	route := planDiagonalRoutesWithObstacles(links, 24, nodes)[0]
	if !strings.Contains(route.Path, " Q ") {
		t.Fatalf("clear diagonal route unnecessarily lost its curve: %+v", route)
	}
}

func TestDiagonalPlannerProtectsUnrelatedEndpointStubs(t *testing.T) {
	links := []routedLink{
		{
			Index: 0, FromNode: "dal-pe2", ToNode: "core",
			Start: point{X: 100, Y: 100}, StartSide: "top", StartStub: 100,
			End: point{X: 500, Y: 20}, EndSide: "bottom",
		},
		{
			Index: 1, FromNode: "lax-pe1", ToNode: "other-core",
			Start: point{X: 0, Y: 50}, StartSide: "right",
			End: point{X: 220, Y: 50}, EndSide: "left",
		},
	}
	protected, ok := endpointStubObstacle(links[0].Start, links[0].StartSide, links[0].StartStub, 18)
	if !ok {
		t.Fatal("expected protected endpoint stub obstacle")
	}
	if !routeIntersectsObstacle(routedDiagonalRoute(links[1], 0), protected) {
		t.Fatal("test setup route did not cross protected endpoint stub")
	}

	route := planDiagonalRoutesWithObstacles(links, 72, nil)[1]
	if routeIntersectsObstacle(route, protected) {
		t.Fatalf("route still crosses protected endpoint stub: %+v", route)
	}
}

func TestRouteProximityPenaltyProtectsClearanceWithoutCrossing(t *testing.T) {
	first := directRoute(point{X: 0, Y: 0}, point{X: 400, Y: 0}, "right", "left", "clean")
	near := directRoute(point{X: 0, Y: 12}, point{X: 400, Y: 12}, "right", "left", "clean")
	far := directRoute(point{X: 0, Y: 80}, point{X: 400, Y: 80}, "right", "left", "clean")
	if routeIntersectionCount(first, near) != 0 {
		t.Fatal("near parallel routes unexpectedly intersect")
	}
	if routeProximityPenalty(first, near, 34) <= routeProximityPenalty(first, far, 34) {
		t.Fatal("near route did not receive a larger clearance penalty")
	}
}

func TestGlobalCrossingPlannerIgnoresLinksConvergingAtSameNode(t *testing.T) {
	links := []routedLink{
		{Index: 0, FromNode: "pe", ToNode: "p1", Start: point{X: 0, Y: 0}, End: point{X: 400, Y: 400}},
		{Index: 1, FromNode: "pe", ToNode: "p2", Start: point{X: 0, Y: 0}, End: point{X: 400, Y: 0}},
	}
	routes := planDiagonalRoutes(links)
	if offset := diagonalRouteOffset(routes[0]); offset != 0 {
		t.Fatalf("shared-node route was unnecessarily bent by %.1f", offset)
	}
}

func TestGlobalCrossingPlannerSeparatesSharedNodeLinksWithDistinctAttachments(t *testing.T) {
	links := []routedLink{
		{Index: 0, FromNode: "pe", ToNode: "p1", Start: point{X: 0, Y: 0}, End: point{X: 400, Y: 400}},
		{Index: 1, FromNode: "pe", ToNode: "p2", Start: point{X: 0, Y: 30}, End: point{X: 400, Y: 0}},
	}
	initial := routeIntersectionCount(diagonalRoute(links[0].Start, links[0].End, 0), diagonalRoute(links[1].Start, links[1].End, 0))
	routes := planDiagonalRoutes(links)
	planned := routeIntersectionCount(routes[0], routes[1])
	if initial == 0 || planned >= initial {
		t.Fatalf("shared-node routes with distinct attachments were not separated: initial=%d planned=%d", initial, planned)
	}
}

func TestHubSpokeMultiHomedPEUsesDistinctAttachmentSides(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Nodes: []model.Node{
			{ID: "pe", Role: "edge-router"},
			{ID: "p1", Role: "core-router"},
			{ID: "p2", Role: "core-router"},
		},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "pe", Port: "Hu0/0"}, To: model.LinkEndpoint{Node: "p1", Port: "Hu0/0"}},
			{From: model.LinkEndpoint{Node: "pe", Port: "Hu0/1"}, To: model.LinkEndpoint{Node: "p2", Port: "Hu0/0"}},
		},
	}
	nodes := map[string]placedNode{
		"pe": {ID: "pe", Node: diagram.Nodes[0], Box: box{X: 100, Y: 100, W: 280, H: 82}},
		"p1": {ID: "p1", Node: diagram.Nodes[1], Box: box{X: 500, Y: 500, W: 280, H: 82}},
		"p2": {ID: "p2", Node: diagram.Nodes[2], Box: box{X: 900, Y: 500, W: 280, H: 82}},
	}
	geometry, err := endpointAttachments(diagram, nodes)
	if err != nil {
		t.Fatal(err)
	}
	first := geometry[endpointKey(0, true)]
	second := geometry[endpointKey(1, true)]
	if first.Side == second.Side {
		t.Fatalf("multi-homed PE links share attachment side %q", first.Side)
	}
	if first.Point == second.Point {
		t.Fatalf("multi-homed PE links share attachment point %+v", first.Point)
	}
}

func TestEndpointPositionPinsTerminationAlongSide(t *testing.T) {
	position := 0.25
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke"},
		Nodes: []model.Node{{ID: "pe", Role: "edge-router"}, {ID: "p", Role: "core-router"}},
		Links: []model.Link{{
			From: model.LinkEndpoint{Node: "pe", Port: "Hu0/0", Side: "top", Position: &position},
			To:   model.LinkEndpoint{Node: "p", Port: "Hu0/0"},
		}},
	}
	nodes := map[string]placedNode{
		"pe": {ID: "pe", Node: diagram.Nodes[0], Box: box{X: 100, Y: 100, W: 280, H: 82}},
		"p":  {ID: "p", Node: diagram.Nodes[1], Box: box{X: 500, Y: 500, W: 280, H: 82}},
	}
	geometry, err := endpointAttachments(diagram, nodes)
	if err != nil {
		t.Fatal(err)
	}
	got := geometry[endpointKey(0, true)]
	if got.Side != "top" || got.Point != (point{X: 170, Y: 100}) {
		t.Fatalf("positioned endpoint = %+v, want top at {170 100}", got)
	}
}

func TestEndpointClearanceSeparatesCrowdedTerminations(t *testing.T) {
	firstPosition, secondPosition := 0.48, 0.52
	diagram := &model.Diagram{
		Theme: model.Theme{Layout: "hub-spoke", EndpointClearance: 60},
		Nodes: []model.Node{{ID: "pe", Role: "edge-router"}, {ID: "p1"}, {ID: "p2"}},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "pe", Side: "bottom", Position: &firstPosition}, To: model.LinkEndpoint{Node: "p1"}},
			{From: model.LinkEndpoint{Node: "pe", Side: "bottom", Position: &secondPosition}, To: model.LinkEndpoint{Node: "p2"}},
		},
	}
	nodes := map[string]placedNode{
		"pe": {ID: "pe", Node: diagram.Nodes[0], Box: box{X: 100, Y: 100, W: 280, H: 82}},
		"p1": {ID: "p1", Node: diagram.Nodes[1], Box: box{X: 500, Y: 500, W: 280, H: 82}},
		"p2": {ID: "p2", Node: diagram.Nodes[2], Box: box{X: 900, Y: 500, W: 280, H: 82}},
	}
	geometry, err := endpointAttachments(diagram, nodes)
	if err != nil {
		t.Fatal(err)
	}
	first := geometry[endpointKey(0, true)].Point
	second := geometry[endpointKey(1, true)].Point
	if second.X-first.X < 59.99 {
		t.Fatalf("crowded endpoint clearance = %.1f, want at least 60", second.X-first.X)
	}
}

func TestHubSpokeFanOutPreservesExplicitAttachmentSide(t *testing.T) {
	node := placedNode{Node: model.Node{Role: "edge-router"}, Box: box{X: 100, Y: 100, W: 280, H: 82}}
	items := []attachment{
		{PeerX: 500, PeerY: 500, Side: "left", Pinned: true},
		{PeerX: 900, PeerY: 500, Side: "bottom"},
	}
	got := spreadAttachmentSides(node, items)
	if got[0].Side != "left" {
		t.Fatalf("explicit attachment side changed to %q", got[0].Side)
	}
}

func TestPEAndPDevicesUseDistinctColors(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"pe": {Role: "edge-router"},
			"p":  {Role: "core-router"},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	for _, expected := range []string{`stroke="#d97706"`, `fill="#fffbeb"`, `stroke="#7c3aed"`, `fill="#f5f3ff"`} {
		if !strings.Contains(got, expected) {
			t.Fatalf("render missing PE/P color %s", expected)
		}
	}
}

func TestSiteLayoutWrapsWideCompositionsIntoRows(t *testing.T) {
	diagram := &model.Diagram{Theme: model.Theme{Layout: "sites"}}
	for site := 1; site <= 4; site++ {
		group := model.Group{ID: fmt.Sprintf("site-%d", site), Kind: "site"}
		for node := 1; node <= 4; node++ {
			id := fmt.Sprintf("site-%d-router-%d", site, node)
			group.NodeIDs = append(group.NodeIDs, id)
			diagram.Nodes = append(diagram.Nodes, model.Node{ID: id, Role: "router"})
		}
		diagram.Groups = append(diagram.Groups, group)
	}

	layout := placeSiteLayout(diagram)
	if layout.Width > siteCanvasMax {
		t.Fatalf("wrapped site layout width = %.1f, want at most %.1f", layout.Width, siteCanvasMax)
	}
	firstRowY := layout.Groups[0].Box.Y
	wrapped := false
	for _, group := range layout.Groups[1:] {
		if group.Box.Y > firstRowY {
			wrapped = true
			break
		}
	}
	if !wrapped {
		t.Fatal("expected wide site composition to wrap onto another row")
	}
}

func TestOrthogonalRouteAvoidsDeviceBox(t *testing.T) {
	start := point{X: 100, Y: 100}
	end := point{X: 500, Y: 100}
	nodes := map[string]placedNode{
		"middle": {Box: box{X: 250, Y: 50, W: 100, H: 100}},
	}
	route := orthogonalRoute(start, end, "right", "left", nodes, 0)
	for i := 1; i < len(route.Points); i++ {
		if segmentIntersectsBox(route.Points[i-1], route.Points[i], expandBox(nodes["middle"].Box, 22)) {
			t.Fatalf("route crosses obstacle: %v", route.Points)
		}
	}
	if !strings.Contains(route.Path, " V ") {
		t.Fatalf("expected detour with vertical segments, got %s", route.Path)
	}
}

func TestHorizontalEndpointLabelsFollowTheirAttachmentLanes(t *testing.T) {
	var out bytes.Buffer
	renderEndpointLabel(&out, point{X: 500, Y: 280}, "Hu0/0", "right", 0, model.InterfaceLabelStyle{})
	renderEndpointLabel(&out, point{X: 500, Y: 320}, "Hu0/1", "right", 1, model.InterfaceLabelStyle{})
	if !strings.Contains(out.String(), `y="268.0"`) || !strings.Contains(out.String(), `y="308.0"`) {
		t.Fatalf("horizontal endpoint labels did not follow attachment lanes: %s", out.String())
	}
	if got := strings.Count(out.String(), `x="555.0"`); got != 2 {
		t.Fatalf("horizontal endpoint badge and text positions are not uniform: %s", out.String())
	}
}

func TestOpposingHorizontalEndpointLabelsSitAboveLink(t *testing.T) {
	var out bytes.Buffer
	renderEndpointLabel(&out, point{X: 100, Y: 300}, "Te0/0/0/0", "right", 0, model.InterfaceLabelStyle{})
	renderEndpointLabel(&out, point{X: 260, Y: 300}, "Te0/0/0/1", "left", 0, model.InterfaceLabelStyle{})
	if got := strings.Count(out.String(), `y="288.0"`); got != 2 {
		t.Fatalf("opposing endpoint labels did not sit above the link: %s", out.String())
	}
}

func TestInterfaceLabelBadgeUsesCustomStyle(t *testing.T) {
	var out bytes.Buffer
	style := model.InterfaceLabelStyle{
		Fill: "#fff7ed", Color: "#9a3412", Border: "#fb923c",
		Radius: 8, PaddingX: 12, PaddingY: 6,
	}
	renderEndpointLabel(&out, point{X: 100, Y: 300}, "Te0/0/0/0", "right", 0, style)
	got := out.String()
	for _, expected := range []string{
		`class="interface-label-badge"`,
		`fill="#fff7ed"`,
		`stroke="#fb923c"`,
		`rx="8.0"`,
		`class="interface-label-text"`,
		`fill="#9a3412"`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("custom interface label badge missing %s: %s", expected, got)
		}
	}
}

func TestRenderSanitizesAttributeIdentifiers(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			`router" onclick="alert(1)`: {Role: `router" bad="value`, Color: `#fff" onload="alert(1)`},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	if strings.Contains(got, `" onclick="`) || strings.Contains(got, `" bad="`) || strings.Contains(got, `" onload="`) {
		t.Fatalf("unsafe identifier escaped its SVG attribute: %s", got)
	}
}

func TestRenderCanHideInterfaceLabels(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{InterfaceAt: "none"},
		Nodes:   map[string]spec.Node{"a": {Role: "router"}, "b": {Role: "router"}},
		Links:   []spec.Link{{From: spec.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: spec.LinkEndpoint{Node: "b", Port: "Eth0/1"}}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "interface-label-text") {
		t.Fatal("interface labels rendered despite interface_labels: none")
	}
}

func TestExplicitZeroInterfaceLabelStyleIsPreserved(t *testing.T) {
	zero := 0.0
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{InterfaceLabelStyle: spec.InterfaceLabelStyle{
			Radius: &zero, PaddingX: &zero, PaddingY: &zero,
		}},
		Nodes: map[string]spec.Node{"a": {Role: "router"}, "b": {Role: "router"}},
		Links: []spec.Link{{From: spec.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: spec.LinkEndpoint{Node: "b", Port: "Eth0/1"}}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `rx="0.0"`) {
		t.Fatal("explicit zero interface-label radius was replaced with a default")
	}
}

func TestRowLayoutLeavesReadableLinkSpan(t *testing.T) {
	diagram := &model.Diagram{Theme: model.Theme{Layout: "rows"}}
	for index := 1; index <= 4; index++ {
		diagram.Nodes = append(diagram.Nodes, model.Node{ID: fmt.Sprintf("r%d", index), Role: "router"})
	}
	roles, byRole := groupNodes(diagram)
	layout := layoutDiagram(diagram, roles, byRole)
	for index := 1; index < 4; index++ {
		left := layout.Nodes[fmt.Sprintf("r%d", index)].Box
		right := layout.Nodes[fmt.Sprintf("r%d", index+1)].Box
		if gap := right.X - (left.X + left.W); gap < rowLinkGap {
			t.Fatalf("row link span = %.1f, want at least %.1f", gap, rowLinkGap)
		}
	}
}

func TestLongestSegmentLabelAvoidsEndpointStubs(t *testing.T) {
	points := []point{{X: 100, Y: 100}, {X: 142, Y: 100}, {X: 142, Y: 220}, {X: 500, Y: 220}, {X: 500, Y: 100}}
	got, horizontal := longestSegmentLabel(points)
	if got.X != 321 || got.Y != 220 {
		t.Fatalf("longestSegmentLabel() = %+v, want center of long middle segment", got)
	}
	if !horizontal {
		t.Fatal("longestSegmentLabel() did not report horizontal segment")
	}
}

func TestRouteLabelOffsetsPerpendicularToVerticalSegment(t *testing.T) {
	var out bytes.Buffer
	renderRouteLabel(&out, point{X: 200, Y: 300}, false, "TRUST", "#dc2626", 0)
	if !strings.Contains(out.String(), `x="182.0"`) || !strings.Contains(out.String(), `y="300.0"`) {
		t.Fatalf("vertical route label was not offset sideways: %s", out.String())
	}
}

func TestLabelMaskIsOpaqueAndPadded(t *testing.T) {
	var out bytes.Buffer
	renderLabel(&out, 100, 100, "100G", "#334155", "middle", 11, false)
	if !strings.Contains(out.String(), `class="label-mask"`) || strings.Contains(out.String(), "fill-opacity") {
		t.Fatalf("label mask is not opaque: %s", out.String())
	}
}

func TestMultiplePortsUseDistinctAttachmentPoints(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"spine-01": {Role: "spine"},
			"leaf-01":  {Role: "leaf"},
			"leaf-02":  {Role: "leaf"},
		},
		Links: []spec.Link{
			{From: spec.LinkEndpoint{Node: "spine-01", Port: "Ethernet1/1"}, To: spec.LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/49"}},
			{From: spec.LinkEndpoint{Node: "spine-01", Port: "Ethernet1/2"}, To: spec.LinkEndpoint{Node: "leaf-02", Port: "Ethernet1/49"}},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	roles, byRole := groupNodes(diag)
	nodes := placeNodes(diag, roles, byRole)
	geometry, err := endpointAttachments(diag, nodes)
	if err != nil {
		t.Fatal(err)
	}
	first := geometry[endpointKey(0, true)].Point
	second := geometry[endpointKey(1, true)].Point
	if first.X == second.X {
		t.Fatalf("expected distinct port attachment points, both were %.1f", first.X)
	}
	if first.Y != second.Y {
		t.Fatalf("expected aligned port attachment points, got %.1f and %.1f", first.Y, second.Y)
	}
}

func TestSameLayerLinksUseHorizontalAttachments(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"router-01": {Role: "router"},
			"router-02": {Role: "router"},
		},
		Links: []spec.Link{
			{From: spec.LinkEndpoint{Node: "router-01", Port: "Ethernet0/0"}, To: spec.LinkEndpoint{Node: "router-02", Port: "Ethernet0/0"}},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	roles, byRole := groupNodes(diag)
	nodes := placeNodes(diag, roles, byRole)
	geometry, err := endpointAttachments(diag, nodes)
	if err != nil {
		t.Fatal(err)
	}
	start := geometry[endpointKey(0, true)]
	end := geometry[endpointKey(0, false)]
	if start.Side != "right" || end.Side != "left" {
		t.Fatalf("expected right-to-left attachments, got %s-to-%s", start.Side, end.Side)
	}
	if start.Point.Y != end.Point.Y {
		t.Fatalf("expected horizontally aligned attachments, got %.1f and %.1f", start.Point.Y, end.Point.Y)
	}
	if !strings.Contains(pathData(start.Point, end.Point, start.Side, end.Side, "clean"), " H ") {
		t.Fatal("expected same-layer clean path to use horizontal lead-ins")
	}
}

func TestSameLayerCenterLabelUsesSeparateLane(t *testing.T) {
	var out bytes.Buffer
	start := point{X: 100, Y: 200}
	end := point{X: 300, Y: 200}
	renderCenterLabel(&out, start, end, "right", "left", "Area 0", "#2563eb", 0)
	if !strings.Contains(out.String(), `y="231.0"`) {
		t.Fatalf("expected horizontal center label below interface-label lane, got %s", out.String())
	}
}

func TestBundleTagsRenderOnce(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"leaf-01": {Role: "leaf"},
			"leaf-02": {Role: "leaf"},
			"app-01":  {Role: "server"},
		},
		Links: []spec.Link{
			{From: spec.LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/1"}, To: spec.LinkEndpoint{Node: "app-01", Port: "Ethernet0/0"}, Bundle: "Port-Channel10", LACP: true},
			{From: spec.LinkEndpoint{Node: "leaf-02", Port: "Ethernet1/1"}, To: spec.LinkEndpoint{Node: "app-01", Port: "Ethernet0/1"}, Bundle: "Port-Channel10", LACP: true},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(result), ">Po10</text>"); got != 2 {
		t.Fatalf("expected compact bundle name in marker and legend, got %d", got)
	}
	if !strings.Contains(string(result), "2 links") {
		t.Fatal("expected bundle member count")
	}
	if strings.Contains(string(result), `class="bundle-card"`) {
		t.Fatal("bundle marker must not render a caption box")
	}
	if got := strings.Count(string(result), " L "); got < 2 {
		t.Fatal("expected bundle member paths to route through the LAG marker")
	}
}

func TestExpandedIconLibrary(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Nodes: map[string]spec.Node{
			"aws":      {Role: "public-cloud", Icon: "aws"},
			"router":   {Role: "edge-router", Icon: "router"},
			"firewall": {Role: "firewall", Icon: "firewall"},
			"dwdm":     {Role: "dwdm", Icon: "dwdm"},
		},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	for _, class := range []string{"device-icon-aws", "device-icon-router", "device-icon-firewall", "device-icon-dwdm"} {
		if !strings.Contains(string(result), class) {
			t.Fatalf("render missing %q", class)
		}
	}
}

func TestHubSpokeLayoutWithOrthogonalLinks(t *testing.T) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Layout: "hub-spoke", LinkStyle: "orthogonal"},
		Groups: map[string]*spec.Group{
			"core":  {Label: "Core Site", Kind: "core", Nodes: map[string]interface{}{"core-a": nil}},
			"spoke": {Label: "Spoke Site", Kind: "site", Nodes: map[string]interface{}{"spoke-a": nil}},
		},
		Nodes: map[string]spec.Node{
			"core-a":  {Role: "core-router"},
			"spoke-a": {Role: "edge-router"},
		},
		Links: []spec.Link{{
			From: spec.LinkEndpoint{Node: "core-a", Port: "Hu0/0"},
			To:   spec.LinkEndpoint{Node: "spoke-a", Port: "Hu0/0"},
		}},
	}
	diag, err := model.Compile(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(diag)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	if !strings.Contains(got, " H ") && !strings.Contains(got, " V ") {
		t.Fatal("hub-spoke layout with link_style: orthogonal did not produce orthogonal link segments")
	}
	if strings.Contains(got, " L ") {
		t.Fatal("hub-spoke layout with link_style: orthogonal unexpectedly produced diagonal link segments")
	}
}
