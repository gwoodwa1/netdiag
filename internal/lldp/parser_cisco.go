package lldp

import "strings"

func parseCisco(data []byte) (Result, error) {
	return parseText(data, "cisco")
}

func parseCiscoSummary(text string) []Neighbor {
	lines := strings.Split(text, "\n")
	inTable := false
	var result []Neighbor
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "INDEX PORT DEVICE ID PORT ID NAME CAPABILITIES TTL") {
			inTable = true
			continue
		}
		if !inTable || trimmed == "" || strings.HasPrefix(trimmed, "-") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 7 {
			continue
		}
		if _, ok := parsePositiveInteger(fields[0]); !ok {
			continue
		}
		ttlIndex := len(fields) - 1
		if _, ok := parsePositiveInteger(fields[ttlIndex]); !ok {
			continue
		}
		result = append(result, Neighbor{
			LocalPort: fields[1], ChassisID: fields[2], PortID: fields[3],
			SystemName: fields[4], Capabilities: strings.Join(fields[5:ttlIndex], " "),
		})
	}
	return result
}

func parseCiscoIOSXRSummary(text string) []Neighbor {
	lines := strings.Split(text, "\n")
	inTable := false
	var result []Neighbor
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "DEVICE ID") && strings.Contains(upper, "LOCAL INTF") && strings.Contains(upper, "HOLD-TIME") && strings.Contains(upper, "PORT ID") {
			inTable = true
			continue
		}
		if !inTable || trimmed == "" || strings.HasPrefix(upper, "TOTAL ENTRIES") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}
		if _, ok := parsePositiveInteger(fields[2]); !ok {
			continue
		}
		result = append(result, Neighbor{
			LocalPort: fields[1], PortID: fields[len(fields)-1],
			SystemName: fields[0], Capabilities: useful(strings.Join(fields[3:len(fields)-1], " ")),
		})
	}
	return result
}
