package d2backend

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/model"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
	"oss.terrastruct.com/util-go/go2"
)

type Options struct {
	Layout string
}

func Render(diagram *model.Diagram, opts Options) ([]byte, error) {
	if opts.Layout == "" {
		opts.Layout = "elk"
	}
	source := Source(diagram)
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("create D2 text ruler: %w", err)
	}
	layout := opts.Layout
	compileOpts := &d2lib.CompileOptions{
		Layout:         &layout,
		LayoutResolver: layoutResolver,
		Ruler:          ruler,
	}
	salt := "netdiag-d2-v1"
	renderOpts := &d2svg.RenderOpts{
		Pad:         go2.Pointer(int64(60)),
		NoXMLTag:    go2.Pointer(true),
		OmitVersion: go2.Pointer(true),
		Salt:        &salt,
	}
	ctx := log.WithDefault(context.Background())
	target, _, err := d2lib.Compile(ctx, source, compileOpts, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("compile D2 using %s: %w", opts.Layout, err)
	}
	out, err := d2svg.Render(target, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("render D2 SVG: %w", err)
	}
	return out, nil
}

func layoutResolver(engine string) (d2graph.LayoutGraph, error) {
	switch engine {
	case "elk":
		return d2elklayout.DefaultLayout, nil
	case "dagre":
		return d2dagrelayout.DefaultLayout, nil
	default:
		return nil, fmt.Errorf("unsupported D2 layout %q; use elk or dagre", engine)
	}
}

func Source(diagram *model.Diagram) string {
	var out bytes.Buffer
	direction := diagram.Theme.Direction
	if direction == "" || direction == "down" {
		direction = "down"
	}
	fmt.Fprintf(&out, "direction: %s\n", direction)

	groupByID := make(map[string]model.Group)
	children := make(map[string][]model.Group)
	nodePath := make(map[string]string)
	for _, group := range diagram.Groups {
		groupByID[group.ID] = group
		children[group.ParentID] = append(children[group.ParentID], group)
	}
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool { return children[parent][i].ID < children[parent][j].ID })
	}
	nodeByID := make(map[string]model.Node)
	for _, node := range diagram.Nodes {
		nodeByID[node.ID] = node
	}

	var writeGroup func(model.Group, string, int)
	writeGroup = func(group model.Group, parentPath string, depth int) {
		path := joinPath(parentPath, group.ID)
		indent := strings.Repeat("  ", depth)
		fmt.Fprintf(&out, "%s%s: %s {\n", indent, group.ID, quote(defaultString(group.Label, group.ID)))
		fmt.Fprintf(&out, "%s  style.fill: \"#eff6ff\"\n", indent)
		fmt.Fprintf(&out, "%s  style.stroke: \"#93c5fd\"\n", indent)
		for _, nodeID := range group.NodeIDs {
			writeNode(&out, nodeByID[nodeID], depth+1)
			nodePath[nodeID] = joinPath(path, nodeID)
		}
		for _, child := range children[group.ID] {
			writeGroup(child, path, depth+1)
		}
		fmt.Fprintf(&out, "%s}\n", indent)
	}
	for _, group := range children[""] {
		writeGroup(group, "", 0)
	}

	for _, node := range diagram.Nodes {
		if _, grouped := nodePath[node.ID]; grouped {
			continue
		}
		writeNode(&out, node, 0)
		nodePath[node.ID] = node.ID
	}

	for _, link := range diagram.Links {
		from := nodePath[link.From.Node]
		to := nodePath[link.To.Node]
		fmt.Fprintf(&out, "%s -> %s: %s {\n", from, to, quote(link.MiddleLabel()))
		fmt.Fprintf(&out, "  source-arrowhead.label: %s\n", quote(endpointLabel(link.SourceLabel(), link.From.Address)))
		fmt.Fprintf(&out, "  target-arrowhead.label: %s\n", quote(endpointLabel(link.TargetLabel(), link.To.Address)))
		fmt.Fprintf(&out, "  source-arrowhead.shape: none\n")
		fmt.Fprintf(&out, "  target-arrowhead.shape: none\n")
		fmt.Fprintf(&out, "  style.stroke: %s\n", quote(linkColor(link.Style)))
		if dash := linkDash(link.Style); dash > 0 {
			fmt.Fprintf(&out, "  style.stroke-dash: %d\n", dash)
		}
		fmt.Fprintf(&out, "}\n")
	}
	return out.String()
}

func endpointLabel(label, address string) string {
	if address == "" {
		return label
	}
	return label + " · " + address
}

func writeNode(out *bytes.Buffer, node model.Node, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(out, "%s%s: %s {\n", indent, node.ID, quote(defaultString(node.Label, node.ID)))
	fmt.Fprintf(out, "%s  shape: %s\n", indent, nodeShape(node.Role))
	fmt.Fprintf(out, "%s  style.fill: %s\n", indent, quote("#ffffff"))
	fmt.Fprintf(out, "%s  style.stroke: %s\n", indent, quote(defaultString(node.Color, roleColor(node.Role))))
	fmt.Fprintf(out, "%s  style.stroke-width: 2\n", indent)
	fmt.Fprintf(out, "%s}\n", indent)
}

func quote(value string) string {
	return strconv.Quote(value)
}

func joinPath(parent, id string) string {
	if parent == "" {
		return id
	}
	return parent + "." + id
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func nodeShape(role string) string {
	switch role {
	case "wan-cloud", "public-cloud", "internet":
		return "cloud"
	case "router", "edge-router", "ospf-backbone", "isis-level-1", "isis-level-2":
		return "oval"
	default:
		return "rectangle"
	}
}

func roleColor(role string) string {
	switch role {
	case "firewall":
		return "#dc2626"
	case "wan-cloud", "internet":
		return "#64748b"
	case "public-cloud":
		return "#f59e0b"
	case "server":
		return "#16a34a"
	default:
		return "#0878b9"
	}
}

func linkColor(style string) string {
	switch style {
	case "security":
		return "#dc2626"
	case "wan", "isis":
		return "#7c3aed"
	case "ospf":
		return "#2563eb"
	case "ibgp":
		return "#0891b2"
	case "ebgp":
		return "#ea580c"
	default:
		return "#334155"
	}
}

func linkDash(style string) int {
	switch style {
	case "ibgp", "ebgp", "internet", "dwdm":
		return 5
	default:
		return 0
	}
}
