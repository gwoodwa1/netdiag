package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/source"
	"github.com/gwoodwa1/netdiag/internal/spec"
	"github.com/gwoodwa1/netdiag/internal/yamlutil"
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

type TemplateUse = source.TemplateUse
type SourceDocument = source.Document

type TemplateLoader interface {
	Load(id string) (*Template, error)
}

type TemplateInfo struct {
	ID             string   `json:"id"`
	Version        int      `json:"version"`
	Description    string   `json:"description"`
	RequiredParams []string `json:"required_params"`
	OptionalParams []string `json:"optional_params"`
}

type TemplateRegistry struct {
	root      string
	templates map[string]*Template
	infos     []TemplateInfo
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
	document, err := source.Load(path)
	if err != nil {
		return nil, err
	}
	result, err := (&TemplateExpander{Loader: loader}).Expand(document)
	if err != nil {
		return nil, fmt.Errorf("expand %s: %w", path, err)
	}
	return result, nil
}

func NewTemplateRegistry(root string) (*TemplateRegistry, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load templates from %s: %w", root, err)
	}
	sort.Strings(matches)
	registry := &TemplateRegistry{
		root:      root,
		templates: make(map[string]*Template),
	}
	paths := make(map[string]string)
	for _, path := range matches {
		template, err := loadTemplateFile(path)
		if err != nil {
			return nil, err
		}
		if existingPath, exists := paths[template.ID]; exists {
			return nil, fmt.Errorf("duplicate template ID %q in %s and %s", template.ID, existingPath, path)
		}
		paths[template.ID] = path
		registry.templates[template.ID] = template
		registry.infos = append(registry.infos, templateInfo(template))
	}
	sort.Slice(registry.infos, func(i, j int) bool {
		return registry.infos[i].ID < registry.infos[j].ID
	})
	return registry, nil
}

func (registry *TemplateRegistry) Load(id string) (*Template, error) {
	template, ok := registry.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %q not found in %s", id, registry.root)
	}
	return template, nil
}

func (registry *TemplateRegistry) List() []TemplateInfo {
	items := make([]TemplateInfo, len(registry.infos))
	for i, info := range registry.infos {
		items[i] = info
		items[i].RequiredParams = make([]string, len(info.RequiredParams))
		copy(items[i].RequiredParams, info.RequiredParams)
		items[i].OptionalParams = make([]string, len(info.OptionalParams))
		copy(items[i].OptionalParams, info.OptionalParams)
	}
	return items
}

func templateInfo(template *Template) TemplateInfo {
	info := TemplateInfo{
		ID:             template.ID,
		Version:        template.Version,
		Description:    template.Description,
		RequiredParams: make([]string, 0),
		OptionalParams: make([]string, 0),
	}
	for name, param := range template.Params {
		if param.Required {
			info.RequiredParams = append(info.RequiredParams, name)
		} else {
			info.OptionalParams = append(info.OptionalParams, name)
		}
	}
	sort.Strings(info.RequiredParams)
	sort.Strings(info.OptionalParams)
	return info
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
	if err := validateParamCycles(template, resolved); err != nil {
		return nil, err
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

func validateParamCycles(template *Template, values map[string]string) error {
	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[string]int)
	var stack []string
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == visited {
			return nil
		}
		if state[name] == visiting {
			start := 0
			for stack[start] != name {
				start++
			}
			cycle := append(append([]string(nil), stack[start:]...), name)
			return fmt.Errorf("template %q parameter cycle: %s", template.ID, strings.Join(cycle, " -> "))
		}
		state[name] = visiting
		stack = append(stack, name)
		for _, dependency := range unresolvedPlaceholders(values[name]) {
			if _, ok := template.Params[dependency]; ok {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = visited
		return nil
	}
	for name := range template.Params {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
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
	return yamlutil.DecodeStrict(data, value)
}

func cloneGroups(groups map[string]*spec.Group) map[string]*spec.Group {
	if groups == nil {
		return nil
	}
	cloned := make(map[string]*spec.Group, len(groups))
	for id, group := range groups {
		if group == nil {
			cloned[id] = nil
			continue
		}
		cloned[id] = &spec.Group{
			Label:  group.Label,
			Kind:   group.Kind,
			Nodes:  cloneStringMap(group.Nodes),
			Groups: cloneGroups(group.Groups),
		}
	}
	return cloned
}

func cloneStringMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneStringMap(typed)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for index, item := range typed {
			cloned[index] = cloneValue(item)
		}
		return cloned
	default:
		return value
	}
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
