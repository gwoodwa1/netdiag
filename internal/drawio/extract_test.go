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
}
