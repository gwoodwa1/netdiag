package interactive

import (
	"strings"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/model"
)

func TestRenderProducesSingleFileInteractivePreview(t *testing.T) {
	diagram := &model.Diagram{
		Theme: model.Theme{Title: `Core <Preview>`},
		Groups: []model.Group{
			{ID: "site/a", Label: "Site A", Kind: "site"},
			{ID: "rack:a", Label: "Rack A", Kind: "rack", ParentID: "site/a", NodeIDs: []string{"router/a"}},
		},
		Nodes: []model.Node{{ID: "router/a", Label: "Router A", Role: "router", Metadata: map[string]interface{}{"vendor": "Example"}}},
		Links: []model.Link{{From: model.LinkEndpoint{Node: "router/a", Port: "Eth0/0"}, To: model.LinkEndpoint{Node: "router/a", Port: "Lo0"}, Label: "self-test"}},
	}
	svg := []byte(`<svg viewBox="0 0 100 100"><g id="group-site-a" data-netdiag-kind="group"></g><g id="router-a" data-netdiag-kind="node"></g><g id="link-1" data-netdiag-kind="link"></g></svg>`)

	result, err := Render(diagram, svg)
	if err != nil {
		t.Fatal(err)
	}
	html := string(result)
	for _, expected := range []string{
		`<!doctype html>`, `Core &lt;Preview&gt;`, `<svg viewBox="0 0 100 100">`,
		`"domId":"router-a"`, `"domId":"group-site-a"`, `"vendor":"Example"`,
		`Wheel to zoom`, `function refresh()`, `classList.toggle('is-hidden'`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("interactive HTML missing %q", expected)
		}
	}
	if !strings.Contains(html, `"nodeIds":["router/a"]`) {
		t.Fatal("root group must include descendant nodes for collapse")
	}
}

func TestRenderEscapesScriptTerminatingMetadata(t *testing.T) {
	diagram := &model.Diagram{Nodes: []model.Node{{ID: "node", Role: "router", Metadata: map[string]interface{}{"note": "</script>"}}}}
	result, err := Render(diagram, []byte(`<svg/>`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), `{"note":"</script>"}`) {
		t.Fatal("embedded JSON must escape script-terminating metadata")
	}
}
