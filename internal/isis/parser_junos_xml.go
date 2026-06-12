package isis

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var junosPromptPattern = regexp.MustCompile(`(?im)^[A-Za-z0-9_.-]+@([A-Za-z0-9_.-]+)>\s*show\s+isis`)

func parseJunosXML(data []byte) (Result, error) {
	text := string(data)
	start := strings.Index(text, "<rpc-reply")
	if start < 0 {
		start = strings.Index(text, "<isis-adjacency-information")
	}
	if start < 0 {
		return Result{}, fmt.Errorf("Junos XML does not contain IS-IS adjacency information")
	}
	result := Result{}
	if match := junosPromptPattern.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = match[1]
	}
	decoder := xml.NewDecoder(strings.NewReader(text[start:]))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("parse Junos IS-IS XML: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "isis-adjacency" {
			continue
		}
		var raw junosXMLAdjacency
		if err := decoder.DecodeElement(&raw, &start); err != nil {
			return Result{}, fmt.Errorf("parse Junos IS-IS adjacency XML: %w", err)
		}
		holdtime, _ := strconv.Atoi(strings.TrimSpace(raw.Holdtime))
		result.Neighbors = append(result.Neighbors, Neighbor{
			SystemID:  strings.TrimSpace(raw.SystemName),
			Interface: strings.TrimSpace(raw.InterfaceName),
			State:     normalizeState(raw.State),
			Holdtime:  holdtime,
			Type:      normalizeLevel(raw.Level),
		})
	}
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no IS-IS adjacencies found in Junos XML")
	}
	return result, nil
}

type junosXMLAdjacency struct {
	InterfaceName string `xml:"interface-name"`
	SystemName    string `xml:"system-name"`
	Level         string `xml:"level"`
	State         string `xml:"adjacency-state"`
	Holdtime      string `xml:"holdtime"`
}
