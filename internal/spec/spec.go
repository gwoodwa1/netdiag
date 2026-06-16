package spec

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/yamlutil"
	"gopkg.in/yaml.v3"
)

type Document struct {
	Version int               `yaml:"version"`
	Diagram Diagram           `yaml:"diagram,omitempty"`
	Groups  map[string]*Group `yaml:"groups,omitempty"`
	Nodes   map[string]Node   `yaml:"nodes,omitempty"`
	Links   []Link            `yaml:"links,omitempty"`
}

type Diagram struct {
	Title               string              `yaml:"title,omitempty"`
	Subtitle            string              `yaml:"subtitle,omitempty"`
	Badge               string              `yaml:"badge,omitempty"`
	Layout              string              `yaml:"layout,omitempty"`
	Direction           string              `yaml:"direction,omitempty"`
	LinkStyle           string              `yaml:"link_style,omitempty"`
	RouteClearance      float64             `yaml:"route_clearance,omitempty"`
	EndpointClearance   float64             `yaml:"endpoint_clearance,omitempty"`
	InterfaceAt         string              `yaml:"interface_labels,omitempty"`
	Theme               string              `yaml:"theme,omitempty"`
	Renderer            string              `yaml:"renderer,omitempty"`
	LinkStyles          LinkStyleRules      `yaml:"link_styles,omitempty"`
	InterfaceLabelStyle InterfaceLabelStyle `yaml:"interface_label_style,omitempty"`
}

type InterfaceLabelStyle struct {
	Fill     string   `yaml:"fill,omitempty"`
	Color    string   `yaml:"color,omitempty"`
	Border   string   `yaml:"border,omitempty"`
	Radius   *float64 `yaml:"radius,omitempty"`
	PaddingX *float64 `yaml:"padding_x,omitempty"`
	PaddingY *float64 `yaml:"padding_y,omitempty"`
}

type LinkStyleRules struct {
	Protocol map[string]VisualStyle `yaml:"protocol,omitempty"`
	Status   map[string]VisualStyle `yaml:"status,omitempty"`
}

type VisualStyle struct {
	Color   string  `yaml:"color,omitempty"`
	Pattern string  `yaml:"pattern,omitempty"`
	Width   float64 `yaml:"width,omitempty"`
}

type Group struct {
	Label  string                 `yaml:"label,omitempty"`
	Kind   string                 `yaml:"kind,omitempty"`
	Groups map[string]*Group      `yaml:"groups,omitempty"`
	Nodes  map[string]interface{} `yaml:"nodes,omitempty"`
}

type Node struct {
	Label     string                 `yaml:"label,omitempty"`
	Role      string                 `yaml:"role"`
	Icon      string                 `yaml:"icon,omitempty"`
	IconLabel string                 `yaml:"icon_label,omitempty"`
	Color     string                 `yaml:"color,omitempty"`
	Width     float64                `yaml:"width,omitempty"`
	Height    float64                `yaml:"height,omitempty"`
	Order     int                    `yaml:"order,omitempty"`
	Metadata  map[string]interface{} `yaml:"metadata,omitempty"`
}

type LinkEndpoint struct {
	Node          string   `yaml:"node"`
	Port          string   `yaml:"port"`
	Side          string   `yaml:"side,omitempty"`
	Position      *float64 `yaml:"position,omitempty"`
	Stub          float64  `yaml:"stub,omitempty"`
	LabelRotation int      `yaml:"label_rotation,omitempty"`
	LabelAlong    *float64 `yaml:"label_along,omitempty"`
	LabelOffset   *float64 `yaml:"label_offset,omitempty"`
	Label         string   `yaml:"label,omitempty"`
	Address       string   `yaml:"address,omitempty"`
}

func (le *LinkEndpoint) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		ep, err := ParseEndpoint(s)
		if err != nil {
			return err
		}
		le.Node = ep.Node
		le.Port = ep.Port
		return nil
	}

	type rawLinkEndpoint struct {
		Node          string   `yaml:"node"`
		Port          string   `yaml:"port"`
		Side          string   `yaml:"side"`
		Position      *float64 `yaml:"position"`
		Stub          float64  `yaml:"stub"`
		LabelRotation int      `yaml:"label_rotation"`
		LabelAlong    *float64 `yaml:"label_along"`
		LabelOffset   *float64 `yaml:"label_offset"`
		Label         string   `yaml:"label"`
		Address       string   `yaml:"address"`
	}
	var raw rawLinkEndpoint
	if err := value.Decode(&raw); err != nil {
		return err
	}
	le.Node = raw.Node
	le.Port = raw.Port
	le.Side = raw.Side
	le.Position = raw.Position
	le.Stub = raw.Stub
	le.LabelRotation = raw.LabelRotation
	le.LabelAlong = raw.LabelAlong
	le.LabelOffset = raw.LabelOffset
	le.Label = raw.Label
	le.Address = raw.Address
	return nil
}

type Link struct {
	ID           string       `yaml:"id,omitempty"`
	From         LinkEndpoint `yaml:"from"`
	To           LinkEndpoint `yaml:"to"`
	Label        string       `yaml:"label,omitempty"`
	Style        string       `yaml:"style,omitempty"`
	Protocol     string       `yaml:"protocol,omitempty"`
	Status       string       `yaml:"status,omitempty"`
	Bundle       string       `yaml:"bundle,omitempty"`
	LACP         bool         `yaml:"lacp,omitempty"`
	MultiChassis bool         `yaml:"multi_chassis,omitempty"`
	Trunk        *Trunk       `yaml:"trunk,omitempty"`
	Labels       *LinkLabels  `yaml:"labels,omitempty"`
}

type LinkLabels struct {
	Source string `yaml:"source,omitempty"`
	Middle string `yaml:"middle,omitempty"`
	Target string `yaml:"target,omitempty"`
}

type Trunk struct {
	Encapsulation string   `yaml:"encapsulation,omitempty"`
	AllowedVLANs  []string `yaml:"allowed_vlans,omitempty"`
}

type Endpoint struct {
	Node string
	Port string
}

var interfaceParts = regexp.MustCompile(`^([A-Za-z][A-Za-z -]*?)([0-9].*)$`)

var interfaceAbbreviations = map[string]string{
	"ethernet":               "Eth",
	"fastethernet":           "Fa",
	"gigabitethernet":        "Gi",
	"tengig":                 "Te",
	"tengige":                "Te",
	"tengigabitethernet":     "Te",
	"tengigectrlr":           "Te",
	"twentyfivegige":         "Twe",
	"twentyfivegigethernet":  "Twe",
	"fortygige":              "Fo",
	"fortygigabitethernet":   "Fo",
	"hundredgige":            "Hu",
	"hundredgigabitethernet": "Hu",
	"fourhundredgige":        "Four",
	"portchannel":            "Po",
	"management":             "Mgmt",
	"loopback":               "Lo",
	"vlan":                   "Vl",
}

func Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc Document
	if err := yamlutil.DecodeStrict(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := Prepare(&doc); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &doc, nil
}

// Prepare applies canonical defaults and validates a document before compile or rendering.
func Prepare(doc *Document) error {
	applyDefaults(doc)
	return Validate(doc)
}

func ParseEndpoint(value string) (Endpoint, error) {
	node, port, ok := strings.Cut(value, ":")
	if !ok {
		return Endpoint{}, fmt.Errorf("endpoint %q must use node:interface", value)
	}
	node, port = strings.TrimSpace(node), strings.TrimSpace(port)
	if node == "" || port == "" {
		return Endpoint{}, fmt.Errorf("endpoint %q must include both node and interface", value)
	}
	return Endpoint{Node: node, Port: port}, nil
}

// DisplayPort returns a compact interface label while preserving the complete
// interface name in the source document and internal model.
func DisplayPort(port string) string {
	port = strings.TrimSpace(port)
	parts := interfaceParts.FindStringSubmatch(port)
	if len(parts) != 3 {
		return port
	}

	prefix := strings.TrimSpace(parts[1])
	key := strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(prefix))
	if abbreviation, ok := interfaceAbbreviations[key]; ok {
		return abbreviation + parts[2]
	}

	runes := []rune(prefix)
	if len(runes) > 5 {
		runes = runes[:5]
	}
	return string(runes) + parts[2]
}

func Validate(doc *Document) error {
	var problems []string
	bundleSignatures := make(map[string]string)
	bundleSources := make(map[string]map[string]bool)
	if doc.Version != 1 {
		problems = append(problems, "version must be 1")
	}
	if len(doc.Nodes) == 0 {
		problems = append(problems, "at least one node is required")
	}
	if doc.Diagram.Layout != "" && doc.Diagram.Layout != "rows" && doc.Diagram.Layout != "ring" && doc.Diagram.Layout != "sites" && doc.Diagram.Layout != "hub-spoke" && doc.Diagram.Layout != "auto" && doc.Diagram.Layout != "manual" && doc.Diagram.Layout != "elk" {
		problems = append(problems, "diagram layout must be auto, rows, ring, sites, hub-spoke, manual, or elk")
	}
	if doc.Diagram.Renderer != "" && doc.Diagram.Renderer != "native" && doc.Diagram.Renderer != "d2" && doc.Diagram.Renderer != "drawio" {
		problems = append(problems, "diagram renderer must be native, d2, or drawio")
	}
	if doc.Diagram.Theme != "" && doc.Diagram.Theme != "light" && doc.Diagram.Theme != "premium" && doc.Diagram.Theme != "nord" && doc.Diagram.Theme != "dracula" {
		problems = append(problems, "diagram theme must be light, premium, nord, or dracula")
	}
	if doc.Diagram.InterfaceAt != "" && doc.Diagram.InterfaceAt != "ends" && doc.Diagram.InterfaceAt != "none" {
		problems = append(problems, "diagram interface_labels must be ends or none")
	}
	validateStyleRules := func(kind string, rules map[string]VisualStyle) {
		for name, style := range rules {
			if strings.TrimSpace(name) == "" {
				problems = append(problems, fmt.Sprintf("diagram link_styles %s name cannot be empty", kind))
			}
			if style.Pattern != "" && style.Pattern != "solid" && style.Pattern != "dashed" && style.Pattern != "dotted" {
				problems = append(problems, fmt.Sprintf("diagram link_styles %s %q pattern must be solid, dashed, or dotted", kind, name))
			}
			if style.Width < 0 {
				problems = append(problems, fmt.Sprintf("diagram link_styles %s %q width cannot be negative", kind, name))
			}
		}
	}
	validateStyleRules("protocol", doc.Diagram.LinkStyles.Protocol)
	validateStyleRules("status", doc.Diagram.LinkStyles.Status)
	interfaceLabelStyle := doc.Diagram.InterfaceLabelStyle
	if interfaceLabelStyle.Radius != nil && *interfaceLabelStyle.Radius < 0 {
		problems = append(problems, "diagram interface_label_style radius cannot be negative")
	}
	if interfaceLabelStyle.PaddingX != nil && *interfaceLabelStyle.PaddingX < 0 {
		problems = append(problems, "diagram interface_label_style padding_x cannot be negative")
	}
	if interfaceLabelStyle.PaddingY != nil && *interfaceLabelStyle.PaddingY < 0 {
		problems = append(problems, "diagram interface_label_style padding_y cannot be negative")
	}
	if doc.Diagram.RouteClearance < 0 || doc.Diagram.RouteClearance > 200 {
		problems = append(problems, "diagram route_clearance must be between 0 and 200")
	}
	if doc.Diagram.EndpointClearance < 0 || doc.Diagram.EndpointClearance > 200 {
		problems = append(problems, "diagram endpoint_clearance must be between 0 and 200")
	}

	for id, node := range doc.Nodes {
		if strings.TrimSpace(id) == "" {
			problems = append(problems, "node ID cannot be empty")
		}
		if node.Role == "" {
			problems = append(problems, fmt.Sprintf("node %q must have a role", id))
		}
		if len([]rune(strings.TrimSpace(node.IconLabel))) > 6 {
			problems = append(problems, fmt.Sprintf("node %q icon_label must be at most 6 characters", id))
		}
		if node.Width < 0 {
			problems = append(problems, fmt.Sprintf("node %q width cannot be negative", id))
		}
		if node.Height < 0 {
			problems = append(problems, fmt.Sprintf("node %q height cannot be negative", id))
		}
	}

	nodeGroups := make(map[string]string)
	var validateGroupNodes func(*Group, string)
	validateGroupNodes = func(g *Group, groupID string) {
		if g == nil {
			return
		}
		for nodeID := range g.Nodes {
			if _, ok := doc.Nodes[nodeID]; !ok {
				problems = append(problems, fmt.Sprintf("group %q references unknown node %q", groupID, nodeID))
			} else if previous, ok := nodeGroups[nodeID]; ok {
				problems = append(problems, fmt.Sprintf("node %q belongs to multiple groups %q and %q", nodeID, previous, groupID))
			} else {
				nodeGroups[nodeID] = groupID
			}
		}
		for subID, subG := range g.Groups {
			validateGroupNodes(subG, subID)
		}
	}
	for gID, g := range doc.Groups {
		validateGroupNodes(g, gID)
	}

	for i, link := range doc.Links {
		if strings.TrimSpace(link.ID) == "" && link.ID != "" {
			problems = append(problems, fmt.Sprintf("link %d ID cannot be blank", i+1))
		}
		from := link.From
		if from.Node == "" || from.Port == "" {
			problems = append(problems, fmt.Sprintf("link %d from: endpoint must include both node and interface", i+1))
		} else if _, ok := doc.Nodes[from.Node]; !ok {
			problems = append(problems, fmt.Sprintf("link %d references unknown node %q", i+1, from.Node))
		}
		to := link.To
		if to.Node == "" || to.Port == "" {
			problems = append(problems, fmt.Sprintf("link %d to: endpoint must include both node and interface", i+1))
		} else if _, ok := doc.Nodes[to.Node]; !ok {
			problems = append(problems, fmt.Sprintf("link %d references unknown node %q", i+1, to.Node))
		}
		for endpointName, endpoint := range map[string]LinkEndpoint{"from": from, "to": to} {
			if endpoint.Side != "" && endpoint.Side != "top" && endpoint.Side != "right" && endpoint.Side != "bottom" && endpoint.Side != "left" {
				problems = append(problems, fmt.Sprintf("link %d %s side must be top, right, bottom, or left", i+1, endpointName))
			}
			if endpoint.Position != nil {
				if endpoint.Side == "" {
					problems = append(problems, fmt.Sprintf("link %d %s position requires side", i+1, endpointName))
				}
				if *endpoint.Position < 0 || *endpoint.Position > 1 {
					problems = append(problems, fmt.Sprintf("link %d %s position must be between 0 and 1", i+1, endpointName))
				}
			}
			if endpoint.Stub < 0 {
				problems = append(problems, fmt.Sprintf("link %d %s stub must be zero or greater", i+1, endpointName))
			} else if endpoint.Stub > 0 && endpoint.Side == "" {
				problems = append(problems, fmt.Sprintf("link %d %s stub requires side", i+1, endpointName))
			}
			if endpoint.LabelRotation != 0 && endpoint.LabelRotation != 90 && endpoint.LabelRotation != 180 && endpoint.LabelRotation != 270 {
				problems = append(problems, fmt.Sprintf("link %d %s label_rotation must be 0, 90, 180, or 270", i+1, endpointName))
			}
			if endpoint.LabelAlong != nil && (*endpoint.LabelAlong < 0 || *endpoint.LabelAlong > 1) {
				problems = append(problems, fmt.Sprintf("link %d %s label_along must be between 0 and 1", i+1, endpointName))
			}
			if endpoint.LabelOffset != nil && (*endpoint.LabelOffset < -200 || *endpoint.LabelOffset > 200) {
				problems = append(problems, fmt.Sprintf("link %d %s label_offset must be between -200 and 200", i+1, endpointName))
			}
			if endpoint.Address != "" {
				if _, _, err := net.ParseCIDR(endpoint.Address); err != nil {
					problems = append(problems, fmt.Sprintf("link %d %s address %q must use CIDR notation", i+1, endpointName, endpoint.Address))
				}
			}
		}
		if link.LACP && link.Bundle == "" {
			problems = append(problems, fmt.Sprintf("link %d uses LACP but has no bundle name", i+1))
		}
		if link.MultiChassis && (!link.LACP || link.Bundle == "") {
			problems = append(problems, fmt.Sprintf("link %d uses multi_chassis but is not a named LACP bundle", i+1))
		}
		if link.Bundle != "" {
			signature := strings.Join(link.Tags(), "|")
			if existing, ok := bundleSignatures[link.Bundle]; ok && existing != signature {
				problems = append(problems, fmt.Sprintf("link %d bundle %q has inconsistent LACP or trunk settings", i+1, link.Bundle))
			} else {
				bundleSignatures[link.Bundle] = signature
			}
			if from.Node != "" {
				if bundleSources[link.Bundle] == nil {
					bundleSources[link.Bundle] = make(map[string]bool)
				}
				bundleSources[link.Bundle][from.Node] = true
			}
		}
		if link.Trunk != nil {
			if link.Trunk.Encapsulation != "dot1q" {
				problems = append(problems, fmt.Sprintf("link %d trunk encapsulation must be dot1q", i+1))
			}
			for _, vlan := range link.Trunk.AllowedVLANs {
				if strings.TrimSpace(vlan) == "" {
					problems = append(problems, fmt.Sprintf("link %d contains an empty allowed VLAN", i+1))
				}
			}
		}
	}
	linkIDs := make(map[string]int)
	for i, link := range doc.Links {
		if link.ID == "" {
			continue
		}
		if previous, ok := linkIDs[link.ID]; ok {
			problems = append(problems, fmt.Sprintf("links %d and %d use duplicate ID %q", previous+1, i+1, link.ID))
		} else {
			linkIDs[link.ID] = i
		}
	}
	for i, link := range doc.Links {
		if link.MultiChassis && len(bundleSources[link.Bundle]) < 2 {
			problems = append(problems, fmt.Sprintf("link %d multi_chassis bundle %q must span at least two source nodes", i+1, link.Bundle))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

func applyDefaults(doc *Document) {
	if doc.Diagram.Layout == "" {
		doc.Diagram.Layout = "rows"
	}
	if doc.Diagram.Direction == "" {
		doc.Diagram.Direction = "down"
	}
	if doc.Diagram.Badge == "" {
		doc.Diagram.Badge = "NETWORK DIAGRAM"
	}
	if doc.Diagram.LinkStyle == "" {
		doc.Diagram.LinkStyle = "clean"
	}
	if doc.Diagram.InterfaceAt == "" {
		doc.Diagram.InterfaceAt = "ends"
	}
	for id, node := range doc.Nodes {
		if node.Label == "" {
			node.Label = id
		}
		doc.Nodes[id] = node
	}
	for i := range doc.Links {
		if doc.Links[i].Trunk != nil && doc.Links[i].Trunk.Encapsulation == "" {
			doc.Links[i].Trunk.Encapsulation = "dot1q"
		}
	}
}

func (link Link) Tags() []string {
	var tags []string
	if link.MultiChassis {
		tags = append(tags, "MC-LAG")
	}
	if link.LACP {
		tags = append(tags, "LACP")
	}
	if link.Bundle != "" {
		tags = append(tags, link.Bundle)
	}
	if link.Trunk != nil {
		tags = append(tags, "TRUNK", strings.ToUpper(link.Trunk.Encapsulation))
		if len(link.Trunk.AllowedVLANs) > 0 {
			tags = append(tags, "VLAN "+strings.Join(link.Trunk.AllowedVLANs, ","))
		}
	}
	return tags
}
