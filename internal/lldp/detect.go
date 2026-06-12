package lldp

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	promptPattern = regexp.MustCompile(`(?im)^(?:[A-Za-z0-9/]+:)?([A-Za-z0-9_.-]+)[>#]\s*show\s+(?:switch\s+)?lldp`)
	junosPrompt   = regexp.MustCompile(`(?im)^[A-Za-z0-9_.-]+@([A-Za-z0-9_.-]+)>\s*show\s+lldp`)
	fieldPattern  = regexp.MustCompile(`(?i)^\s*([A-Za-z][A-Za-z0-9 /_-]*?)\s*:\s*(.*?)\s*$`)
)

func Parse(data []byte, format string) (Result, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "auto" {
		format = Detect(data)
	}
	switch format {
	case "openconfig", "openconfig-json", "json":
		return parseOpenConfig(data)
	case "juniper-xml", "junos-xml", "xml":
		return parseJunosXML(data)
	case "cisco", "nexus", "nxos", "ios", "iosxe":
		return parseCisco(data)
	case "juniper", "junos":
		return parseJunos(data)
	case "arista", "eos":
		return parseArista(data)
	default:
		return Result{}, fmt.Errorf("unknown LLDP format %q; use auto, openconfig, juniper-xml, cisco, juniper, or arista", format)
	}
}

func Detect(data []byte) string {
	trimmed := strings.TrimSpace(string(data))
	if strings.Contains(trimmed, "<lldp-neighbors-information") || strings.Contains(trimmed, "<lldp-neighbor-information>") {
		return "juniper-xml"
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "openconfig"
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "junos") || strings.Contains(lower, "lldp neighbor information") || junosPrompt.MatchString(trimmed):
		return "juniper"
	case strings.Contains(lower, "arista") || strings.Contains(lower, "detected 1 lldp neighbor"):
		return "arista"
	default:
		return "cisco"
	}
}
