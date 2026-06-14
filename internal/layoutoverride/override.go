package layoutoverride

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

type Document struct {
	Version         int       `yaml:"version"`
	LayoutOverrides Overrides `yaml:"layout_overrides"`
}

type Overrides struct {
	Nodes  map[string]Bounds `yaml:"nodes,omitempty"`
	Groups map[string]Bounds `yaml:"groups,omitempty"`
	Links  map[string]Link   `yaml:"links,omitempty"`
}

type Bounds struct {
	X      *float64 `yaml:"x,omitempty"`
	Y      *float64 `yaml:"y,omitempty"`
	Width  *float64 `yaml:"width,omitempty"`
	Height *float64 `yaml:"height,omitempty"`
	Locked bool     `yaml:"locked,omitempty"`
	Style  string   `yaml:"style,omitempty"`
}

type Link struct {
	SourceSide string  `yaml:"source_side,omitempty"`
	TargetSide string  `yaml:"target_side,omitempty"`
	Waypoints  []Point `yaml:"waypoints,omitempty"`
	Locked     bool    `yaml:"locked,omitempty"`
	Style      string  `yaml:"style,omitempty"`
}

type Point struct {
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
}

func Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc Document
	if err := yamlutil.DecodeStrict(data, &doc); err != nil {
		return nil, fmt.Errorf("parse layout overrides %s: %w", path, err)
	}
	if err := Validate(&doc); err != nil {
		return nil, fmt.Errorf("validate layout overrides %s: %w", path, err)
	}
	return &doc, nil
}

func Format(doc *Document) ([]byte, error) {
	if err := Validate(doc); err != nil {
		return nil, err
	}
	return yaml.Marshal(doc)
}

func Validate(doc *Document) error {
	var problems []string
	if doc.Version != 1 {
		problems = append(problems, "version must be 1")
	}
	validateBounds := func(kind string, values map[string]Bounds) {
		for id, value := range values {
			if strings.TrimSpace(id) == "" {
				problems = append(problems, kind+" ID cannot be empty")
			}
			if value.Width != nil && *value.Width <= 0 {
				problems = append(problems, fmt.Sprintf("%s %q width must be greater than zero", kind, id))
			}
			if value.Height != nil && *value.Height <= 0 {
				problems = append(problems, fmt.Sprintf("%s %q height must be greater than zero", kind, id))
			}
		}
	}
	validateBounds("node", doc.LayoutOverrides.Nodes)
	validateBounds("group", doc.LayoutOverrides.Groups)
	for id, link := range doc.LayoutOverrides.Links {
		if strings.TrimSpace(id) == "" {
			problems = append(problems, "link ID cannot be empty")
		}
		for name, side := range map[string]string{"source_side": link.SourceSide, "target_side": link.TargetSide} {
			if side != "" && side != "top" && side != "right" && side != "bottom" && side != "left" {
				problems = append(problems, fmt.Sprintf("link %q %s must be top, right, bottom, or left", id, name))
			}
		}
		if link.Style != "" && link.Style != "orthogonal" && link.Style != "straight" && link.Style != "curved" {
			problems = append(problems, fmt.Sprintf("link %q style must be orthogonal, straight, or curved", id))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}
