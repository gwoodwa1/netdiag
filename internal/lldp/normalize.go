package lldp

import (
	"strings"
	"unicode"
)

func completeNeighbors(neighbors []Neighbor) []Neighbor {
	result := make([]Neighbor, 0, len(neighbors))
	for _, neighbor := range neighbors {
		if neighbor.LocalPort != "" && neighbor.PortID != "" && firstUseful(neighbor.SystemName, neighbor.ChassisID, neighbor.ManagementAddress) != "" {
			result = append(result, neighbor)
		}
	}
	return result
}

func normalizeKey(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func useful(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "null", "not advertised", "n/a", "-":
		return ""
	default:
		return value
	}
}

func nodeID(value string) string {
	value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	var out strings.Builder
	dash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' {
			out.WriteRune(r)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func firstUseful(values ...string) string {
	for _, value := range values {
		if value = useful(value); value != "" {
			return value
		}
	}
	return ""
}

func parsePositiveInteger(value string) (int, bool) {
	result := 0
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		result = result*10 + int(r-'0')
	}
	return result, true
}
