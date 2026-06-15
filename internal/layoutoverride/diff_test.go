package layoutoverride

import (
	"reflect"
	"testing"
)

func TestCompareReportsSemanticLayoutChanges(t *testing.T) {
	x, movedX := 10.0, 20.0
	oldDoc := &Document{
		Version: 1,
		LayoutOverrides: Overrides{
			Nodes: map[string]Bounds{"changed": {X: &x}, "removed": {}},
			Links: map[string]Link{"same": {Style: "curved"}},
		},
	}
	newDoc := &Document{
		Version: 1,
		LayoutOverrides: Overrides{
			Nodes: map[string]Bounds{"changed": {X: &movedX}, "added": {}},
			Links: map[string]Link{"same": {Style: "curved"}},
		},
	}
	got := Compare(oldDoc, newDoc)
	want := Diff{Nodes: Changes{Added: []string{"added"}, Removed: []string{"removed"}, Changed: []string{"changed"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compare() = %+v, want %+v", got, want)
	}
	if got.Empty() {
		t.Fatal("changed diff reported empty")
	}
}
