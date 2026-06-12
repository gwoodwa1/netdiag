package lldp

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/gwoodwa1/netdiag/internal/spec"
)

type Neighbor struct {
	LocalPort         string
	ChassisID         string
	PortID            string
	PortDescription   string
	SystemName        string
	SystemDescription string
	ManagementAddress string
	Capabilities      string
}

type Result struct {
	LocalNode string
	Neighbors []Neighbor
}

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
		return parseText(data, "cisco")
	case "juniper", "junos":
		return parseJunos(data)
	case "arista", "eos":
		return parseText(data, "arista")
	default:
		return Result{}, fmt.Errorf("unknown LLDP format %q; use auto, openconfig, juniper-xml, cisco, juniper, or arista", format)
	}
}

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
		if err == io.EOF {
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
			LocalPort:         useful(raw.LocalInterface),
			ChassisID:         useful(raw.ChassisID),
			PortID:            useful(raw.PortID),
			PortDescription:   useful(raw.PortDescription),
			SystemName:        useful(raw.SystemName),
			SystemDescription: useful(raw.SystemDescription),
			ManagementAddress: useful(raw.ManagementAddress),
			Capabilities:      useful(raw.Capabilities),
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
		result = append(result, Neighbor{
			LocalPort:    fields[0],
			ChassisID:    fields[2],
			PortID:       fields[3],
			SystemName:   strings.Join(fields[4:], " "),
			Capabilities: "",
		})
	}
	return result
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

func ToDocument(result Result, localNode string) (*spec.Document, error) {
	if strings.TrimSpace(localNode) == "" {
		localNode = result.LocalNode
	}
	if strings.TrimSpace(localNode) == "" {
		return nil, fmt.Errorf("local device name is unavailable; provide --local")
	}
	result.LocalNode = localNode
	return ToDocumentSet([]Result{result})
}

// ToDocumentSet merges LLDP observations from multiple local devices into one
// topology and deduplicates links observed from both ends.
func ToDocumentSet(results []Result) (*spec.Document, error) {
	doc := &spec.Document{
		Version: 1,
		Diagram: spec.Diagram{Title: "LLDP discovered topology", Layout: "rows"},
		Nodes:   make(map[string]spec.Node),
	}
	seenLinks := make(map[string]bool)
	localCount := 0
	for _, result := range results {
		localNode := strings.TrimSpace(result.LocalNode)
		if localNode == "" {
			return nil, fmt.Errorf("local device name is unavailable; provide --local or use a descriptive filename")
		}
		localCount++
		localID := nodeID(localNode)
		mergeNode(doc.Nodes, localID, spec.Node{
			Label: localNode, Role: "switch",
			Metadata: map[string]interface{}{"discovery": "lldp", "local": true},
		})
		for _, neighbor := range result.Neighbors {
			identity := firstUseful(neighbor.SystemName, neighbor.ChassisID, neighbor.ManagementAddress)
			if identity == "" || neighbor.LocalPort == "" || neighbor.PortID == "" {
				continue
			}
			remoteID := nodeID(identity)
			metadata := map[string]interface{}{"discovery": "lldp"}
			addMetadata(metadata, "chassis_id", neighbor.ChassisID)
			addMetadata(metadata, "management_address", neighbor.ManagementAddress)
			addMetadata(metadata, "system_description", neighbor.SystemDescription)
			addMetadata(metadata, "capabilities", neighbor.Capabilities)
			mergeNode(doc.Nodes, remoteID, spec.Node{Label: identity, Role: inferRole(neighbor), Metadata: metadata})

			link := spec.Link{
				From:     spec.LinkEndpoint{Node: localID, Port: neighbor.LocalPort},
				To:       spec.LinkEndpoint{Node: remoteID, Port: neighbor.PortID},
				Label:    neighbor.PortDescription,
				Protocol: "lldp",
			}
			key := linkKey(link)
			if !seenLinks[key] {
				seenLinks[key] = true
				doc.Links = append(doc.Links, link)
			}
		}
	}
	if len(doc.Links) == 0 {
		return nil, fmt.Errorf("no complete LLDP neighbors found")
	}
	if localCount == 1 {
		for _, result := range results {
			doc.Diagram.Title = "LLDP topology: " + result.LocalNode
		}
	}
	sort.Slice(doc.Links, func(i, j int) bool {
		left, right := linkKey(doc.Links[i]), linkKey(doc.Links[j])
		return left < right
	})
	return doc, nil
}

func mergeNode(nodes map[string]spec.Node, id string, incoming spec.Node) {
	existing, ok := nodes[id]
	if !ok {
		nodes[id] = incoming
		return
	}
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]interface{})
	}
	for key, value := range incoming.Metadata {
		existing.Metadata[key] = value
	}
	if existing.Label == "" {
		existing.Label = incoming.Label
	}
	if incoming.Role == "router" || existing.Role == "" || existing.Role == "device" {
		existing.Role = incoming.Role
	}
	nodes[id] = existing
}

func linkKey(link spec.Link) string {
	left := link.From.Node + "\x00" + strings.ToLower(link.From.Port)
	right := link.To.Node + "\x00" + strings.ToLower(link.To.Port)
	if right < left {
		left, right = right, left
	}
	return left + "\x01" + right
}

func parseText(data []byte, vendor string) (Result, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	result := Result{}
	if match := promptPattern.FindStringSubmatch(text); len(match) > 0 {
		result.LocalNode = match[1]
	}
	if vendor == "cisco" {
		result.Neighbors = append(result.Neighbors, parseCiscoSummary(text)...)
		result.Neighbors = append(result.Neighbors, parseCiscoIOSXRSummary(text)...)
	}
	var current *Neighbor
	var aristaLocal string
	flush := func() {
		if current != nil {
			result.Neighbors = append(result.Neighbors, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if vendor == "arista" && strings.HasPrefix(lower, "interface ") && strings.Contains(lower, " lldp neighbor") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				aristaLocal = fields[1]
			}
			continue
		}
		match := fieldPattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		key, value := normalizeKey(match[1]), useful(match[2])
		if key == "chassisid" {
			pendingLocal := ""
			if current != nil && current.ChassisID == "" && current.PortID == "" {
				pendingLocal = current.LocalPort
				current = nil
			}
			flush()
			current = &Neighbor{ChassisID: value, LocalPort: firstUseful(aristaLocal, pendingLocal)}
			continue
		}
		if current == nil {
			current = &Neighbor{LocalPort: aristaLocal}
		}
		switch key {
		case "localportid", "localinterface", "localintf":
			current.LocalPort = value
		case "portid":
			current.PortID = value
		case "portdescription":
			current.PortDescription = value
		case "systemname":
			current.SystemName = value
		case "systemdescription":
			current.SystemDescription = value
		case "managementaddress", "managementaddressipv4", "managementaddressipv6":
			if current.ManagementAddress == "" {
				current.ManagementAddress = value
			}
		case "enabledcapabilities", "systemcapabilities":
			if current.Capabilities == "" {
				current.Capabilities = value
			}
		}
	}
	flush()
	result.Neighbors = completeNeighbors(result.Neighbors)
	if len(result.Neighbors) == 0 {
		return Result{}, fmt.Errorf("no LLDP neighbors found in %s text", vendor)
	}
	return result, nil
}

func parseCiscoSummary(text string) []Neighbor {
	lines := strings.Split(text, "\n")
	inTable := false
	var result []Neighbor
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "INDEX PORT DEVICE ID PORT ID NAME CAPABILITIES TTL") {
			inTable = true
			continue
		}
		if !inTable || trimmed == "" || strings.HasPrefix(trimmed, "-") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 7 {
			continue
		}
		if _, ok := parsePositiveInteger(fields[0]); !ok {
			continue
		}
		ttlIndex := len(fields) - 1
		if _, ok := parsePositiveInteger(fields[ttlIndex]); !ok {
			continue
		}
		result = append(result, Neighbor{
			LocalPort:    fields[1],
			ChassisID:    fields[2],
			PortID:       fields[3],
			SystemName:   fields[4],
			Capabilities: strings.Join(fields[5:ttlIndex], " "),
		})
	}
	return result
}

func parseCiscoIOSXRSummary(text string) []Neighbor {
	lines := strings.Split(text, "\n")
	inTable := false
	var result []Neighbor
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "DEVICE ID") && strings.Contains(upper, "LOCAL INTF") && strings.Contains(upper, "HOLD-TIME") && strings.Contains(upper, "PORT ID") {
			inTable = true
			continue
		}
		if !inTable || trimmed == "" || strings.HasPrefix(upper, "TOTAL ENTRIES") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 5 {
			continue
		}
		if _, ok := parsePositiveInteger(fields[2]); !ok {
			continue
		}
		result = append(result, Neighbor{
			LocalPort:    fields[1],
			ChassisID:    fields[len(fields)-1],
			PortID:       fields[len(fields)-1],
			SystemName:   fields[0],
			Capabilities: useful(strings.Join(fields[3:len(fields)-1], " ")),
		})
	}
	return result
}

func parsePositiveInteger(value string) (int, bool) {
	result := 0
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		result = result*10 + int(r-'0')
	}
	return result, true
}

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
		LocalPort:         localPort,
		ChassisID:         firstUseful(stringValue(state, "chassis-id"), stringValue(entry, "chassis-id")),
		PortID:            firstUseful(stringValue(state, "port-id"), stringValue(entry, "port-id"), stringValue(entry, "id")),
		PortDescription:   stringValue(state, "port-description"),
		SystemName:        stringValue(state, "system-name"),
		SystemDescription: stringValue(state, "system-description"),
		ManagementAddress: firstString(state["management-address"]),
		Capabilities:      firstString(state["system-capabilities"]),
	}
}

func completeNeighbors(neighbors []Neighbor) []Neighbor {
	result := make([]Neighbor, 0, len(neighbors))
	for _, neighbor := range neighbors {
		if neighbor.LocalPort != "" && neighbor.PortID != "" && firstUseful(neighbor.SystemName, neighbor.ChassisID, neighbor.ManagementAddress) != "" {
			result = append(result, neighbor)
		}
	}
	return result
}

func normalizeKey(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func useful(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "null", "not advertised", "n/a", "-":
		return ""
	default:
		return value
	}
}

func nodeID(value string) string {
	value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	var out strings.Builder
	dash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' {
			out.WriteRune(r)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func inferRole(neighbor Neighbor) string {
	capabilities := strings.FieldsFunc(strings.ToLower(neighbor.Capabilities), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == ';'
	})
	for _, capability := range capabilities {
		if capability == "r" || capability == "router" {
			return "router"
		}
	}
	for _, capability := range capabilities {
		if capability == "b" || capability == "bridge" {
			return "switch"
		}
	}
	value := strings.ToLower(neighbor.SystemDescription)
	switch {
	case strings.Contains(value, "router"):
		return "router"
	case strings.Contains(value, "bridge") || strings.Contains(value, "switch"):
		return "switch"
	case strings.Contains(value, "server") || strings.Contains(value, "station"):
		return "server"
	default:
		return "device"
	}
}

func addMetadata(metadata map[string]interface{}, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

func firstUseful(values ...string) string {
	for _, value := range values {
		if value = useful(value); value != "" {
			return value
		}
	}
	return ""
}

func stripPrefix(key string) string {
	if _, value, ok := strings.Cut(key, ":"); ok {
		return value
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
		for _, item := range typed {
			if result := firstString(item); result != "" {
				return result
			}
		}
	}
	return ""
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
