package svg

import (
	"math"
	"sort"

	"github.com/gwoodwa1/netdiag/internal/model"
)

const (
	siteGap        = 90.0
	sitePadding    = 54.0
	siteHeader     = 78.0
	siteRoleHeight = 205.0
	siteNodeGap    = 42.0
	siteLinkGap    = 220.0
)

type placedGroup struct {
	ID       string
	Label    string
	Kind     string
	ParentID string
	Box      box
	Depth    int
}

type layoutResult struct {
	Nodes  map[string]placedNode
	Groups []placedGroup
	Width  float64
	Height float64
}

func layoutDiagram(doc *model.Diagram, roles []string, byRole map[string][]string) layoutResult {
	if doc.Theme.Layout == "sites" {
		return placeSiteLayout(doc)
	}
	height := headerHeight + float64(len(roles))*rowHeight + 100
	if doc.Theme.Layout == "ring" {
		height = 1250
	}
	return layoutResult{Nodes: placeNodes(doc, roles, byRole), Width: canvasWidth, Height: height}
}

func placeSiteLayout(doc *model.Diagram) layoutResult {
	nodesByID := make(map[string]model.Node)
	for _, node := range doc.Nodes {
		nodesByID[node.ID] = node
	}
	groupsByID := make(map[string]model.Group)
	children := make(map[string][]string)
	for _, group := range doc.Groups {
		groupsByID[group.ID] = group
		children[group.ParentID] = append(children[group.ParentID], group.ID)
	}
	for parent := range children {
		sort.Strings(children[parent])
	}

	var collectNodes func(string) []string
	collectNodes = func(groupID string) []string {
		group := groupsByID[groupID]
		result := append([]string{}, group.NodeIDs...)
		for _, childID := range children[groupID] {
			result = append(result, collectNodes(childID)...)
		}
		sort.Strings(result)
		return result
	}

	rootIDs := append([]string{}, children[""]...)
	rootIDs = orderSiteRoots(rootIDs, groupsByID)
	grouped := make(map[string]bool)
	for _, rootID := range rootIDs {
		for _, nodeID := range collectNodes(rootID) {
			grouped[nodeID] = true
		}
	}
	var ungrouped []string
	for _, node := range doc.Nodes {
		if !grouped[node.ID] {
			ungrouped = append(ungrouped, node.ID)
		}
	}
	sort.Strings(ungrouped)
	if len(rootIDs) == 0 || len(ungrouped) > 0 {
		rootIDs = append(rootIDs, "__shared__")
	}

	type sitePlan struct {
		ID      string
		Label   string
		Kind    string
		NodeIDs []string
		Roles   []string
		ByRole  map[string][]string
		Width   float64
		Height  float64
	}
	var plans []sitePlan
	for _, rootID := range rootIDs {
		label, kind, nodeIDs := "Shared / Ungrouped", "shared", ungrouped
		if rootID != "__shared__" {
			group := groupsByID[rootID]
			label, kind, nodeIDs = group.Label, group.Kind, collectNodes(rootID)
			if label == "" {
				label = rootID
			}
		}
		if len(nodeIDs) == 0 {
			continue
		}
		byRole := make(map[string][]string)
		for _, nodeID := range nodeIDs {
			byRole[nodesByID[nodeID].Role] = append(byRole[nodesByID[nodeID].Role], nodeID)
		}
		roles := orderedRoles(byRole, nodesByID)
		maxRowWidth := 0.0
		for _, role := range roles {
			gap := siteRowGap(len(byRole[role]))
			rowWidth := gap
			for _, nodeID := range byRole[role] {
				rowWidth += nodeWidth(nodesByID[nodeID].Role) + gap
			}
			maxRowWidth = math.Max(maxRowWidth, rowWidth)
		}
		plans = append(plans, sitePlan{
			ID: rootID, Label: label, Kind: kind, NodeIDs: nodeIDs, Roles: roles, ByRole: byRole,
			Width: math.Max(520, maxRowWidth+sitePadding*2), Height: siteHeader + float64(len(roles))*siteRoleHeight + sitePadding,
		})
	}

	result := layoutResult{Nodes: make(map[string]placedNode)}
	x := 70.0
	siteY := headerHeight + 55
	for _, plan := range plans {
		siteBox := box{X: x, Y: siteY, W: plan.Width, H: plan.Height}
		result.Groups = append(result.Groups, placedGroup{ID: plan.ID, Label: plan.Label, Kind: plan.Kind, Box: siteBox})
		for row, role := range plan.Roles {
			ids := plan.ByRole[role]
			rowWidth := 0.0
			for _, nodeID := range ids {
				rowWidth += nodeWidth(nodesByID[nodeID].Role)
			}
			gap := siteRowGap(len(ids))
			rowWidth += float64(len(ids)-1) * gap
			nodeX := siteBox.X + (siteBox.W-rowWidth)/2
			nodeY := siteBox.Y + siteHeader + float64(row)*siteRoleHeight + 42
			for _, nodeID := range ids {
				node := nodesByID[nodeID]
				width := nodeWidth(node.Role)
				result.Nodes[nodeID] = placedNode{ID: nodeID, Node: node, Box: box{X: nodeX, Y: nodeY, W: width, H: nodeHeight}}
				nodeX += width + gap
			}
		}
		x += plan.Width + siteGap
		result.Height = math.Max(result.Height, siteBox.Y+siteBox.H+80)
	}
	result.Width = math.Max(canvasWidth, x-siteGap+70)

	for _, rootID := range children[""] {
		appendNestedGroupBoxes(&result, rootID, 1, groupsByID, children)
	}
	sort.SliceStable(result.Groups, func(i, j int) bool { return result.Groups[i].Depth < result.Groups[j].Depth })
	return result
}

func siteRowGap(nodeCount int) float64 {
	if nodeCount > 1 {
		return siteLinkGap
	}
	return siteNodeGap
}

func appendNestedGroupBoxes(result *layoutResult, groupID string, depth int, groups map[string]model.Group, children map[string][]string) {
	for _, childID := range children[groupID] {
		group := groups[childID]
		var bounds box
		first := true
		var include func(string)
		include = func(id string) {
			for _, nodeID := range groups[id].NodeIDs {
				node, ok := result.Nodes[nodeID]
				if !ok {
					continue
				}
				if first {
					bounds = node.Box
					first = false
				} else {
					bounds = unionBox(bounds, node.Box)
				}
			}
			for _, nestedID := range children[id] {
				include(nestedID)
			}
		}
		include(childID)
		if !first {
			padding := 24.0
			bounds = box{X: bounds.X - padding, Y: bounds.Y - 32, W: bounds.W + padding*2, H: bounds.H + 32 + padding}
			label := group.Label
			if label == "" {
				label = childID
			}
			result.Groups = append(result.Groups, placedGroup{ID: childID, Label: label, Kind: group.Kind, ParentID: groupID, Box: bounds, Depth: depth})
		}
		appendNestedGroupBoxes(result, childID, depth+1, groups, children)
	}
}

func orderedRoles(byRole map[string][]string, nodesByID map[string]model.Node) []string {
	temp := &model.Diagram{}
	for _, ids := range byRole {
		for _, id := range ids {
			temp.Nodes = append(temp.Nodes, nodesByID[id])
		}
	}
	roles, ordered := groupNodes(temp)
	for role := range byRole {
		byRole[role] = ordered[role]
	}
	return roles
}

func orderSiteRoots(ids []string, groups map[string]model.Group) []string {
	var sites, cores []string
	for _, id := range ids {
		kind := groups[id].Kind
		if kind == "core" || kind == "wan" || kind == "cloud" {
			cores = append(cores, id)
		} else {
			sites = append(sites, id)
		}
	}
	sort.Strings(sites)
	sort.Strings(cores)
	middle := (len(sites) + 1) / 2
	result := append([]string{}, sites[:middle]...)
	result = append(result, cores...)
	result = append(result, sites[middle:]...)
	return result
}

func unionBox(a, b box) box {
	left := math.Min(a.X, b.X)
	top := math.Min(a.Y, b.Y)
	right := math.Max(a.X+a.W, b.X+b.W)
	bottom := math.Max(a.Y+a.H, b.Y+b.H)
	return box{X: left, Y: top, W: right - left, H: bottom - top}
}

func nodeRootGroups(doc *model.Diagram) map[string]string {
	groups := make(map[string]model.Group)
	for _, group := range doc.Groups {
		groups[group.ID] = group
	}
	result := make(map[string]string)
	for _, group := range doc.Groups {
		root := group.ID
		for groups[root].ParentID != "" {
			root = groups[root].ParentID
		}
		for _, nodeID := range group.NodeIDs {
			result[nodeID] = root
		}
	}
	return result
}
