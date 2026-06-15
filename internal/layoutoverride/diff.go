package layoutoverride

import (
	"reflect"
	"sort"
)

type Diff struct {
	Nodes  Changes `json:"nodes"`
	Groups Changes `json:"groups"`
	Links  Changes `json:"links"`
}

type Changes struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Changed []string `json:"changed,omitempty"`
}

func Compare(oldDoc, newDoc *Document) Diff {
	return Diff{
		Nodes:  compareMaps(oldDoc.LayoutOverrides.Nodes, newDoc.LayoutOverrides.Nodes),
		Groups: compareMaps(oldDoc.LayoutOverrides.Groups, newDoc.LayoutOverrides.Groups),
		Links:  compareMaps(oldDoc.LayoutOverrides.Links, newDoc.LayoutOverrides.Links),
	}
}

func (diff Diff) Empty() bool {
	return changesEmpty(diff.Nodes) && changesEmpty(diff.Groups) && changesEmpty(diff.Links)
}

func compareMaps[T any](oldValues, newValues map[string]T) Changes {
	var result Changes
	for id, oldValue := range oldValues {
		newValue, ok := newValues[id]
		if !ok {
			result.Removed = append(result.Removed, id)
		} else if !reflect.DeepEqual(oldValue, newValue) {
			result.Changed = append(result.Changed, id)
		}
	}
	for id := range newValues {
		if _, ok := oldValues[id]; !ok {
			result.Added = append(result.Added, id)
		}
	}
	sort.Strings(result.Added)
	sort.Strings(result.Removed)
	sort.Strings(result.Changed)
	return result
}

func changesEmpty(changes Changes) bool {
	return len(changes.Added) == 0 && len(changes.Removed) == 0 && len(changes.Changed) == 0
}
