package drawio

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/layoutoverride"
	"github.com/gwoodwa1/netdiag/internal/model"
)

func TestExtractOverridesRoundTrip(t *testing.T) {
	x, y, width, height := 120.0, 80.0, 240.0, 90.0
	groupX, groupY := 40.0, 50.0
	diagram := &model.Diagram{
		Groups: []model.Group{{ID: "site-a", NodeIDs: []string{"a"}}},
		Nodes:  []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{{
			ID: "core-link", From: model.LinkEndpoint{Node: "a"}, To: model.LinkEndpoint{Node: "b"},
		}},
	}
	expected := &layoutoverride.Document{
		Version: 1,
		LayoutOverrides: layoutoverride.Overrides{
			Nodes: map[string]layoutoverride.Bounds{"a": {
				X: &x, Y: &y, Width: &width, Height: &height, Locked: true,
			}},
			Groups: map[string]layoutoverride.Bounds{"site-a": {X: &groupX, Y: &groupY}},
			Links: map[string]layoutoverride.Link{"core-link": {
				SourceSide: "right", TargetSide: "left", Style: "curved", Locked: true,
				Waypoints: []layoutoverride.Point{{X: 0, Y: 125}, {X: 400, Y: 125}},
			}},
		},
	}
	rendered, err := RenderWithOptions(diagram, Options{Overrides: expected})
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := ExtractOverrides(rendered, diagram)
	if err != nil {
		t.Fatal(err)
	}

	node := extracted.LayoutOverrides.Nodes["a"]
	if *node.X != x || *node.Y != y || *node.Width != width || *node.Height != height || !node.Locked {
		t.Fatalf("unexpected extracted node: %+v", node)
	}
	group := extracted.LayoutOverrides.Groups["site-a"]
	if *group.X != groupX || *group.Y != groupY {
		t.Fatalf("unexpected extracted group: %+v", group)
	}
	link := extracted.LayoutOverrides.Links["core-link"]
	if link.SourceSide != "right" || link.TargetSide != "left" || link.Style != "curved" || !link.Locked {
		t.Fatalf("unexpected extracted link: %+v", link)
	}
	if len(link.Waypoints) != 2 || link.Waypoints[0].X != 0 || link.Waypoints[1].X != 400 {
		t.Fatalf("unexpected extracted waypoints: %+v", link.Waypoints)
	}
	if _, ok := extracted.LayoutOverrides.Nodes["b"]; !ok {
		t.Fatal("expected generated position for node b to be extracted")
	}
	rerendered, err := RenderWithOptions(diagram, Options{Overrides: extracted})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rendered, rerendered) {
		t.Fatal("render -> extract -> render did not produce byte-identical draw.io output")
	}
}

func TestDecodeCompressedPage(t *testing.T) {
	graph := `<mxGraphModel><root><mxCell id="0"></mxCell></root></mxGraphModel>`
	var compressed bytes.Buffer
	writer, err := flate.NewWriter(&compressed, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(url.QueryEscape(graph))); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(compressed.Bytes())
	got, err := decodePage(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != graph {
		t.Fatalf("decoded graph = %q, want %q", got, graph)
	}
}

func TestExtractOverridesIgnoresUnmanagedCellsAndRejectsUnknownManagedIDs(t *testing.T) {
	diagram := &model.Diagram{Nodes: []model.Node{{ID: "known"}}}
	unmanaged := []byte(`<mxfile><diagram><mxGraphModel><root><mxCell id="freehand"></mxCell></root></mxGraphModel></diagram></mxfile>`)
	result, err := ExtractOverrides(unmanaged, diagram)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LayoutOverrides.Nodes) != 0 {
		t.Fatalf("unmanaged cell was extracted: %+v", result.LayoutOverrides.Nodes)
	}

	unknown := strings.ReplaceAll(string(unmanaged), `id="freehand"`, `id="node-missing" netdiag-id="missing" netdiag-kind="node"`)
	if _, err := ExtractOverrides([]byte(unknown), diagram); err == nil {
		t.Fatal("unknown managed node was accepted")
	}

	wrapped := []byte(`<mxfile><diagram><mxGraphModel><root><object label="managed" netdiag-id="known" netdiag-kind="node"><mxCell id="node-known"><mxGeometry x="10" y="20" width="30" height="40"></mxGeometry></mxCell></object></root></mxGraphModel></diagram></mxfile>`)
	result, err = ExtractOverrides(wrapped, diagram)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.LayoutOverrides.Nodes["known"]; got.X == nil || *got.X != 10 {
		t.Fatalf("wrapped managed cell was not extracted: %+v", got)
	}
}

func TestExtractOverridesWithReportSummarizesIgnoredAndMissingObjects(t *testing.T) {
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "known"}, {ID: "missing"}},
		Links: []model.Link{{
			ID: "current-link", From: model.LinkEndpoint{Node: "known"}, To: model.LinkEndpoint{Node: "missing"},
		}},
	}
	data := []byte(`<mxfile><diagram><mxGraphModel><root>
		<mxCell id="0"></mxCell>
		<mxCell id="known" netdiag-id="known" netdiag-kind="node" vertex="1"><mxGeometry x="10" y="20" width="30" height="40"></mxGeometry></mxCell>
		<mxCell id="stale-link" netdiag-id="stale-link" netdiag-kind="link" edge="1"><mxGeometry></mxGeometry></mxCell>
		<mxCell id="generated-label" netdiag-id="label:known" netdiag-kind="label" vertex="1"></mxCell>
		<mxCell id="annotation" value="note" style="text;html=1;" vertex="1"></mxCell>
		<mxCell id="shape" style="rounded=1;" vertex="1"></mxCell>
		<mxCell id="connector" edge="1"></mxCell>
	</root></mxGraphModel></diagram></mxfile>`)

	result, report, err := ExtractOverridesWithReport(data, diagram)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LayoutOverrides.Nodes) != 1 || report.Managed.Nodes != 1 {
		t.Fatalf("unexpected managed nodes: result=%+v report=%+v", result.LayoutOverrides.Nodes, report.Managed)
	}
	if report.Ignored.Annotations != 1 || report.Ignored.DecorativeShapes != 1 || report.Ignored.Connectors != 1 || report.Ignored.UnknownManaged != 1 {
		t.Fatalf("unexpected ignored counts: %+v", report.Ignored)
	}
	got := strings.Join(report.Warnings, "\n")
	for _, want := range []string{
		"link current-link exists in source but was not found in draw.io",
		"link stale-link exists in draw.io but source topology no longer contains it",
		"node missing exists in source but was not found in draw.io",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report warnings %q do not contain %q", got, want)
		}
	}
	formatted := FormatExtractionReport(report)
	if !strings.Contains(formatted, "1 manually added connectors without netdiag-id") || !strings.Contains(formatted, "Warnings:") {
		t.Fatalf("unexpected formatted report:\n%s", formatted)
	}
}

func TestTopologyGrowthPreservesExistingLayoutAndPlacesNewObjectsDeterministically(t *testing.T) {
	coreAX, coreAY := 320.0, 180.0
	coreBX, coreBY := 760.0, 180.0
	v1 := &model.Diagram{
		Nodes: []model.Node{{ID: "core-a"}, {ID: "core-b"}},
		Links: []model.Link{{
			ID: "core-link", From: model.LinkEndpoint{Node: "core-a"}, To: model.LinkEndpoint{Node: "core-b"},
		}},
	}
	polished := &layoutoverride.Document{
		Version: 1,
		LayoutOverrides: layoutoverride.Overrides{
			Nodes: map[string]layoutoverride.Bounds{
				"core-a": {X: &coreAX, Y: &coreAY},
				"core-b": {X: &coreBX, Y: &coreBY},
			},
			Links: map[string]layoutoverride.Link{
				"core-link": {Waypoints: []layoutoverride.Point{{X: 575, Y: 215}}},
			},
		},
	}
	renderedV1, err := RenderWithOptions(v1, Options{Overrides: polished})
	if err != nil {
		t.Fatal(err)
	}
	extractedV1, err := ExtractOverrides(renderedV1, v1)
	if err != nil {
		t.Fatal(err)
	}

	v2 := &model.Diagram{
		Nodes: []model.Node{{ID: "core-a"}, {ID: "core-b"}, {ID: "edge-01"}},
		Links: []model.Link{
			{ID: "core-link", From: model.LinkEndpoint{Node: "core-a"}, To: model.LinkEndpoint{Node: "core-b"}},
			{ID: "edge-link", From: model.LinkEndpoint{Node: "core-a"}, To: model.LinkEndpoint{Node: "edge-01"}},
		},
	}
	firstV2, err := RenderWithOptions(v2, Options{Overrides: extractedV1})
	if err != nil {
		t.Fatal(err)
	}
	secondV2, err := RenderWithOptions(v2, Options{Overrides: extractedV1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstV2, secondV2) {
		t.Fatal("new topology objects were not placed deterministically")
	}

	extractedV2, err := ExtractOverrides(firstV2, v2)
	if err != nil {
		t.Fatal(err)
	}
	for id, expected := range map[string]layoutoverride.Bounds{
		"core-a": extractedV1.LayoutOverrides.Nodes["core-a"],
		"core-b": extractedV1.LayoutOverrides.Nodes["core-b"],
	} {
		got := extractedV2.LayoutOverrides.Nodes[id]
		if *got.X != *expected.X || *got.Y != *expected.Y || *got.Width != *expected.Width || *got.Height != *expected.Height {
			t.Fatalf("existing node %s moved after topology growth: got=%+v expected=%+v", id, got, expected)
		}
	}
	gotCoreLink := extractedV2.LayoutOverrides.Links["core-link"]
	wantCoreLink := extractedV1.LayoutOverrides.Links["core-link"]
	if len(gotCoreLink.Waypoints) != 1 || gotCoreLink.Waypoints[0] != wantCoreLink.Waypoints[0] {
		t.Fatalf("existing link route changed after topology growth: got=%+v expected=%+v", gotCoreLink, wantCoreLink)
	}
	edge := extractedV2.LayoutOverrides.Nodes["edge-01"]
	if edge.X == nil || edge.Y == nil {
		t.Fatalf("new node was not auto-placed: %+v", edge)
	}
	if *edge.X != coreAX || *edge.Y != coreAY+70+80 {
		t.Fatalf("new node was not placed near its managed neighbor: %+v", edge)
	}
	if _, ok := extractedV2.LayoutOverrides.Links["edge-link"]; !ok {
		t.Fatal("new link was not rendered")
	}
}
