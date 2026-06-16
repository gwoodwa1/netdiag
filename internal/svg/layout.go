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
	hubSpokePEGap  = 500.0
	siteCanvasMax  = 5600.0
)

const (
	hubSpokeHubNodeWidth    = 760.0
	hubSpokeHubNodeHeight   = 220.0
	hubSpokeSpokeNodeWidth  = 420.0
	hubSpokeSpokeNodeHeight = 140.0
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
	if doc.Theme.Layout == "hub-spoke" {
		return placeHubSpokeLayout(doc)
	}
	if doc.Theme.Layout == "sites" {
		return placeSiteLayout(doc)
	}
	height := headerHeight + float64(len(roles))*rowHeight + 100
	if doc.Theme.Layout == "ring" {
		return layoutResult{Nodes: placeRingNodes(doc, roles, byRole), Width: canvasWidth, Height: 1250}
	}
	width := rowLayoutWidth(doc, roles, byRole)
	return layoutResult{Nodes: placeRowNodes(doc, roles, byRole, width), Width: width, Height: height}
}

func placeHubSpokeLayout(doc *model.Diagram) layoutResult {
	const (
		width       = 6400.0
		height      = 3000.0
		spokeWidth  = 1400.0
		spokeHeight = 340.0
		coreWidth   = 2700.0
		coreHeight  = 560.0
		sideMargin  = 70.0
		top         = headerHeight + 55
		rowGap      = 90.0
	)
	nodes := make(map[string]model.Node)
	for _, node := range doc.Nodes {
		nodes[node.ID] = node
	}
	degrees := nodeDegrees(doc)
	var cores, spokes []model.Group
	for _, group := range doc.Groups {
		isCore := false
		for _, nodeID := range group.NodeIDs {
			if nodes[nodeID].Role == "core-router" {
				isCore = true
			}
		}
		if isCore {
			cores = append(cores, group)
		} else {
			spokes = append(spokes, group)
		}
	}
	sort.Slice(cores, func(i, j int) bool { return cores[i].ID < cores[j].ID })
	sort.Slice(spokes, func(i, j int) bool { return spokes[i].ID < spokes[j].ID })
	if len(cores) == 0 || len(spokes) == 0 {
		return placeSiteLayout(doc)
	}

	result := layoutResult{Nodes: make(map[string]placedNode), Width: width, Height: height}
	coreX := (width - coreWidth) / 2
	coreTotalHeight := float64(len(cores))*coreHeight + float64(len(cores)-1)*rowGap
	coreY := (height + headerHeight - coreTotalHeight) / 2
	for index, group := range cores {
		groupBox := box{X: coreX, Y: coreY + float64(index)*(coreHeight+rowGap), W: coreWidth, H: coreHeight}
		result.Groups = append(result.Groups, placedGroup{ID: group.ID, Label: group.Label, Kind: group.Kind, Box: groupBox})
		placeHubGroupNodes(&result, group, groupBox, nodes, degrees, true)
	}
	split := (len(spokes) + 1) / 2
	for index, group := range spokes {
		rowGroups := split
		rowIndex := index
		y := top
		if index >= split {
			rowGroups = len(spokes) - split
			rowIndex -= split
			y = height - spokeHeight - 80
		}
		available := width - sideMargin*2
		step := available / float64(rowGroups)
		x := sideMargin + step*(float64(rowIndex)+0.5) - spokeWidth/2
		groupBox := box{X: x, Y: y, W: spokeWidth, H: spokeHeight}
		result.Groups = append(result.Groups, placedGroup{ID: group.ID, Label: group.Label, Kind: group.Kind, Box: groupBox})
		placeHubGroupNodes(&result, group, groupBox, nodes, degrees, false)
	}
	return result
}

func placeHubGroupNodes(result *layoutResult, group model.Group, groupBox box, nodes map[string]model.Node, degrees map[string]int, coreGroup bool) {
	ids := append([]string(nil), group.NodeIDs...)
	sort.Strings(ids)
	totalWidth := 0.0
	for _, id := range ids {
		totalWidth += hubSpokeNodeWidth(nodes[id], degrees[id], coreGroup)
	}
	gap := hubGroupGap(ids, nodes)
	totalWidth += float64(len(ids)-1) * gap
	x := groupBox.X + (groupBox.W-totalWidth)/2
	for _, id := range ids {
		node := nodes[id]
		width := hubSpokeNodeWidth(node, degrees[id], coreGroup)
		height := hubSpokeNodeHeight(node, degrees[id], coreGroup)
		y := groupBox.Y + siteHeader + (groupBox.H-siteHeader-height)/2
		result.Nodes[id] = placedNode{ID: id, Node: node, Box: box{X: x, Y: y, W: width, H: height}}
		x += width + gap
	}
}

func hubSpokeNodeWidth(node model.Node, degree int, coreGroup bool) float64 {
	if node.Width > 0 {
		return node.Width
	}
	width := hubSpokeSpokeNodeWidth
	if coreGroup || node.Role == "core-router" {
		width = hubSpokeHubNodeWidth
	}
	if degree <= 4 {
		return width
	}
	return width + float64(degree-4)*70
}

func hubSpokeNodeHeight(node model.Node, degree int, coreGroup bool) float64 {
	if node.Height > 0 {
		return node.Height
	}
	height := hubSpokeSpokeNodeHeight
	if coreGroup || node.Role == "core-router" {
		height = hubSpokeHubNodeHeight
	}
	if degree <= 4 {
		return height
	}
	return height + float64(degree-4)*22
}

func hubGroupGap(ids []string, nodes map[string]model.Node) float64 {
	gap := siteRowGap(len(ids))
	if len(ids) < 2 {
		return gap
	}
	for _, id := range ids {
		if nodes[id].Role != "edge-router" {
			return gap
		}
	}
	return hubSpokePEGap
}

func placeSiteLayout(doc *model.Diagram) layoutResult {
	degrees := nodeDegrees(doc)
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
		ID         string
		Label      string
		Kind       string
		NodeIDs    []string
		Roles      []string
		ByRole     map[string][]string
		RowHeights []float64
		Width      float64
		Height     float64
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
		rowHeights := make([]float64, len(roles))
		for _, role := range roles {
			gap := siteRowGap(len(byRole[role]))
			rowWidth := gap
			for _, nodeID := range byRole[role] {
				rowWidth += siteNodeWidth(nodesByID[nodeID], degrees[nodeID]) + gap
			}
			maxRowWidth = math.Max(maxRowWidth, rowWidth)
		}
		for index, role := range roles {
			rowHeight := siteRoleHeight
			for _, nodeID := range byRole[role] {
				rowHeight = math.Max(rowHeight, siteNodeHeight(nodesByID[nodeID], degrees[nodeID])+123)
			}
			rowHeights[index] = rowHeight
		}
		planHeight := siteHeader + sitePadding
		for _, rowHeight := range rowHeights {
			planHeight += rowHeight
		}
		plans = append(plans, sitePlan{
			ID: rootID, Label: label, Kind: kind, NodeIDs: nodeIDs, Roles: roles, ByRole: byRole, RowHeights: rowHeights,
			Width: math.Max(520, maxRowWidth+sitePadding*2), Height: planHeight,
		})
	}

	result := layoutResult{Nodes: make(map[string]placedNode)}
	x := 70.0
	siteY := headerHeight + 55
	rowHeight := 0.0
	maxRight := 0.0
	for _, plan := range plans {
		if x > 70 && x+plan.Width+70 > siteCanvasMax {
			x = 70
			siteY += rowHeight + siteGap
			rowHeight = 0
		}
		siteBox := box{X: x, Y: siteY, W: plan.Width, H: plan.Height}
		result.Groups = append(result.Groups, placedGroup{ID: plan.ID, Label: plan.Label, Kind: plan.Kind, Box: siteBox})
		rowY := siteBox.Y + siteHeader + 42
		for row, role := range plan.Roles {
			ids := plan.ByRole[role]
			rowWidth := 0.0
			for _, nodeID := range ids {
				rowWidth += siteNodeWidth(nodesByID[nodeID], degrees[nodeID])
			}
			gap := siteRowGap(len(ids))
			rowWidth += float64(len(ids)-1) * gap
			nodeX := siteBox.X + (siteBox.W-rowWidth)/2
			nodeY := rowY
			for _, nodeID := range ids {
				node := nodesByID[nodeID]
				width := siteNodeWidth(node, degrees[nodeID])
				height := siteNodeHeight(node, degrees[nodeID])
				result.Nodes[nodeID] = placedNode{ID: nodeID, Node: node, Box: box{X: nodeX, Y: nodeY, W: width, H: height}}
				nodeX += width + gap
			}
			rowY += plan.RowHeights[row]
		}
		x += plan.Width + siteGap
		rowHeight = math.Max(rowHeight, plan.Height)
		maxRight = math.Max(maxRight, siteBox.X+siteBox.W)
		result.Height = math.Max(result.Height, siteBox.Y+siteBox.H+80)
	}
	result.Width = math.Max(canvasWidth, maxRight+70)

	for _, rootID := range children[""] {
		appendNestedGroupBoxes(&result, rootID, 1, groupsByID, children)
	}
	sort.SliceStable(result.Groups, func(i, j int) bool { return result.Groups[i].Depth < result.Groups[j].Depth })
	return result
}

func nodeDegrees(doc *model.Diagram) map[string]int {
	result := make(map[string]int)
	for _, link := range doc.Links {
		result[link.From.Node]++
		result[link.To.Node]++
	}
	return result
}

func siteNodeWidth(node model.Node, degree int) float64 {
	if node.Width > 0 {
		return node.Width
	}
	width := nodeWidth(node.Role)
	if degree <= 4 {
		return width
	}
	return width + float64(degree-4)*70
}

func siteNodeHeight(node model.Node, degree int) float64 {
	if node.Height > 0 {
		return node.Height
	}
	if degree <= 4 {
		return nodeHeight
	}
	return nodeHeight + float64(degree-4)*22
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
