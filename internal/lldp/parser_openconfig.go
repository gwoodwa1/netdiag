package lldp

import (
	"encoding/json"
	"fmt"
	"sort"
)

func parseOpenConfig(data []byte) (Result, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return Result{}, fmt.Errorf("parse OpenConfig LLDP JSON: %w", err)
	}
	result := Result{}
	walkOpenConfig(root, "", &result)
	result.Neighbors = completeNeighbors(result.Neighbors)
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no OpenConfig LLDP neighbors found")
	}
	return result, nil
}

func walkOpenConfig(value interface{}, localPort string, result *Result) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			walkOpenConfig(item, localPort, result)
		}
	case map[string]interface{}:
		if name := stringValue(typed, "name", "id"); name != "" && hasAnyKey(typed, "neighbors", "openconfig-lldp:neighbors") {
			localPort = name
		}
		if state := mapValue(typed, "state", "openconfig-lldp:state"); state != nil && looksLikeNeighbor(state) {
			result.Neighbors = append(result.Neighbors, neighborFromMaps(localPort, typed, state))
			return
		}
		for key, child := range typed {
			nextPort := localPort
			if stripPrefix(key) == "interface" {
				if childMap, ok := child.(map[string]interface{}); ok {
					nextPort = firstUseful(stringValue(childMap, "name", "id"), localPort)
				}
			}
			walkOpenConfig(child, nextPort, result)
		}
	}
}

func neighborFromMaps(localPort string, entry, state map[string]interface{}) Neighbor {
	return Neighbor{
		LocalPort: localPort, ChassisID: firstUseful(stringValue(state, "chassis-id"), stringValue(entry, "chassis-id")),
		PortID:          firstUseful(stringValue(state, "port-id"), stringValue(entry, "port-id"), stringValue(entry, "id")),
		PortDescription: stringValue(state, "port-description"), SystemName: stringValue(state, "system-name"),
		SystemDescription: stringValue(state, "system-description"),
		ManagementAddress: structuredString(mapEntry(state, "management-address"), "address", "management-address"),
		Capabilities:      structuredString(mapEntry(state, "system-capabilities"), "name", "capability"),
	}
}

func stripPrefix(key string) string {
	for index, r := range key {
		if r == ':' {
			return key[index+1:]
		}
	}
	return key
}

func mapValue(value map[string]interface{}, keys ...string) map[string]interface{} {
	for key, child := range value {
		for _, wanted := range keys {
			if stripPrefix(key) == stripPrefix(wanted) {
				if result, ok := child.(map[string]interface{}); ok {
					return result
				}
			}
		}
	}
	return nil
}

func stringValue(value map[string]interface{}, keys ...string) string {
	for key, child := range value {
		for _, wanted := range keys {
			if stripPrefix(key) == stripPrefix(wanted) {
				return firstString(child)
			}
		}
	}
	return ""
}

func mapEntry(value map[string]interface{}, keys ...string) interface{} {
	for key, child := range value {
		for _, wanted := range keys {
			if stripPrefix(key) == stripPrefix(wanted) {
				return child
			}
		}
	}
	return nil
}

func structuredString(value interface{}, preferredKeys ...string) string {
	if result, ok := value.(string); ok {
		return useful(result)
	}
	for _, key := range preferredKeys {
		if result := findStringForKey(value, key); result != "" {
			return result
		}
	}
	return firstString(value)
}

func findStringForKey(value interface{}, wanted string) string {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			if result := findStringForKey(item, wanted); result != "" {
				return result
			}
		}
	case map[string]interface{}:
		for key, item := range typed {
			if stripPrefix(key) == wanted {
				return firstString(item)
			}
		}
		for _, key := range sortedKeys(typed) {
			if result := findStringForKey(typed[key], wanted); result != "" {
				return result
			}
		}
	}
	return ""
}

func firstString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return useful(typed)
	case []interface{}:
		for _, item := range typed {
			if result := firstString(item); result != "" {
				return result
			}
		}
	case map[string]interface{}:
		for _, key := range sortedKeys(typed) {
			if result := firstString(typed[key]); result != "" {
				return result
			}
		}
	}
	return ""
}

func sortedKeys(value map[string]interface{}) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func hasAnyKey(value map[string]interface{}, keys ...string) bool {
	for key := range value {
		for _, wanted := range keys {
			if stripPrefix(key) == stripPrefix(wanted) {
				return true
			}
		}
	}
	return false
}

func looksLikeNeighbor(state map[string]interface{}) bool {
	return stringValue(state, "port-id") != "" && firstUseful(stringValue(state, "system-name"), stringValue(state, "chassis-id")) != ""
}
