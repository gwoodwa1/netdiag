package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/spec"
	"gopkg.in/yaml.v3"
)

type Template struct {
	ID          string                   `yaml:"id"`
	Version     int                      `yaml:"version"`
	Description string                   `yaml:"description"`
	Params      map[string]TemplateParam `yaml:"params"`
	Groups      map[string]*spec.Group   `yaml:"groups"`
	Nodes       map[string]spec.Node     `yaml:"nodes"`
	Links       []spec.Link              `yaml:"links"`
}

type TemplateParam struct {
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

type TemplateUse struct {
	Template string            `yaml:"template"`
	As       string            `yaml:"as"`
	Params   map[string]string `yaml:"params"`
}

type SourceDocument struct {
	Version int                    `yaml:"version"`
	Diagram spec.Diagram           `yaml:"diagram"`
	Use     []TemplateUse          `yaml:"use,omitempty"`
	Connect []spec.Link            `yaml:"connect,omitempty"`
	Groups  map[string]*spec.Group `yaml:"groups,omitempty"`
	Nodes   map[string]spec.Node   `yaml:"nodes,omitempty"`
	Links   []spec.Link            `yaml:"links,omitempty"`
}

type TemplateLoader interface {
	Load(id string) (*Template, error)
}

type FileTemplateLoader struct {
	Root string
}

type TemplateExpander struct {
	Loader TemplateLoader
}

type ParamValidator struct{}

type ExpansionResult struct {
	Document *spec.Document
	Uses     []TemplateUse
}

var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_-]*)\s*\}\}`)

func Load(path string, loader TemplateLoader) (*ExpansionResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var source SourceDocument
	if err := decodeStrict(data, &source); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	result, err := (&TemplateExpander{Loader: loader}).Expand(&source)
	if err != nil {
		return nil, fmt.Errorf("expand %s: %w", path, err)
	}
	return result, nil
}

func (loader *FileTemplateLoader) Load(id string) (*Template, error) {
	var matches []string
	err := filepath.WalkDir(loader.Root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load templates from %s: %w", loader.Root, err)
	}
	sort.Strings(matches)
	for _, path := range matches {
		template, err := loadTemplateFile(path)
		if err != nil {
			return nil, err
		}
		if template.ID == id {
			return template, nil
		}
	}
	return nil, fmt.Errorf("template %q not found in %s", id, loader.Root)
}

func loadTemplateFile(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var template Template
	if err := decodeStrict(data, &template); err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	if template.ID == "" {
		return nil, fmt.Errorf("template %s has no id", path)
	}
	if template.Version != 1 {
		return nil, fmt.Errorf("template %q version must be 1", template.ID)
	}
	if _, ok := template.Params["instance"]; ok {
		return nil, fmt.Errorf("template %q cannot declare reserved parameter %q", template.ID, "instance")
	}
	return &template, nil
}

func (ParamValidator) Resolve(template *Template, values map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)
	for name, value := range values {
		if _, ok := template.Params[name]; !ok && name != "instance" {
			return nil, fmt.Errorf("template %q has no parameter %q", template.ID, name)
		}
		resolved[name] = value
	}
	for name, param := range template.Params {
		if param.Type != "" && param.Type != "string" {
			return nil, fmt.Errorf("template %q parameter %q type must be string", template.ID, name)
		}
		if _, ok := resolved[name]; !ok && param.Default != "" {
			resolved[name] = param.Default
		}
	}
	for range template.Params {
		changed := false
		for name, value := range resolved {
			next := interpolateString(value, resolved)
			if next != value {
				resolved[name] = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for name, param := range template.Params {
		if param.Required && strings.TrimSpace(resolved[name]) == "" {
			return nil, fmt.Errorf("template %q requires parameter %q", template.ID, name)
		}
		if value, ok := resolved[name]; ok {
			if unresolved := unresolvedPlaceholders(value); len(unresolved) > 0 {
				return nil, fmt.Errorf("template %q parameter %q has unresolved placeholder %q", template.ID, name, unresolved[0])
			}
		}
	}
	return resolved, nil
}

func (expander *TemplateExpander) Expand(source *SourceDocument) (*ExpansionResult, error) {
	if expander.Loader == nil && len(source.Use) > 0 {
		return nil, fmt.Errorf("template loader is required when use is present")
	}
	doc := &spec.Document{
		Version: source.Version,
		Diagram: source.Diagram,
		Groups:  cloneGroups(source.Groups),
		Nodes:   cloneNodes(source.Nodes),
		Links:   append([]spec.Link(nil), source.Links...),
	}
	if doc.Groups == nil {
		doc.Groups = make(map[string]*spec.Group)
	}
	if doc.Nodes == nil {
		doc.Nodes = make(map[string]spec.Node)
	}

	for _, use := range source.Use {
		if strings.TrimSpace(use.As) == "" {
			return nil, fmt.Errorf("template %q use must specify as", use.Template)
		}
		template, err := expander.Loader.Load(use.Template)
		if err != nil {
			return nil, err
		}
		values := make(map[string]string, len(use.Params)+1)
		for name, value := range use.Params {
			values[name] = value
		}
		values["instance"] = use.As
		params, err := (ParamValidator{}).Resolve(template, values)
		if err != nil {
			return nil, err
		}
		expanded, err := interpolateTemplate(template, params)
		if err != nil {
			return nil, fmt.Errorf("template %q as %q: %w", template.ID, use.As, err)
		}
		for id, node := range expanded.Nodes {
			if _, exists := doc.Nodes[id]; exists {
				return nil, fmt.Errorf("template %q as %q produces duplicate node ID %q", template.ID, use.As, id)
			}
			doc.Nodes[id] = node
		}
		for id, group := range expanded.Groups {
			if _, exists := doc.Groups[id]; exists {
				return nil, fmt.Errorf("template %q as %q produces duplicate group ID %q", template.ID, use.As, id)
			}
			doc.Groups[id] = group
		}
		doc.Links = append(doc.Links, expanded.Links...)
	}
	doc.Links = append(doc.Links, source.Connect...)
	if err := spec.Prepare(doc); err != nil {
		return nil, err
	}
	return &ExpansionResult{Document: doc, Uses: append([]TemplateUse(nil), source.Use...)}, nil
}

func interpolateTemplate(template *Template, values map[string]string) (*Template, error) {
	data, err := yaml.Marshal(template)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var unresolved []string
	var interpolateNode func(*yaml.Node)
	interpolateNode = func(node *yaml.Node) {
		if node.Kind == yaml.ScalarNode {
			node.Value = interpolateString(node.Value, values)
			unresolved = append(unresolved, unresolvedPlaceholders(node.Value)...)
		}
		for _, child := range node.Content {
			interpolateNode(child)
		}
	}
	interpolateNode(&root)
	if len(unresolved) > 0 {
		return nil, fmt.Errorf("unresolved placeholder %q", unresolved[0])
	}
	var expanded Template
	if err := root.Decode(&expanded); err != nil {
		return nil, err
	}
	return &expanded, nil
}

func interpolateString(value string, values map[string]string) string {
	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		name := placeholderPattern.FindStringSubmatch(match)[1]
		if replacement, ok := values[name]; ok {
			return replacement
		}
		return match
	})
}

func unresolvedPlaceholders(value string) []string {
	matches := placeholderPattern.FindAllStringSubmatch(value, -1)
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		result = append(result, match[1])
	}
	return result
}

func decodeStrict(data []byte, value interface{}) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	return decoder.Decode(value)
}

func cloneGroups(groups map[string]*spec.Group) map[string]*spec.Group {
	if groups == nil {
		return nil
	}
	data, _ := yaml.Marshal(groups)
	var cloned map[string]*spec.Group
	_ = yaml.Unmarshal(data, &cloned)
	return cloned
}

func cloneNodes(nodes map[string]spec.Node) map[string]spec.Node {
	if nodes == nil {
		return nil
	}
	cloned := make(map[string]spec.Node, len(nodes))
	for id, node := range nodes {
		cloned[id] = node
	}
	return cloned
}
