package yamlutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var locationPattern = regexp.MustCompile(`(?m)(?:yaml: )?line ([0-9]+)(?::|, column ([0-9]+):) (.*)`)

// DecodeStrict decodes YAML while rejecting unknown fields and adds a compact
// source excerpt to parser errors.
func DecodeStrict(data []byte, value interface{}) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(value); err != nil {
		return Context(data, err)
	}
	return nil
}

// Context turns yaml.v3's line-oriented errors into compiler-style diagnostics.
func Context(data []byte, err error) error {
	if err == nil {
		return nil
	}
	match := locationPattern.FindStringSubmatch(err.Error())
	if len(match) == 0 {
		return err
	}
	line, conversionErr := strconv.Atoi(match[1])
	if conversionErr != nil {
		return err
	}
	column := 0
	if match[2] != "" {
		column, _ = strconv.Atoi(match[2])
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	start, end := max(1, line-1), min(len(lines), line+1)
	var excerpt strings.Builder
	for current := start; current <= end; current++ {
		marker := " "
		if current == line {
			marker = ">"
		}
		fmt.Fprintf(&excerpt, "\n%s %4d | %s", marker, current, lines[current-1])
		if current == line && column > 0 {
			fmt.Fprintf(&excerpt, "\n       | %s^", strings.Repeat(" ", max(0, column-1)))
		}
	}

	location := fmt.Sprintf("line %d", line)
	if column > 0 {
		location += fmt.Sprintf(", column %d", column)
	}
	return fmt.Errorf("%s: %s%s", location, messageHint(match[3]), excerpt.String())
}

func messageHint(message string) string {
	if strings.Contains(message, "field label_rotation not found") ||
		strings.Contains(message, "field label_along not found") ||
		strings.Contains(message, "field label_offset not found") {
		return message + "; label_rotation, label_along, and label_offset are endpoint settings and must be nested under a link's from: or to: block"
	}
	return message
}
