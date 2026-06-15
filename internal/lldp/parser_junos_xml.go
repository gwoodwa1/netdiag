package lldp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

func parseJunosXML(data []byte) (Result, error) {
	text := string(data)
	start := strings.Index(text, "<rpc-reply")
	if start < 0 {
		start = strings.Index(text, "<lldp-neighbors-information")
	}
	if start < 0 {
		return Result{}, fmt.Errorf("Junos XML does not contain an LLDP rpc reply")
	}
	decoder := xml.NewDecoder(strings.NewReader(text[start:]))
	result := Result{}
	if match := junosPrompt.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = match[1]
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("parse Junos LLDP XML: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "lldp-neighbor-information" {
			continue
		}
		var raw junosXMLNeighbor
		if err := decoder.DecodeElement(&raw, &start); err != nil {
			return Result{}, fmt.Errorf("parse Junos LLDP neighbor XML: %w", err)
		}
		result.Neighbors = append(result.Neighbors, Neighbor{
			LocalPort: useful(raw.LocalInterface), ChassisID: useful(raw.ChassisID),
			PortID: useful(raw.PortID), PortDescription: useful(raw.PortDescription),
			SystemName: useful(raw.SystemName), SystemDescription: useful(raw.SystemDescription),
			ManagementAddress: useful(raw.ManagementAddress), Capabilities: useful(raw.Capabilities),
		})
	}
	result.Neighbors = completeNeighbors(result.Neighbors)
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no LLDP neighbors found in Junos XML")
	}
	return result, nil
}

type junosXMLNeighbor struct {
	LocalInterface    string `xml:"lldp-local-interface"`
	ChassisID         string `xml:"lldp-remote-chassis-id"`
	PortID            string `xml:"lldp-remote-port-id"`
	PortDescription   string `xml:"lldp-remote-port-description"`
	SystemName        string `xml:"lldp-remote-system-name"`
	SystemDescription string `xml:"lldp-system-description>lldp-remote-system-description"`
	Capabilities      string `xml:"lldp-remote-system-capabilities-enabled"`
	ManagementAddress string `xml:"lldp-remote-management-address"`
}
