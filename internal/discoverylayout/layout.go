package discoverylayout

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/gwoodwa1/netdiag/internal/spec"
)

const autoLayoutGroupSize = 10

var hostnameRoleSuffix = regexp.MustCompile(`(?i)^(.+)[-_.](?:PE|P|RR|R|SW|LEAF|SPINE)\d+$`)

type Report struct {
	Layout                 string
	Groups                 int
	SuppressedMiddleLabels int
	Grouping               string
}

// Apply selects a deterministic presentation for a discovered topology. It
// operates on the authored document so the resulting YAML remains inspectable
// and editable.
func Apply(doc *spec.Document) Report {
	report := Report{Layout: "rows", Grouping: "none"}
	nodeCount := len(doc.Nodes)
	if nodeCount <= 12 && isRing(doc) {
		doc.Diagram.Layout = "ring"
		report.Layout = "ring"
		return report
	}
	if nodeCount < 20 {
		doc.Diagram.Layout = "rows"
		return report
	}

	doc.Diagram.Layout = "sites"
	doc.Diagram.LinkStyle = "orthogonal"
	doc.Diagram.InterfaceAt = "ends"
	doc.Diagram.InterfaceLabelStyle = spec.InterfaceLabelStyle{
		Fill: "#ffffff", Color: "#334155", Border: "#94a3b8",
		Radius: floatPointer(6), PaddingX: floatPointer(10), PaddingY: floatPointer(5),
	}
	report.Layout = "sites"

	groups := hostnameGroups(doc)
	if len(groups) < 2 {
		groups = balancedGroups(doc)
		report.Grouping = "balanced"
	} else {
		report.Grouping = "hostname-prefix"
	}
	doc.Groups = groups
	report.Groups = len(groups)
	if hubSpokeTopology(doc) {
		doc.Diagram.Layout = "hub-spoke"
		doc.Diagram.LinkStyle = "clean"
		report.Layout = "hub-spoke"
	}
	report.SuppressedMiddleLabels = suppressRepeatedLabels(doc)
	return report
}

func hubSpokeTopology(doc *spec.Document) bool {
	coreNodes := 0
	for _, node := range doc.Nodes {
		if node.Role == "core-router" {
			coreNodes++
		}
	}
	return coreNodes >= 2 && len(doc.Groups) >= 5
}

func hostnameGroups(doc *spec.Document) map[string]*spec.Group {
	byPrefix := make(map[string][]string)
	for id, node := range doc.Nodes {
		prefix := hostnamePrefix(node.Label)
		if prefix == "" {
			prefix = hostnamePrefix(id)
		}
		byPrefix[prefix] = append(byPrefix[prefix], id)
	}
	prefixes := make([]string, 0, len(byPrefix))
	for prefix := range byPrefix {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	result := make(map[string]*spec.Group)
	for _, prefix := range prefixes {
		ids := byPrefix[prefix]
		if prefix == "" || len(ids) < 2 {
			return nil
		}
		sort.Strings(ids)
		id := groupID(prefix)
		for suffix := 2; result[id] != nil; suffix++ {
			id = fmt.Sprintf("%s-%d", groupID(prefix), suffix)
		}
		result[id] = &spec.Group{
			Label: prefix,
			Kind:  "discovered-cluster",
			Nodes: nodeSet(ids),
		}
	}
	return result
}

func floatPointer(value float64) *float64 {
	return &value
}

func balancedGroups(doc *spec.Document) map[string]*spec.Group {
	ids := make([]string, 0, len(doc.Nodes))
	for id := range doc.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make(map[string]*spec.Group)
	for start := 0; start < len(ids); start += autoLayoutGroupSize {
		end := start + autoLayoutGroupSize
		if end > len(ids) {
			end = len(ids)
		}
		index := start/autoLayoutGroupSize + 1
		id := fmt.Sprintf("cluster-%02d", index)
		result[id] = &spec.Group{
			Label: fmt.Sprintf("Discovered Cluster %02d", index),
			Kind:  "discovered-cluster",
			Nodes: nodeSet(ids[start:end]),
		}
	}
	return result
}

func hostnamePrefix(value string) string {
	value = strings.TrimSpace(value)
	if parts := hostnameRoleSuffix.FindStringSubmatch(value); len(parts) == 2 {
		return parts[1]
	}
	for _, separator := range []string{"-", "_", "."} {
		if index := strings.LastIndex(value, separator); index > 0 && digitsOnly(value[index+1:]) {
			return value[:index]
		}
	}
	return ""
}

func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}

func groupID(value string) string {
	var out strings.Builder
	dash := false
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '.' || char == '_' {
			out.WriteRune(char)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func nodeSet(ids []string) map[string]interface{} {
	result := make(map[string]interface{}, len(ids))
	for _, id := range ids {
		result[id] = map[string]interface{}{}
	}
	return result
}

func suppressRepeatedLabels(doc *spec.Document) int {
	counts := make(map[string]int)
	for _, link := range doc.Links {
		if link.Label != "" {
			counts[link.Label]++
		}
	}
	suppressed := 0
	for index := range doc.Links {
		if counts[doc.Links[index].Label] >= 8 {
			doc.Links[index].Label = ""
			suppressed++
		}
	}
	return suppressed
}

func isRing(doc *spec.Document) bool {
	if len(doc.Nodes) < 3 || len(doc.Links) != len(doc.Nodes) {
		return false
	}
	degree := make(map[string]int)
	for _, link := range doc.Links {
		degree[link.From.Node]++
		degree[link.To.Node]++
	}
	for id := range doc.Nodes {
		if degree[id] != 2 {
			return false
		}
	}
	return true
}
