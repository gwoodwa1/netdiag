package isis

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	executionHostPattern = regexp.MustCompile(`(?m)^\+{3}\s*([^:]+):\s*executing command ['"]show isis neighbors`)
	xrPromptPattern      = regexp.MustCompile(`(?im)^(?:[A-Za-z0-9/]+:)?([A-Za-z0-9_.-]+)[>#]\s*show\s+isis\s+neighbors`)
	instancePattern      = regexp.MustCompile(`(?i)^IS-IS\s+(.+?)\s+neighbors:$`)
)

func ParseIOSXR(data []byte) (Result, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	result := Result{}
	if match := executionHostPattern.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = strings.TrimSpace(match[1])
	} else if match := xrPromptPattern.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = match[1]
	}

	instance := ""
	inTable := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := instancePattern.FindStringSubmatch(trimmed); len(match) > 0 {
			instance = strings.TrimSpace(match[1])
			inTable = false
			continue
		}
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "SYSTEM ID") && strings.Contains(upper, "INTERFACE") && strings.Contains(upper, "SNPA") && strings.Contains(upper, "HOLDTIME") {
			inTable = true
			continue
		}
		if !inTable || trimmed == "" || strings.HasPrefix(upper, "TOTAL NEIGHBOR COUNT") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 7 {
			continue
		}
		holdtime, err := strconv.Atoi(fields[4])
		if err != nil {
			continue
		}
		result.Neighbors = append(result.Neighbors, Neighbor{
			SystemID: fields[0], Interface: fields[1], SNPA: fields[2],
			State: fields[3], Holdtime: holdtime, Type: fields[5],
			IETFNSF: strings.Join(fields[6:], " "), Instance: instance,
		})
	}
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no IS-IS neighbors found in Cisco IOS XR text")
	}
	return result, nil
}
