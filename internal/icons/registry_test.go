package icons

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListIsSortedAndAliasesResolve(t *testing.T) {
	items := List()
	if len(items) < 10 {
		t.Fatalf("expected polished built-in icon catalog, got %d", len(items))
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].ID >= items[i].ID {
			t.Fatalf("icon list is not sorted: %#v", items)
		}
	}
	icon, ok := Resolve("route-reflector")
	if !ok || icon.ID != "router" {
		t.Fatalf("route reflector alias did not resolve: %#v", icon)
	}
}

func TestListJSONUsesArraysForEmptyAliases(t *testing.T) {
	data, err := json.Marshal(List())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"aliases":null`) {
		t.Fatalf("empty aliases must be JSON arrays: %s", data)
	}
}
