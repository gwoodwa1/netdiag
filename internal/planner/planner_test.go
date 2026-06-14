package planner

import (
	"testing"

	"github.com/gwoodwa1/netdiag/internal/model"
)

func TestRecommendD2ForNestedGroupsAndParallelLinks(t *testing.T) {
	diagram := &model.Diagram{
		Groups: []model.Group{{ID: "site"}, {ID: "rack", ParentID: "site"}},
		Nodes:  []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: model.LinkEndpoint{Node: "b", Port: "Eth0/0"}},
			{From: model.LinkEndpoint{Node: "a", Port: "Eth0/1"}, To: model.LinkEndpoint{Node: "b", Port: "Eth0/1"}},
		},
	}
	if got := Recommend(diagram); got != "d2" {
		t.Fatalf("Recommend() = %q, want d2", got)
	}
}

func TestPlanWarnsAboutD2EndpointSides(t *testing.T) {
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{{
			From: model.LinkEndpoint{Node: "a", Port: "Eth0/0", Side: "right"},
			To:   model.LinkEndpoint{Node: "b", Port: "Eth0/0", Side: "left"},
		}},
	}
	plan, err := Build(diagram, "d2")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.BestEffort) != 1 || plan.BestEffort[0].Feature != FeatureEndpointSides {
		t.Fatalf("unexpected best-effort assessment: %+v", plan.BestEffort)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected a capability warning")
	}
}

func TestCapabilitiesUseThreeSupportLevels(t *testing.T) {
	seen := map[SupportLevel]bool{}
	for _, renderer := range Capabilities() {
		for _, capability := range renderer.Capabilities {
			seen[capability.Level] = true
		}
	}
	for _, level := range []SupportLevel{Strict, BestEffort, Unsupported} {
		if !seen[level] {
			t.Fatalf("capabilities do not contain %q", level)
		}
	}
}

func TestEveryRendererAdvertisesEveryKnownCapability(t *testing.T) {
	for _, renderer := range Capabilities() {
		if len(renderer.Capabilities) != len(capabilityNotes) {
			t.Fatalf("%s advertises %d capabilities, want %d", renderer.Renderer, len(renderer.Capabilities), len(capabilityNotes))
		}
	}
}

func TestPlanSupportsDrawIO(t *testing.T) {
	diagram := &model.Diagram{
		Nodes: []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{{
			From: model.LinkEndpoint{Node: "a", Port: "Eth0/0"},
			To:   model.LinkEndpoint{Node: "b", Port: "Eth0/0"},
		}},
	}
	plan, err := Build(diagram, "drawio")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Renderer != "drawio" {
		t.Fatalf("renderer = %q, want drawio", plan.Renderer)
	}
}

func TestRecommendNativeForSiteAwareLayout(t *testing.T) {
	diagram := &model.Diagram{
		Theme:  model.Theme{Layout: "sites", LinkStyle: "orthogonal"},
		Groups: []model.Group{{ID: "west"}, {ID: "east"}},
		Nodes:  []model.Node{{ID: "a"}, {ID: "b"}},
		Links: []model.Link{
			{From: model.LinkEndpoint{Node: "a", Port: "Eth0/0"}, To: model.LinkEndpoint{Node: "b", Port: "Eth0/0"}},
			{From: model.LinkEndpoint{Node: "a", Port: "Eth0/1"}, To: model.LinkEndpoint{Node: "b", Port: "Eth0/1"}},
		},
	}
	if got := Recommend(diagram); got != "native" {
		t.Fatalf("Recommend() = %q, want native", got)
	}
	plan, err := Build(diagram, "native")
	if err != nil {
		t.Fatal(err)
	}
	for _, assessment := range plan.Strict {
		if assessment.Feature == FeatureGroups {
			return
		}
	}
	t.Fatal("site-aware native plan did not promote groups to strict")
}
