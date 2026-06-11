package spec

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

func Format(doc *Document) ([]byte, error) {
	return yaml.Marshal(doc)
}

func JSONSchema() ([]byte, error) {
	schema := map[string]interface{}{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://netdiag.dev/schema/v1.json",
		"title":                "netdiag diagram",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"version", "nodes", "links"},
		"properties": map[string]interface{}{
			"version": map[string]interface{}{"const": 1},
			"diagram": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"title":            map[string]interface{}{"type": "string"},
					"subtitle":         map[string]interface{}{"type": "string"},
					"badge":            map[string]interface{}{"type": "string"},
					"layout":           map[string]interface{}{"enum": []string{"auto", "rows", "ring", "sites", "manual", "elk"}},
					"direction":        map[string]interface{}{"enum": []string{"up", "down", "left", "right"}},
					"link_style":       map[string]interface{}{"type": "string"},
					"interface_labels": map[string]interface{}{"type": "string"},
					"theme":            map[string]interface{}{"type": "string"},
					"renderer":         map[string]interface{}{"enum": []string{"native", "d2"}},
				},
			},
			"groups": map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"$ref": "#/$defs/group"}},
			"nodes":  map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"$ref": "#/$defs/node"}, "minProperties": 1},
			"links":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/$defs/link"}},
		},
		"$defs": map[string]interface{}{
			"endpoint": map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{"type": "string", "pattern": "^[^:]+:.+$"},
					map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"node", "port"},
						"properties": map[string]interface{}{
							"node":    map[string]interface{}{"type": "string"},
							"port":    map[string]interface{}{"type": "string"},
							"side":    map[string]interface{}{"enum": []string{"top", "right", "bottom", "left"}},
							"label":   map[string]interface{}{"type": "string"},
							"address": map[string]interface{}{"type": "string"},
						},
					},
				},
			},
			"group": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"label":  map[string]interface{}{"type": "string"},
					"kind":   map[string]interface{}{"type": "string"},
					"groups": map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"$ref": "#/$defs/group"}},
					"nodes":  map[string]interface{}{"type": "object"},
				},
			},
			"node": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"role"},
				"properties": map[string]interface{}{
					"label":    map[string]interface{}{"type": "string"},
					"role":     map[string]interface{}{"type": "string"},
					"icon":     map[string]interface{}{"type": "string"},
					"color":    map[string]interface{}{"type": "string"},
					"order":    map[string]interface{}{"type": "integer"},
					"metadata": map[string]interface{}{"type": "object"},
				},
			},
			"link": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"from", "to"},
				"properties": map[string]interface{}{
					"from":          map[string]interface{}{"$ref": "#/$defs/endpoint"},
					"to":            map[string]interface{}{"$ref": "#/$defs/endpoint"},
					"label":         map[string]interface{}{"type": "string"},
					"style":         map[string]interface{}{"type": "string"},
					"bundle":        map[string]interface{}{"type": "string"},
					"lacp":          map[string]interface{}{"type": "boolean"},
					"multi_chassis": map[string]interface{}{"type": "boolean"},
					"trunk":         map[string]interface{}{"type": "object"},
					"labels":        map[string]interface{}{"$ref": "#/$defs/linkLabels"},
				},
			},
			"linkLabels": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"source": map[string]interface{}{"type": "string"},
					"middle": map[string]interface{}{"type": "string"},
					"target": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	return json.MarshalIndent(schema, "", "  ")
}
