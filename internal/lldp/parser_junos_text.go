package lldp

import (
	"fmt"
	"strings"
)

func parseJunos(data []byte) (Result, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	result := Result{Neighbors: parseJunosDetail(text)}
	if len(result.Neighbors) == 0 {
		result.Neighbors = parseJunosSummary(text)
	}
	if len(result.Neighbors) == 0 {
		if loose, err := parseText(data, "juniper"); err == nil {
			result.Neighbors = loose.Neighbors
		}
	}
	if match := junosPrompt.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = match[1]
	}
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no LLDP neighbors found in juniper text")
	}
	return result, nil
}

func parseJunosDetail(text string) []Neighbor {
	var result []Neighbor
	var current *Neighbor
	section := ""
	flush := func() {
		if current != nil {
			result = append(result, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "lldp neighbor information:"):
			flush()
			current = &Neighbor{}
			section = ""
			continue
		case lower == "local information:":
			section = "local"
			continue
		case lower == "neighbour information:" || lower == "neighbor information:":
			section = "neighbor"
			continue
		case lower == "system capabilities":
			section = "capabilities"
			continue
		case lower == "management address":
			section = "management"
			continue
		case lower == "organization info":
			section = "organization"
			continue
		}
		if current == nil {
			continue
		}
		match := fieldPattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		key, value := normalizeKey(match[1]), useful(match[2])
		switch section {
		case "local":
			if key == "localinterface" {
				current.LocalPort = value
			}
		case "neighbor":
			switch key {
			case "chassisid":
				current.ChassisID = value
			case "portid":
				current.PortID = value
			case "portdescription":
				current.PortDescription = value
			case "systemname":
				current.SystemName = value
			case "systemdescription":
				current.SystemDescription = value
			}
		case "capabilities":
			if key == "enabled" {
				current.Capabilities = value
			}
		case "management":
			if key == "address" && current.ManagementAddress == "" {
				current.ManagementAddress = value
			}
		}
	}
	flush()
	return completeNeighbors(result)
}

func parseJunosSummary(text string) []Neighbor {
	lines := strings.Split(text, "\n")
	inTable := false
	var result []Neighbor
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "LOCAL INTERFACE") && strings.Contains(upper, "PARENT INTERFACE") && strings.Contains(upper, "CHASSIS ID") && strings.Contains(upper, "PORT INFO") && strings.Contains(upper, "SYSTEM NAME") {
			inTable = true
			continue
		}
		if !inTable || trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}
		result = append(result, Neighbor{LocalPort: fields[0], ChassisID: fields[2], PortID: fields[3], SystemName: strings.Join(fields[4:], " ")})
	}
	return result
}
