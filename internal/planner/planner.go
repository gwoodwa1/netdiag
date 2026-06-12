package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/model"
)

type SupportLevel string

const (
	Unsupported SupportLevel = "unsupported"
	BestEffort  SupportLevel = "best_effort"
	Strict      SupportLevel = "strict"
)

type Capability struct {
	Feature string       `json:"feature"`
	Level   SupportLevel `json:"level"`
	Note    string       `json:"note,omitempty"`
}

type RendererCapabilities struct {
	Renderer     string       `json:"renderer"`
	Capabilities []Capability `json:"capabilities"`
}

// RendererCapability is the planner-facing contract implemented by every
// renderer. Renderers consume model.Diagram, the renderer-neutral IR, and
// advertise how faithfully they handle each feature required by that IR.
type RendererCapability interface {
	Name() string
	Support(diagram *model.Diagram, feature string) SupportLevel
}

type Assessment struct {
	Feature string       `json:"feature"`
	Level   SupportLevel `json:"level"`
	Reason  string       `json:"reason"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Plan struct {
	Renderer            string       `json:"renderer"`
	RecommendedRenderer string       `json:"recommended_renderer"`
	Strict              []Assessment `json:"strict"`
	BestEffort          []Assessment `json:"best_effort"`
	Unsupported         []Assessment `json:"unsupported"`
	Warnings            []Warning    `json:"warnings"`
}

type RenderReport struct {
	Renderer            string       `json:"renderer"`
	RecommendedRenderer string       `json:"recommended_renderer"`
	Layout              string       `json:"layout"`
	Output              string       `json:"output"`
	Features            []Assessment `json:"features"`
	Warnings            []Warning    `json:"warnings"`
}

const (
	FeatureGroups            = "groups"
	FeatureNestedGroups      = "nested_groups"
	FeatureEndpointSides     = "endpoint_sides"
	FeatureEndpointAddresses = "endpoint_addresses"
	FeatureSourceLabels      = "source_labels"
	FeatureMiddleLabels      = "middle_labels"
	FeatureTargetLabels      = "target_labels"
	FeatureParallelLinks     = "parallel_links"
	FeatureNetworkCards      = "network_cards"
	FeatureCustomIcons       = "custom_icons"
	FeatureManualPlacement   = "manual_placement"
	FeatureSiteAwareLayout   = "site_aware_layout"
	FeatureOrthogonalRouting = "orthogonal_routing"
)

var capabilityNotes = map[string]string{
	FeatureGroups:            "render group boundaries",
	FeatureNestedGroups:      "render groups inside groups",
	FeatureEndpointSides:     "honor explicit top/right/bottom/left attachment hints",
	FeatureEndpointAddresses: "place endpoint IP addresses in a separate label lane",
	FeatureSourceLabels:      "place source-side link labels",
	FeatureMiddleLabels:      "place middle link labels",
	FeatureTargetLabels:      "place target-side link labels",
	FeatureParallelLinks:     "route multiple links between the same nodes distinctly",
	FeatureNetworkCards:      "render network-specific device cards",
	FeatureCustomIcons:       "render the built-in network icon library",
	FeatureManualPlacement:   "honor manual node coordinates",
	FeatureSiteAwareLayout:   "place devices inside site-aware network containers",
	FeatureOrthogonalRouting: "route links orthogonally around device cards",
}

type staticRendererCapability struct {
	name   string
	levels map[string]SupportLevel
}

func (renderer staticRendererCapability) Name() string {
	return renderer.name
}

func (renderer staticRendererCapability) Support(diagram *model.Diagram, feature string) SupportLevel {
	if renderer.name == "native" && diagram.Theme.Layout == "sites" {
		switch feature {
		case FeatureGroups, FeatureParallelLinks:
			return Strict
		}
	}
	return renderer.levels[feature]
}

var renderers = []RendererCapability{
	staticRendererCapability{name: "native", levels: map[string]SupportLevel{
		FeatureGroups: BestEffort, FeatureNestedGroups: BestEffort,
		FeatureEndpointSides: Strict, FeatureEndpointAddresses: Strict,
		FeatureSourceLabels: Strict, FeatureMiddleLabels: Strict, FeatureTargetLabels: Strict,
		FeatureParallelLinks: BestEffort, FeatureNetworkCards: Strict, FeatureCustomIcons: Strict,
		FeatureManualPlacement: Unsupported,
		FeatureSiteAwareLayout: Strict, FeatureOrthogonalRouting: Strict,
	}},
	staticRendererCapability{name: "d2", levels: map[string]SupportLevel{
		FeatureGroups: Strict, FeatureNestedGroups: Strict,
		FeatureEndpointSides: BestEffort, FeatureEndpointAddresses: BestEffort,
		FeatureSourceLabels: Strict, FeatureMiddleLabels: Strict, FeatureTargetLabels: Strict,
		FeatureParallelLinks: Strict, FeatureNetworkCards: Unsupported, FeatureCustomIcons: Unsupported,
		FeatureManualPlacement: Unsupported,
		FeatureSiteAwareLayout: Unsupported, FeatureOrthogonalRouting: BestEffort,
	}},
}

func Capabilities() []RendererCapabilities {
	result := make([]RendererCapabilities, 0, len(renderers))
	for _, renderer := range renderers {
		features := make([]string, 0, len(capabilityNotes))
		for feature := range capabilityNotes {
			features = append(features, feature)
		}
		sort.Strings(features)
		caps := RendererCapabilities{Renderer: renderer.Name()}
		for _, feature := range features {
			caps.Capabilities = append(caps.Capabilities, Capability{Feature: feature, Level: renderer.Support(&model.Diagram{}, feature), Note: capabilityNotes[feature]})
		}
		result = append(result, caps)
	}
	return result
}

func Recommend(diagram *model.Diagram) string {
	native := score(diagram, "native")
	d2 := score(diagram, "d2")
	if d2 > native {
		return "d2"
	}
	return "native"
}

func Build(diagram *model.Diagram, renderer string) (Plan, error) {
	capability, ok := rendererByName(renderer)
	if !ok {
		return Plan{}, fmt.Errorf("unknown renderer %q; use native or d2", renderer)
	}
	plan := Plan{
		Renderer: renderer, RecommendedRenderer: Recommend(diagram),
		Strict: []Assessment{}, BestEffort: []Assessment{}, Unsupported: []Assessment{}, Warnings: []Warning{},
	}
	for _, requirement := range requirements(diagram) {
		assessment := Assessment{Feature: requirement.Feature, Level: capability.Support(diagram, requirement.Feature), Reason: requirement.Reason}
		switch assessment.Level {
		case Strict:
			plan.Strict = append(plan.Strict, assessment)
		case BestEffort:
			plan.BestEffort = append(plan.BestEffort, assessment)
			plan.Warnings = append(plan.Warnings, Warning{
				Code:    "best_effort_" + requirement.Feature,
				Message: fmt.Sprintf("%s renderer handles %s as best effort: %s", renderer, strings.ReplaceAll(requirement.Feature, "_", " "), requirement.Reason),
			})
		case Unsupported:
			plan.Unsupported = append(plan.Unsupported, assessment)
			plan.Warnings = append(plan.Warnings, Warning{
				Code:    "unsupported_" + requirement.Feature,
				Message: fmt.Sprintf("%s renderer does not support %s: %s", renderer, strings.ReplaceAll(requirement.Feature, "_", " "), requirement.Reason),
			})
		}
	}
	if renderer != plan.RecommendedRenderer {
		plan.Warnings = append(plan.Warnings, Warning{
			Code:    "renderer_not_recommended",
			Message: fmt.Sprintf("%s is selected; %s is recommended for this diagram", renderer, plan.RecommendedRenderer),
		})
	}
	return plan, nil
}

func Report(plan Plan, layout, output string) RenderReport {
	features := append([]Assessment{}, plan.Strict...)
	features = append(features, plan.BestEffort...)
	features = append(features, plan.Unsupported...)
	sort.Slice(features, func(i, j int) bool { return features[i].Feature < features[j].Feature })
	warnings := append([]Warning{}, plan.Warnings...)
	return RenderReport{
		Renderer: plan.Renderer, RecommendedRenderer: plan.RecommendedRenderer,
		Layout: layout, Output: output, Features: features, Warnings: warnings,
	}
}

type requirement struct {
	Feature string
	Reason  string
	Weight  int
}

func requirements(diagram *model.Diagram) []requirement {
	found := make(map[string]requirement)
	add := func(feature, reason string, weight int) {
		if _, ok := found[feature]; !ok {
			found[feature] = requirement{Feature: feature, Reason: reason, Weight: weight}
		}
	}
	if len(diagram.Groups) > 0 {
		add(FeatureGroups, fmt.Sprintf("%d group(s) are declared", len(diagram.Groups)), 3)
	}
	for _, group := range diagram.Groups {
		if group.ParentID != "" {
			add(FeatureNestedGroups, "at least one group has a parent group", 5)
		}
	}
	pairs := make(map[string]int)
	for _, link := range diagram.Links {
		if link.From.Side != "" || link.To.Side != "" {
			add(FeatureEndpointSides, "at least one endpoint has an explicit side", 4)
		}
		if link.From.Address != "" || link.To.Address != "" {
			add(FeatureEndpointAddresses, "at least one endpoint has a CIDR address", 3)
		}
		if link.Labels.Source != "" {
			add(FeatureSourceLabels, "source interface labels are required", 2)
		}
		if link.Labels.Middle != "" {
			add(FeatureMiddleLabels, "at least one middle link label is required", 2)
		}
		if link.Labels.Target != "" {
			add(FeatureTargetLabels, "target interface labels are required", 2)
		}
		nodes := []string{link.From.Node, link.To.Node}
		sort.Strings(nodes)
		pairs[strings.Join(nodes, "\x00")]++
	}
	for _, count := range pairs {
		if count > 1 {
			add(FeatureParallelLinks, fmt.Sprintf("at least one node pair has %d parallel links", count), 5)
			break
		}
	}
	for _, node := range diagram.Nodes {
		if node.Icon != "" {
			add(FeatureCustomIcons, "at least one node selects a network icon", 4)
			add(FeatureNetworkCards, "network-specific device presentation is requested", 3)
			break
		}
	}
	if diagram.Theme.Layout == "manual" {
		add(FeatureManualPlacement, "diagram.layout is manual", 5)
	}
	if diagram.Theme.Layout == "sites" {
		add(FeatureSiteAwareLayout, "diagram.layout is sites", 8)
		add(FeatureOrthogonalRouting, "site-aware layout uses native orthogonal routing", 6)
	}
	if diagram.Theme.LinkStyle == "orthogonal" {
		add(FeatureOrthogonalRouting, "diagram.link_style is orthogonal", 5)
	}
	result := make([]requirement, 0, len(found))
	for _, item := range found {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Feature < result[j].Feature })
	return result
}

func score(diagram *model.Diagram, renderer string) int {
	capability, ok := rendererByName(renderer)
	if !ok {
		return 0
	}
	score := 0
	for _, item := range requirements(diagram) {
		switch capability.Support(diagram, item.Feature) {
		case Strict:
			score += item.Weight * 2
		case BestEffort:
			score += item.Weight
		case Unsupported:
			score -= item.Weight * 2
		}
	}
	return score
}

func rendererByName(name string) (RendererCapability, bool) {
	for _, renderer := range renderers {
		if renderer.Name() == name {
			return renderer, true
		}
	}
	return nil, false
}
