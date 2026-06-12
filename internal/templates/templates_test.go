package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gwoodwa1/netdiag/internal/spec"
)

type mapLoader map[string]*Template

func (loader mapLoader) Load(id string) (*Template, error) {
	template, ok := loader[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return template, nil
}

func TestLoadTemplateYAMLFile(t *testing.T) {
	template, err := loadTemplateFile(filepath.Join("..", "..", "templates", "site", "dual-pe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if template.ID != "site.dual-pe" || len(template.Nodes) != 2 {
		t.Fatalf("unexpected template: %#v", template)
	}
}

func TestTemplateRegistryListsTemplatesDeterministically(t *testing.T) {
	registry, err := NewTemplateRegistry(filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	items := registry.List()
	if len(items) < 3 {
		t.Fatalf("expected built-in templates, got %#v", items)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].ID >= items[i].ID {
			t.Fatalf("template list is not sorted: %#v", items)
		}
	}
	var dualPE TemplateInfo
	for _, item := range items {
		if item.ID == "site.dual-pe" {
			dualPE = item
		}
	}
	if len(dualPE.RequiredParams) != 1 || dualPE.RequiredParams[0] != "site_label" {
		t.Fatalf("unexpected required params: %#v", dualPE)
	}
	if len(dualPE.OptionalParams) != 2 {
		t.Fatalf("unexpected optional params: %#v", dualPE)
	}
}

func TestTemplateRegistryRejectsDuplicateIDs(t *testing.T) {
	root := t.TempDir()
	writeTemplate(t, root, "one.yaml", "duplicate.id")
	writeTemplate(t, root, "nested/two.yaml", "duplicate.id")

	_, err := NewTemplateRegistry(root)
	if err == nil || !strings.Contains(err.Error(), `duplicate template ID "duplicate.id"`) {
		t.Fatalf("expected duplicate template ID error, got %v", err)
	}
	if !strings.Contains(err.Error(), "one.yaml") || !strings.Contains(err.Error(), "two.yaml") {
		t.Fatalf("duplicate error does not name both files: %v", err)
	}
}

func TestTemplateRegistryJSONUsesArraysForEmptyParamLists(t *testing.T) {
	root := t.TempDir()
	writeTemplate(t, root, "empty-params.yaml", "empty.params")
	registry, err := NewTemplateRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(registry.List())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"required_params":null`) || strings.Contains(string(data), `"optional_params":null`) {
		t.Fatalf("empty parameter lists must be JSON arrays: %s", data)
	}
}

func TestTemplateRegistryCachesParsedTemplates(t *testing.T) {
	root := t.TempDir()
	path := writeTemplate(t, root, "cached.yaml", "cached.id")
	registry, err := NewTemplateRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not: [valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	template, err := registry.Load("cached.id")
	if err != nil {
		t.Fatal(err)
	}
	if template.ID != "cached.id" {
		t.Fatalf("unexpected cached template: %#v", template)
	}
}

func TestParamValidatorRequiresParams(t *testing.T) {
	template := &Template{ID: "required", Params: map[string]TemplateParam{"label": {Type: "string", Required: true}}}
	if _, err := (ParamValidator{}).Resolve(template, nil); err == nil || !strings.Contains(err.Error(), "requires parameter") {
		t.Fatalf("expected required parameter error, got %v", err)
	}
}

func TestParamValidatorAppliesInterpolatedDefaults(t *testing.T) {
	template := &Template{ID: "defaults", Params: map[string]TemplateParam{
		"site_label": {Type: "string", Required: true},
		"pe_label":   {Type: "string", Default: "{{ site_label }} PE"},
		"node_id":    {Type: "string", Default: "{{ instance }}-pe"},
	}}
	params, err := (ParamValidator{}).Resolve(template, map[string]string{"site_label": "London", "instance": "london"})
	if err != nil {
		t.Fatal(err)
	}
	if params["pe_label"] != "London PE" {
		t.Fatalf("got %q", params["pe_label"])
	}
	if params["node_id"] != "london-pe" {
		t.Fatalf("got %q", params["node_id"])
	}
}

func TestParamValidatorRejectsDependencyCycle(t *testing.T) {
	template := &Template{ID: "cycle", Params: map[string]TemplateParam{
		"site":   {Type: "string", Default: "{{ region }}"},
		"region": {Type: "string", Default: "{{ site }}"},
	}}
	_, err := (ParamValidator{}).Resolve(template, nil)
	if err == nil || !strings.Contains(err.Error(), "parameter cycle:") || !strings.Contains(err.Error(), "site") || !strings.Contains(err.Error(), "region") {
		t.Fatalf("expected parameter cycle error, got %v", err)
	}
}

func TestExpandInstanceAndParamPlaceholders(t *testing.T) {
	template := basicTemplate()
	result, err := (&TemplateExpander{Loader: mapLoader{"site": template}}).Expand(&SourceDocument{
		Version: 1,
		Use: []TemplateUse{{
			Template: "site",
			As:       "london",
			Params:   map[string]string{"label": "London: West PE"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document.Nodes["london-pe"].Label != "London: West PE" {
		t.Fatalf("unexpected nodes: %#v", result.Document.Nodes)
	}
	if _, ok := result.Document.Groups["london"]; !ok {
		t.Fatalf("instance group was not expanded: %#v", result.Document.Groups)
	}
}

func TestExpandRejectsDuplicateNodeIDs(t *testing.T) {
	template := basicTemplate()
	_, err := (&TemplateExpander{Loader: mapLoader{"site": template}}).Expand(&SourceDocument{
		Version: 1,
		Use: []TemplateUse{
			{Template: "site", As: "london", Params: map[string]string{"label": "One"}},
			{Template: "site", As: "london", Params: map[string]string{"label": "Two"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate node ID") {
		t.Fatalf("expected duplicate node error, got %v", err)
	}
}

func TestExpandRejectsMissingTemplate(t *testing.T) {
	_, err := (&TemplateExpander{Loader: mapLoader{}}).Expand(&SourceDocument{
		Version: 1,
		Use:     []TemplateUse{{Template: "missing", As: "site"}},
	})
	if err == nil {
		t.Fatal("expected missing template error")
	}
}

func TestExpandRejectsUnresolvedPlaceholder(t *testing.T) {
	template := basicTemplate()
	template.Nodes["{{ instance }}-pe"] = spec.Node{Label: "{{ undeclared }}", Role: "router"}
	_, err := (&TemplateExpander{Loader: mapLoader{"site": template}}).Expand(&SourceDocument{
		Version: 1,
		Use:     []TemplateUse{{Template: "site", As: "london", Params: map[string]string{"label": "London"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "unresolved placeholder") {
		t.Fatalf("expected unresolved placeholder error, got %v", err)
	}
}

func TestExpandCompleteExampleToCanonicalDocument(t *testing.T) {
	result, err := Load(
		filepath.Join("..", "..", "examples", "templates", "mpls-wan-template.yaml"),
		mustRegistry(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Document.Nodes) != 6 || len(result.Document.Groups) != 3 || len(result.Document.Links) != 5 {
		t.Fatalf("unexpected expansion counts: %d nodes, %d groups, %d links",
			len(result.Document.Nodes), len(result.Document.Groups), len(result.Document.Links))
	}
}

func TestExistingNonTemplateExampleStillLoads(t *testing.T) {
	result, err := Load(
		filepath.Join("..", "..", "examples", "spine-leaf.yaml"),
		mustRegistry(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Document.Nodes) == 0 {
		t.Fatal("expected canonical nodes")
	}
}

func TestIncludedTemplateFragmentsExpand(t *testing.T) {
	result, err := Load(
		filepath.Join("..", "..", "examples", "includes", "mpls-wan.yaml"),
		mustRegistry(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Document.Nodes) != 6 || len(result.Document.Groups) != 3 || len(result.Document.Links) != 5 {
		t.Fatalf("unexpected included expansion counts: %d nodes, %d groups, %d links",
			len(result.Document.Nodes), len(result.Document.Groups), len(result.Document.Links))
	}
}

func mustRegistry(t *testing.T) *TemplateRegistry {
	t.Helper()
	registry, err := NewTemplateRegistry(filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func writeTemplate(t *testing.T, root, name, id string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "id: " + id + "\nversion: 1\ndescription: Test template\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConnectRejectsUnknownExpandedNode(t *testing.T) {
	_, err := (&TemplateExpander{}).Expand(&SourceDocument{
		Version: 1,
		Nodes:   map[string]spec.Node{"known": {Role: "router"}},
		Connect: []spec.Link{{
			From: spec.LinkEndpoint{Node: "known", Port: "Eth0/0"},
			To:   spec.LinkEndpoint{Node: "missing", Port: "Eth0/0"},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown node") {
		t.Fatalf("expected unknown connect node error, got %v", err)
	}
}

func basicTemplate() *Template {
	return &Template{
		ID:      "site",
		Version: 1,
		Params: map[string]TemplateParam{
			"label": {Type: "string", Required: true},
		},
		Groups: map[string]*spec.Group{
			"{{ instance }}": {Label: "{{ label }}", Kind: "site", Nodes: map[string]interface{}{"{{ instance }}-pe": nil}},
		},
		Nodes: map[string]spec.Node{
			"{{ instance }}-pe": {Label: "{{ label }}", Role: "router"},
		},
	}
}
