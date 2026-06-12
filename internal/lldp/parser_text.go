package lldp

import (
	"fmt"
	"strings"
)

func parseText(data []byte, vendor string) (Result, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	result := Result{}
	if match := promptPattern.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = match[1]
	}
	if vendor == "cisco" {
		result.Neighbors = append(result.Neighbors, parseCiscoSummary(text)...)
		result.Neighbors = append(result.Neighbors, parseCiscoIOSXRSummary(text)...)
	}
	var current *Neighbor
	var aristaLocal string
	flush := func() {
		if current != nil {
			result.Neighbors = append(result.Neighbors, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if vendor == "arista" && strings.HasPrefix(lower, "interface ") && strings.Contains(lower, " lldp neighbor") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				aristaLocal = fields[1]
			}
			continue
		}
		match := fieldPattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		key, value := normalizeKey(match[1]), useful(match[2])
		if key == "chassisid" {
			pendingLocal := ""
			if current != nil && current.ChassisID == "" && current.PortID == "" {
				pendingLocal = current.LocalPort
				current = nil
			}
			flush()
			current = &Neighbor{ChassisID: value, LocalPort: firstUseful(aristaLocal, pendingLocal)}
			continue
		}
		if current == nil {
			current = &Neighbor{LocalPort: aristaLocal}
		}
		switch key {
		case "localportid", "localinterface", "localintf":
			current.LocalPort = value
		case "portid":
			current.PortID = value
		case "portdescription":
			current.PortDescription = value
		case "systemname":
			current.SystemName = value
		case "systemdescription":
			current.SystemDescription = value
		case "managementaddress", "managementaddressipv4", "managementaddressipv6":
			if current.ManagementAddress == "" {
				current.ManagementAddress = value
			}
		case "enabledcapabilities", "systemcapabilities":
			if current.Capabilities == "" {
				current.Capabilities = value
			}
		}
	}
	flush()
	result.Neighbors = completeNeighbors(result.Neighbors)
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no LLDP neighbors found in %s text", vendor)
	}
	return result, nil
}
