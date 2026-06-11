package spec

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Version int               `yaml:"version"`
	Diagram Diagram           `yaml:"diagram"`
	Groups  map[string]*Group `yaml:"groups"`
	Nodes   map[string]Node   `yaml:"nodes"`
	Links   []Link            `yaml:"links"`
}

type Diagram struct {
	Title       string `yaml:"title"`
	Subtitle    string `yaml:"subtitle"`
	Badge       string `yaml:"badge"`
	Layout      string `yaml:"layout"`
	Direction   string `yaml:"direction"`
	LinkStyle   string `yaml:"link_style"`
	InterfaceAt string `yaml:"interface_labels"`
	Theme       string `yaml:"theme"`
	Renderer    string `yaml:"renderer,omitempty"`
}

type Group struct {
	Label  string                 `yaml:"label"`
	Kind   string                 `yaml:"kind"`
	Groups map[string]*Group      `yaml:"groups"`
	Nodes  map[string]interface{} `yaml:"nodes"`
}

type Node struct {
	Label    string                 `yaml:"label"`
	Role     string                 `yaml:"role"`
	Icon     string                 `yaml:"icon"`
	Color    string                 `yaml:"color"`
	Order    int                    `yaml:"order"`
	Metadata map[string]interface{} `yaml:"metadata"`
}

type LinkEndpoint struct {
	Node    string `yaml:"node"`
	Port    string `yaml:"port"`
	Side    string `yaml:"side,omitempty"`
	Label   string `yaml:"label,omitempty"`
	Address string `yaml:"address,omitempty"`
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
		Node    string `yaml:"node"`
		Port    string `yaml:"port"`
		Side    string `yaml:"side"`
		Label   string `yaml:"label"`
		Address string `yaml:"address"`
	}
	var raw rawLinkEndpoint
	if err := value.Decode(&raw); err != nil {
		return err
	}
	le.Node = raw.Node
	le.Port = raw.Port
	le.Side = raw.Side
	le.Label = raw.Label
	le.Address = raw.Address
	return nil
}

type Link struct {
	From         LinkEndpoint `yaml:"from"`
	To           LinkEndpoint `yaml:"to"`
	Label        string       `yaml:"label"`
	Style        string       `yaml:"style"`
	Bundle       string       `yaml:"bundle"`
	LACP         bool         `yaml:"lacp"`
	MultiChassis bool         `yaml:"multi_chassis"`
	Trunk        *Trunk       `yaml:"trunk"`
	Labels       *LinkLabels  `yaml:"labels,omitempty"`
}

type LinkLabels struct {
	Source string `yaml:"source,omitempty"`
	Middle string `yaml:"middle,omitempty"`
	Target string `yaml:"target,omitempty"`
}

type Trunk struct {
	Encapsulation string   `yaml:"encapsulation"`
	AllowedVLANs  []string `yaml:"allowed_vlans"`
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
	"tengigabitethernet":     "Te",
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
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	applyDefaults(&doc)
	if err := Validate(&doc); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &doc, nil
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
	if doc.Diagram.Layout != "" && doc.Diagram.Layout != "rows" && doc.Diagram.Layout != "ring" && doc.Diagram.Layout != "sites" && doc.Diagram.Layout != "auto" && doc.Diagram.Layout != "manual" && doc.Diagram.Layout != "elk" {
		problems = append(problems, "diagram layout must be auto, rows, ring, sites, manual, or elk")
	}
	if doc.Diagram.Renderer != "" && doc.Diagram.Renderer != "native" && doc.Diagram.Renderer != "d2" {
		problems = append(problems, "diagram renderer must be native or d2")
	}

	for id, node := range doc.Nodes {
		if strings.TrimSpace(id) == "" {
			problems = append(problems, "node ID cannot be empty")
		}
		if node.Role == "" {
			problems = append(problems, fmt.Sprintf("node %q must have a role", id))
		}
	}

	var validateGroupNodes func(*Group, string)
	validateGroupNodes = func(g *Group, groupID string) {
		if g == nil {
			return
		}
		for nodeID := range g.Nodes {
			if _, ok := doc.Nodes[nodeID]; !ok {
				problems = append(problems, fmt.Sprintf("group %q references unknown node %q", groupID, nodeID))
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
