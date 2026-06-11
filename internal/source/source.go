package source

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/spec"
	"gopkg.in/yaml.v3"
)

type TemplateUse struct {
	Template string            `yaml:"template"`
	As       string            `yaml:"as"`
	Params   map[string]string `yaml:"params"`
}

type Document struct {
	Version int                    `yaml:"version"`
	Diagram spec.Diagram           `yaml:"diagram,omitempty"`
	Include []string               `yaml:"include,omitempty"`
	Use     []TemplateUse          `yaml:"use,omitempty"`
	Connect []spec.Link            `yaml:"connect,omitempty"`
	Groups  map[string]*spec.Group `yaml:"groups,omitempty"`
	Nodes   map[string]spec.Node   `yaml:"nodes,omitempty"`
	Links   []spec.Link            `yaml:"links,omitempty"`
}

type Resolver struct {
	Root string
}

func Load(path string) (*Document, error) {
	absolute, err := canonicalPath(path)
	if err != nil {
		return nil, err
	}
	resolver := &Resolver{Root: filepath.Dir(absolute)}
	return resolver.Load(absolute)
}

// Format preserves the authored document structure without resolving includes
// or expanding templates.
func Format(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (resolver *Resolver) Load(path string) (*Document, error) {
	root, err := canonicalPath(resolver.Root)
	if err != nil {
		return nil, err
	}
	absolute, err := canonicalPath(path)
	if err != nil {
		return nil, err
	}
	return resolver.load(root, absolute, nil)
}

func (resolver *Resolver) load(root, path string, stack []string) (*Document, error) {
	if err := withinRoot(root, path); err != nil {
		return nil, err
	}
	for _, active := range stack {
		if active == path {
			cycle := append(append([]string(nil), stack...), path)
			for i := range cycle {
				cycle[i] = filepath.Base(cycle[i])
			}
			return nil, fmt.Errorf("include cycle: %s", strings.Join(cycle, " -> "))
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var current Document
	if err := decodeStrict(data, &current); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if current.Version != 1 {
		return nil, fmt.Errorf("parse %s: version must be 1", path)
	}

	merged := &Document{
		Version: 1,
		Groups:  make(map[string]*spec.Group),
		Nodes:   make(map[string]spec.Node),
	}
	nextStack := append(append([]string(nil), stack...), path)
	for _, include := range current.Include {
		includePath, err := resolveInclude(root, filepath.Dir(path), include)
		if err != nil {
			return nil, fmt.Errorf("include %q from %s: %w", include, path, err)
		}
		included, err := resolver.load(root, includePath, nextStack)
		if err != nil {
			return nil, err
		}
		if err := merge(merged, included, includePath); err != nil {
			return nil, err
		}
	}
	if err := merge(merged, &current, path); err != nil {
		return nil, err
	}
	merged.Diagram = current.Diagram
	merged.Include = nil
	return merged, nil
}

func merge(target, incoming *Document, sourcePath string) error {
	for id, group := range incoming.Groups {
		if _, exists := target.Groups[id]; exists {
			return fmt.Errorf("include %s produces duplicate group ID %q", sourcePath, id)
		}
		target.Groups[id] = group
	}
	for id, node := range incoming.Nodes {
		if _, exists := target.Nodes[id]; exists {
			return fmt.Errorf("include %s produces duplicate node ID %q", sourcePath, id)
		}
		target.Nodes[id] = node
	}
	target.Use = append(target.Use, incoming.Use...)
	target.Links = append(target.Links, incoming.Links...)
	target.Connect = append(target.Connect, incoming.Connect...)
	return nil
}

func resolveInclude(root, parent, include string) (string, error) {
	if strings.TrimSpace(include) == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	if filepath.IsAbs(include) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	path, err := canonicalPath(filepath.Join(parent, include))
	if err != nil {
		return "", err
	}
	if err := withinRoot(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func withinRoot(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %s escapes project root %s", path, root)
	}
	return nil
}

func decodeStrict(data []byte, value interface{}) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	return decoder.Decode(value)
}
