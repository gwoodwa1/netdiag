package isis

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type openConfigContext struct {
	Instance  string
	Interface string
	Level     string
}

func parseOpenConfig(data []byte) (Result, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return Result{}, fmt.Errorf("parse OpenConfig IS-IS JSON: %w", err)
	}
	result := Result{}
	walkOpenConfig(root, openConfigContext{}, &result)
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no OpenConfig IS-IS adjacencies found")
	}
	return result, nil
}

func walkOpenConfig(value interface{}, context openConfigContext, result *Result) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			walkOpenConfig(item, context, result)
		}
	case map[string]interface{}:
		context = updateContext(typed, context)
		if state := childMap(typed, "state"); state != nil && looksLikeAdjacency(state) {
			result.Neighbors = append(result.Neighbors, neighborFromOpenConfig(typed, state, context))
			return
		}
		for _, child := range typed {
			walkOpenConfig(child, context, result)
		}
	}
}

func updateContext(value map[string]interface{}, context openConfigContext) openConfigContext {
	if hasChild(value, "isis") {
		if name := stringValue(value, "name"); name != "" {
			context.Instance = name
		}
	}
	if hasChild(value, "interfaces") || hasChild(value, "levels") {
		if name := stringValue(value, "name", "identifier"); name != "" {
			context.Instance = name
		}
	}
	if hasChild(value, "levels") || hasChild(value, "adjacencies") {
		if name := stringValue(value, "interface-id", "name", "id"); name != "" {
			context.Interface = name
		}
	}
	if hasChild(value, "adjacencies") {
		if level := stringValue(value, "level-number", "level"); level != "" {
			context.Level = normalizeLevel(level)
		}
	}
	return context
}

func looksLikeAdjacency(state map[string]interface{}) bool {
	return stringValue(state, "system-id", "neighbor-system-id") != "" &&
		stringValue(state, "adjacency-state", "state") != ""
}

func neighborFromOpenConfig(entry, state map[string]interface{}, context openConfigContext) Neighbor {
	level := firstNonEmpty(stringValue(state, "level-number", "level"), context.Level)
	return Neighbor{
		SystemID:  firstNonEmpty(stringValue(state, "system-id", "neighbor-system-id"), stringValue(entry, "system-id")),
		Interface: firstNonEmpty(stringValue(state, "interface-id", "interface"), context.Interface),
		SNPA:      stringValue(state, "snpa"),
		State:     normalizeState(stringValue(state, "adjacency-state", "state")),
		Holdtime:  integerValue(state, "remaining-hold-time", "hold-time"),
		Type:      normalizeLevel(level),
		IETFNSF:   stringValue(state, "restart-support", "ietf-nsf"),
		Instance:  context.Instance,
	}
}

func childMap(value map[string]interface{}, wanted string) map[string]interface{} {
	for key, child := range value {
		if localName(key) == wanted {
			if result, ok := child.(map[string]interface{}); ok {
				return result
			}
		}
	}
	return nil
}

func hasChild(value map[string]interface{}, wanted string) bool {
	for key := range value {
		if localName(key) == wanted {
			return true
		}
	}
	return false
}

func stringValue(value map[string]interface{}, wanted ...string) string {
	for key, child := range value {
		for _, name := range wanted {
			if localName(key) == name {
				return scalarString(child)
			}
		}
	}
	return ""
}

func integerValue(value map[string]interface{}, wanted ...string) int {
	result, _ := strconv.Atoi(stringValue(value, wanted...))
	return result
}

func scalarString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	}
	return ""
}

func localName(value string) string {
	if _, local, ok := strings.Cut(value, ":"); ok {
		return local
	}
	return value
}

func normalizeLevel(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "1", "LEVEL_1", "LEVEL-1":
		return "L1"
	case "2", "LEVEL_2", "LEVEL-2":
		return "L2"
	default:
		return value
	}
}

func normalizeState(value string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "adjacency_")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
