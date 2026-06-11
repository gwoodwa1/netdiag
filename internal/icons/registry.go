package icons

import "sort"

type Icon struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Color       string   `json:"color"`
	Aliases     []string `json:"aliases"`
}

var catalog = []Icon{
	{ID: "router", Category: "routing", Description: "Layer 3 router", Color: "#0878b9", Aliases: []string{"edge-router", "ospf-backbone", "ospf-area-10", "ospf-area-20", "isis-level-1", "isis-level-2", "route-reflector", "rr-client", "external-peer", "core-router"}},
	{ID: "spine", Category: "switching", Description: "High-capacity fabric spine", Color: "#2563eb", Aliases: []string{"super-spine"}},
	{ID: "leaf", Category: "switching", Description: "Leaf or access switch", Color: "#0891b2", Aliases: []string{"switch", "core-switch", "distribution-switch", "access-switch", "metro-switch"}},
	{ID: "firewall", Category: "security", Description: "Network firewall", Color: "#dc2626", Aliases: []string{}},
	{ID: "cloud", Category: "cloud", Description: "WAN or internet cloud", Color: "#64748b", Aliases: []string{"wan-cloud", "internet"}},
	{ID: "public-cloud", Category: "cloud", Description: "Public cloud provider", Color: "#f59e0b", Aliases: []string{"aws"}},
	{ID: "dwdm", Category: "transport", Description: "DWDM optical transport", Color: "#7c3aed", Aliases: []string{}},
	{ID: "wireless", Category: "access", Description: "Wireless access point", Color: "#0878b9", Aliases: []string{"access-point"}},
	{ID: "endpoint", Category: "endpoint", Description: "User or industrial endpoint", Color: "#475569", Aliases: []string{"users"}},
	{ID: "server", Category: "compute", Description: "Physical or virtual server", Color: "#16a34a", Aliases: []string{}},
}

func List() []Icon {
	items := make([]Icon, len(catalog))
	for i, icon := range catalog {
		items[i] = icon
		items[i].Aliases = make([]string, len(icon.Aliases))
		copy(items[i].Aliases, icon.Aliases)
		sort.Strings(items[i].Aliases)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func Resolve(id string) (Icon, bool) {
	for _, icon := range catalog {
		if icon.ID == id {
			return icon, true
		}
		for _, alias := range icon.Aliases {
			if alias == id {
				return icon, true
			}
		}
	}
	return Icon{}, false
}
