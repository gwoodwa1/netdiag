package model

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/constraint"
	"github.com/gwoodwa1/netdiag/internal/spec"
)

// Diagram is the renderer-neutral intermediate representation compiled from an
// expanded and validated source document. Rendering backends must consume this
// type rather than authored YAML or spec.Document.
type Diagram struct {
	Nodes  []Node
	Groups []Group
	Links  []Link
	Theme  Theme
	Constraints constraint.Set
}

type Node struct {
	ID        string
	Label     string
	Role      string
	Icon      string
	IconLabel string
	Color     string
	Width     float64
	Height    float64
	Order     int
	Metadata  map[string]interface{}
}

type Group struct {
	ID       string
	Label    string
	Kind     string
	ParentID string
	NodeIDs  []string
}

type Link struct {
	ID           string
	From         LinkEndpoint
	To           LinkEndpoint
	Label        string
	Style        string
	Protocol     string
	Status       string
	Bundle       string
	LACP         bool
	MultiChassis bool
	Trunk        *Trunk
	Labels       LinkLabels
}

func (link Link) StableID() string {
	if link.ID != "" {
		return link.ID
	}
	left := link.From.Node + ":" + link.From.Port
	right := link.To.Node + ":" + link.To.Port
	if right < left {
		left, right = right, left
	}
	sum := sha256.Sum256([]byte(left + "--" + right))
	return fmt.Sprintf("auto-%x", sum[:8])
}

type LinkEndpoint struct {
	Node          string
	Port          string
	Side          string
	Position      *float64
	Stub          float64
	LabelRotation int
	LabelAlong    *float64
	LabelOffset   *float64
	Label         string
	Address       string
}

type LinkLabels struct {
	Source string
	Middle string
	Target string
}

type Trunk struct {
	Encapsulation string
	AllowedVLANs  []string
}

type Theme struct {
	Name                string
	Title               string
	Subtitle            string
	Badge               string
	Layout              string
	Direction           string
	LinkStyle           string
	RouteClearance      float64
	EndpointClearance   float64
	InterfaceLabels     string
	Renderer            string
	LinkStyles          LinkStyleRules
	InterfaceLabelStyle InterfaceLabelStyle
}

type InterfaceLabelStyle struct {
	Fill     string
	Color    string
	Border   string
	Radius   float64
	PaddingX float64
	PaddingY float64
}

type LinkStyleRules struct {
	Protocol map[string]VisualStyle
	Status   map[string]VisualStyle
}

type VisualStyle struct {
	Color   string
	Pattern string
	Width   float64
}

func (d *Diagram) ResolveLinkStyle(link Link) VisualStyle {
	var result VisualStyle
	merge := func(style VisualStyle) {
		if style.Color != "" {
			result.Color = style.Color
		}
		if style.Pattern != "" {
			result.Pattern = style.Pattern
		}
		if style.Width > 0 {
			result.Width = style.Width
		}
	}
	merge(d.Theme.LinkStyles.Protocol[strings.ToLower(link.Protocol)])
	merge(d.Theme.LinkStyles.Status[strings.ToLower(link.Status)])
	return result
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

func (link Link) SourceLabel() string {
	if link.Labels.Source != "" {
		return link.Labels.Source
	}
	return spec.DisplayPort(link.From.Port)
}

func (link Link) MiddleLabel() string {
	if link.Labels.Middle != "" {
		return link.Labels.Middle
	}
	return link.Label
}

func (link Link) TargetLabel() string {
	if link.Labels.Target != "" {
		return link.Labels.Target
	}
	return spec.DisplayPort(link.To.Port)
}

func Compile(doc *spec.Document) (*Diagram, error) {
	// 1. Validate the spec.Document
	if err := spec.Validate(doc); err != nil {
		return nil, err
	}

	// 2. Resolve groups and build flat slice of model.Group
	var groups []Group
	nodeToGroup := make(map[string]string)

	// Sort group IDs for deterministic order
	var groupIDs []string
	for gID := range doc.Groups {
		groupIDs = append(groupIDs, gID)
	}
	sort.Strings(groupIDs)

	for _, gID := range groupIDs {
		resolveGroup(gID, doc.Groups[gID], "", &groups, nodeToGroup)
	}

	// 3. Construct model.Nodes
	var nodes []Node
	for id, nodeSpec := range doc.Nodes {
		label := nodeSpec.Label
		if label == "" {
			label = id
		}
		nodes = append(nodes, Node{
			ID:        id,
			Label:     label,
			Role:      nodeSpec.Role,
			Icon:      nodeSpec.Icon,
			IconLabel: nodeSpec.IconLabel,
			Color:     nodeSpec.Color,
			Width:     nodeSpec.Width,
			Height:    nodeSpec.Height,
			Order:     nodeSpec.Order,
			Metadata:  nodeSpec.Metadata,
		})
	}
	// Sort nodes by ID for determinism
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// 4. Construct model.Links
	var links []Link
	for _, linkSpec := range doc.Links {
		var trunk *Trunk
		if linkSpec.Trunk != nil {
			trunk = &Trunk{
				Encapsulation: linkSpec.Trunk.Encapsulation,
				AllowedVLANs:  linkSpec.Trunk.AllowedVLANs,
			}
		}
		labels := LinkLabels{
			Source: linkSpec.From.Label,
			Middle: linkSpec.Label,
			Target: linkSpec.To.Label,
		}
		if linkSpec.Labels != nil {
			if linkSpec.Labels.Source != "" {
				labels.Source = linkSpec.Labels.Source
			}
			if linkSpec.Labels.Middle != "" {
				labels.Middle = linkSpec.Labels.Middle
			}
			if linkSpec.Labels.Target != "" {
				labels.Target = linkSpec.Labels.Target
			}
		}
		if labels.Source == "" {
			labels.Source = spec.DisplayPort(linkSpec.From.Port)
		}
		if labels.Target == "" {
			labels.Target = spec.DisplayPort(linkSpec.To.Port)
		}
		links = append(links, Link{
			ID: linkSpec.ID,
			From: LinkEndpoint{
				Node:          linkSpec.From.Node,
				Port:          linkSpec.From.Port,
				Side:          linkSpec.From.Side,
				Position:      linkSpec.From.Position,
				Stub:          linkSpec.From.Stub,
				LabelRotation: linkSpec.From.LabelRotation,
				LabelAlong:    linkSpec.From.LabelAlong,
				LabelOffset:   linkSpec.From.LabelOffset,
				Label:         linkSpec.From.Label,
				Address:       linkSpec.From.Address,
			},
			To: LinkEndpoint{
				Node:          linkSpec.To.Node,
				Port:          linkSpec.To.Port,
				Side:          linkSpec.To.Side,
				Position:      linkSpec.To.Position,
				Stub:          linkSpec.To.Stub,
				LabelRotation: linkSpec.To.LabelRotation,
				LabelAlong:    linkSpec.To.LabelAlong,
				LabelOffset:   linkSpec.To.LabelOffset,
				Label:         linkSpec.To.Label,
				Address:       linkSpec.To.Address,
			},
			Label:        linkSpec.Label,
			Style:        linkSpec.Style,
			Protocol:     linkSpec.Protocol,
			Status:       linkSpec.Status,
			Bundle:       linkSpec.Bundle,
			LACP:         linkSpec.LACP,
			MultiChassis: linkSpec.MultiChassis,
			Trunk:        trunk,
			Labels:       labels,
		})
	}

	// 5. Construct Theme
	theme := Theme{
		Name:              doc.Diagram.Theme,
		Title:             doc.Diagram.Title,
		Subtitle:          doc.Diagram.Subtitle,
		Badge:             doc.Diagram.Badge,
		Layout:            doc.Diagram.Layout,
		Direction:         doc.Diagram.Direction,
		LinkStyle:         doc.Diagram.LinkStyle,
		RouteClearance:    doc.Diagram.RouteClearance,
		EndpointClearance: doc.Diagram.EndpointClearance,
		InterfaceLabels:   doc.Diagram.InterfaceAt,
		Renderer:          doc.Diagram.Renderer,
		LinkStyles:        compileLinkStyleRules(doc.Diagram.LinkStyles),
		InterfaceLabelStyle: InterfaceLabelStyle{
			Fill:     doc.Diagram.InterfaceLabelStyle.Fill,
			Color:    doc.Diagram.InterfaceLabelStyle.Color,
			Border:   doc.Diagram.InterfaceLabelStyle.Border,
			Radius:   floatValue(doc.Diagram.InterfaceLabelStyle.Radius, 5),
			PaddingX: floatValue(doc.Diagram.InterfaceLabelStyle.PaddingX, 9),
			PaddingY: floatValue(doc.Diagram.InterfaceLabelStyle.PaddingY, 5),
		},
	}

	diagram := &Diagram{
		Nodes:  nodes,
		Groups: groups,
		Links:  links,
		Theme:  theme,
	}
	diagram.Constraints = compileConstraints(diagram)
	return diagram, nil
}

var preferredRoleRanks = []string{
	"users", "internet", "public-cloud", "wan-cloud", "dwdm",
	"ospf-backbone", "ospf-area-10", "ospf-area-20",
	"isis-level-2", "isis-level-1", "route-reflector", "rr-client", "external-peer",
	"edge-router", "router", "core-router", "firewall", "core-switch",
	"distribution-switch", "access-switch", "wireless", "metro-switch",
	"super-spine", "spine", "leaf", "server", "endpoint",
}

func compileConstraints(diagram *Diagram) constraint.Set {
	var result constraint.Set
	byRole := make(map[string][]string)
	orders := make(map[string][]constraint.OrderedItem)
	for _, node := range diagram.Nodes {
		byRole[node.Role] = append(byRole[node.Role], node.ID)
		if node.Order != 0 {
			orders[node.Role] = append(orders[node.Role], constraint.OrderedItem{NodeID: node.ID, Position: node.Order})
		}
	}
	roles := orderedConstraintRoles(byRole)
	for index, role := range roles {
		result.Ranks = append(result.Ranks, constraint.Rank{ID: role, Order: index, Nodes: byRole[role], Strength: constraint.Strong})
		if len(orders[role]) > 0 {
			sort.Slice(orders[role], func(i, j int) bool {
				if orders[role][i].Position == orders[role][j].Position {
					return orders[role][i].NodeID < orders[role][j].NodeID
				}
				return orders[role][i].Position < orders[role][j].Position
			})
			result.Orders = append(result.Orders, constraint.Order{Scope: role, Axis: constraint.Horizontal, Items: orders[role], Strength: constraint.Strong})
		}
	}
	for _, group := range diagram.Groups {
		if group.ParentID != "" {
			result.Containment = append(result.Containment, constraint.Containment{ChildID: group.ID, ParentID: group.ParentID, IsGroup: true, Strength: constraint.Required})
		}
		for _, nodeID := range group.NodeIDs {
			result.Containment = append(result.Containment, constraint.Containment{ChildID: nodeID, ParentID: group.ID, Strength: constraint.Required})
		}
	}
	for _, link := range diagram.Links {
		id := link.StableID()
		result.Ports = append(result.Ports,
			constraint.Port{LinkID: id, Endpoint: constraint.Source, NodeID: link.From.Node, PortID: link.From.Port, Side: link.From.Side, Position: link.From.Position, Strength: constraint.Required},
			constraint.Port{LinkID: id, Endpoint: constraint.Target, NodeID: link.To.Node, PortID: link.To.Port, Side: link.To.Side, Position: link.To.Position, Strength: constraint.Required},
		)
	}
	return result
}

func orderedConstraintRoles(byRole map[string][]string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, role := range preferredRoleRanks {
		if len(byRole[role]) > 0 {
			result = append(result, role)
			seen[role] = true
		}
	}
	var rest []string
	for role := range byRole {
		if !seen[role] {
			rest = append(rest, role)
		}
	}
	sort.Strings(rest)
	return append(result, rest...)
}

func floatValue(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func compileLinkStyleRules(rules spec.LinkStyleRules) LinkStyleRules {
	compile := func(input map[string]spec.VisualStyle) map[string]VisualStyle {
		output := make(map[string]VisualStyle, len(input))
		for name, style := range input {
			output[strings.ToLower(name)] = VisualStyle{Color: style.Color, Pattern: style.Pattern, Width: style.Width}
		}
		return output
	}
	return LinkStyleRules{Protocol: compile(rules.Protocol), Status: compile(rules.Status)}
}

func resolveGroup(id string, g *spec.Group, parentID string, groups *[]Group, nodeToGroup map[string]string) {
	if g == nil {
		return
	}
	var nodeIDs []string
	for nodeID := range g.Nodes {
		nodeIDs = append(nodeIDs, nodeID)
		nodeToGroup[nodeID] = id
	}
	sort.Strings(nodeIDs)

	*groups = append(*groups, Group{
		ID:       id,
		Label:    g.Label,
		Kind:     g.Kind,
		ParentID: parentID,
		NodeIDs:  nodeIDs,
	})

	var subGroupIDs []string
	for subID := range g.Groups {
		subGroupIDs = append(subGroupIDs, subID)
	}
	sort.Strings(subGroupIDs)

	for _, subID := range subGroupIDs {
		resolveGroup(subID, g.Groups[subID], id, groups, nodeToGroup)
	}
}
