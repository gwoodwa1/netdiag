package isis

import (
	"fmt"
	"strings"
)

func Parse(data []byte, format string) (Result, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "auto" {
		format = Detect(data)
	}
	switch format {
	case "iosxr", "cisco", "cisco-iosxr":
		return ParseIOSXR(data)
	case "juniper-xml", "junos-xml", "xml":
		return parseJunosXML(data)
	case "openconfig", "openconfig-json", "json":
		return parseOpenConfig(data)
	default:
		return Result{}, fmt.Errorf("unknown IS-IS format %q; use auto, iosxr, juniper-xml, or openconfig", format)
	}
}

func Detect(data []byte) string {
	trimmed := strings.TrimSpace(string(data))
	if strings.Contains(trimmed, "<isis-adjacency-information") || strings.Contains(trimmed, "<isis-adjacency>") {
		return "juniper-xml"
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "openconfig"
	}
	return "iosxr"
}
