package lldp

import (
	"strings"
	"testing"
)

func TestParseCiscoNexusAndBuildDocument(t *testing.T) {
	input := `CONTOSO-DC1-TOR-02# show lldp neighbors detail
Chassis id: 689e.0baf.916a
Port id: Ethernet1/12/3
Local Port id: Eth1/47
Port Description: Uplink-to-Spine
System Name: CONTOSO-DC1-SPINE-01.contoso.com
System Description: Cisco Nexus Operating System (NX-OS)
Enabled Capabilities: B, R
Management Address IPV6: 2001:db8::1001
`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "CONTOSO-DC1-TOR-02" || len(result.Neighbors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	doc, err := ToDocument(result, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 2 || len(doc.Links) != 1 {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
	if got := doc.Nodes["contoso-dc1-spine-01.contoso.com"].Role; got != "router" {
		t.Fatalf("neighbor role = %q", got)
	}
	link := doc.Links[0]
	if link.From.Port != "Eth1/47" || link.To.Port != "Ethernet1/12/3" || link.Protocol != "lldp" {
		t.Fatalf("unexpected link: %+v", link)
	}
}

func TestParseCiscoIOSLocalIntf(t *testing.T) {
	input := `edge-01#show lldp neighbors detail
Chassis id: 0011.2233.4455
Port id: Gi0/1
Local Intf: Gi1/0/24
System Name: core-01
Enabled Capabilities: B,R
`
	result, err := Parse([]byte(input), "cisco")
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Neighbors[0].LocalPort; got != "Gi1/0/24" {
		t.Fatalf("local port = %q", got)
	}
}

func TestParseCiscoNFVISSummary(t *testing.T) {
	input := `nfvis# show switch lldp neighbors
SYSTEM
INDEX PORT DEVICE ID PORT ID NAME CAPABILITIES TTL
----------------------------------------------------------------------
1 gi1/1 00:1a:6c:81:f0:80 Gi1/0/31 SW-026 Bridge 93
2 gi1/6 2c:0b:e9:3c:89:00 Gi1/0/5 Switch Bridge 119
`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "nfvis" || len(result.Neighbors) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := result.Neighbors[1]; got.LocalPort != "gi1/6" || got.SystemName != "Switch" || got.PortID != "Gi1/0/5" {
		t.Fatalf("unexpected neighbor: %+v", got)
	}
	doc, err := ToDocument(result, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 3 || len(doc.Links) != 2 {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
}

func TestParseCiscoIOSXRSummary(t *testing.T) {
	input := `RP/0/RP0/CPU0:RouterX#show lldp neighbors HundredGigE0/0/0/0
Thu Feb 23 15:20:54.898 EST
Capability codes:
(R) Router, (B) Bridge

Device ID      Local Intf         Hold-time Capability Port ID
[DISABLED]     HundredGigE0/0/0/0 48        N/A        0123.4567.89ab
RemoteDevice01 HundredGigE0/0/0/0 120       B,R        0123.4567.89ab

Total entries displayed: 2
`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "RouterX" || len(result.Neighbors) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := result.Neighbors[1]; got.LocalPort != "HundredGigE0/0/0/0" || got.SystemName != "RemoteDevice01" || got.PortID != "0123.4567.89ab" {
		t.Fatalf("unexpected neighbor: %+v", got)
	}
	if result.Neighbors[1].ChassisID != "" {
		t.Fatalf("IOS XR summary must not treat Port ID as chassis ID: %+v", result.Neighbors[1])
	}
	doc, err := ToDocument(result, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 3 || len(doc.Links) != 2 {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
}

func TestParseJuniperShowCommand(t *testing.T) {
	input := `LLDP Neighbor Information:
Local interface: ge-0/0/0
Chassis ID: 00:11:22:33:44:55
Port ID: Ethernet1
Port description: leaf uplink
System name: leaf-01
System description: Arista Networks EOS
Enabled capabilities: Bridge, Router
`
	result, err := Parse([]byte(input), "juniper")
	if err != nil {
		t.Fatal(err)
	}
	got := result.Neighbors[0]
	if got.LocalPort != "ge-0/0/0" || got.SystemName != "leaf-01" || got.PortID != "Ethernet1" {
		t.Fatalf("unexpected neighbor: %+v", got)
	}
}

func TestParseJunosSummary(t *testing.T) {
	input := `user@switch> show lldp neighbors

Local Interface   Parent Interface   Chassis Id          Port info    System Name
xe-3/0/4.0        ae31.0             b0:c6:9a:63:80:40   xe-0/0/0.0   host.jnpr.net
xe-3/0/5.0        ae31.0             b0:c6:9a:63:80:40   xe-0/0/1.0   host.jnpr.net
xe-3/0/6.0        ae31.0             b0:c6:9a:63:80:40   xe-0/0/2.0   host.jnpr.net
xe-3/0/7.0        ae31.0             b0:c6:9a:63:80:40   xe-0/0/3.0   host.jnpr.net
xe-3/0/0.0        ae31.0             b0:c6:9a:63:80:40   xe-0/1/0.0   host.jnpr.net
xe-3/0/1.0        ae31.0             b0:c6:9a:63:80:40   xe-0/1/1.0   host.jnpr.net
xe-3/0/2.0        ae31.0             b0:c6:9a:63:80:40   xe-0/1/2.0   host.jnpr.net
xe-3/0/3.0        ae31.0             b0:c6:9a:63:80:40   xe-0/1/3.0   host.jnpr.net
`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "switch" || len(result.Neighbors) != 8 {
		t.Fatalf("unexpected result: %+v", result)
	}
	doc, err := ToDocument(result, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 2 || len(doc.Links) != 8 {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
}

func TestParseJunosDetailedSections(t *testing.T) {
	input := `user@host> show lldp neighbors detail
LLDP Neighbor Information:
Local Information:
Local Interface    : me0
Local Port ID      : 33

Neighbour Information:
Chassis ID         : 1a:22:23:dc:d9:50
Port ID            : 517
Port description   : ge-0/0/7.0
System name        : test
System Description : Juniper Networks ex3300

System capabilities
        Supported: Bridge Router
        Enabled  : Bridge Router

Management address
        Address Type      : IPv4(1)
        Address           : 10.221.0.111
        Interface Number  : 34

Organization Info
       Info     : VLAN ID (52), VLAN Name (vlan52)

LLDP Neighbor Information:
Local Information:
Local Interface    : ge-0/0/2
Local Port ID      : 512

Neighbour Information:
Chassis ID         : 1a:22:23:dc:d9:50
Port ID            : 511
Port description   : ge-0/0/2
System Description : Juniper Networks ex4300 Ethernet Switch

System capabilities
        Enabled  : Bridge Router

Management address
        Address           : 10.204.39.247
`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "host" || len(result.Neighbors) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	first := result.Neighbors[0]
	if first.LocalPort != "me0" || first.SystemName != "test" || first.PortID != "517" || first.ManagementAddress != "10.221.0.111" || first.Capabilities != "Bridge Router" {
		t.Fatalf("unexpected first neighbor: %+v", first)
	}
	second := result.Neighbors[1]
	if second.SystemName != "" || second.ManagementAddress != "10.204.39.247" {
		t.Fatalf("unexpected second neighbor: %+v", second)
	}
	doc, err := ToDocument(result, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 3 || len(doc.Links) != 2 {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
}

func TestParseJunosXML(t *testing.T) {
	input := `{master:0}
user@host1> show lldp neighbors detail | display xml
<rpc-reply xmlns:junos="http://xml.juniper.net/junos/20.2R0/junos">
  <lldp-neighbors-information xmlns="http://xml.juniper.net/junos/20.2R0/lldp-mtx" junos:style="detail">
    <lldp-neighbor-information>
      <lldp-local-interface>ge-0/0/27</lldp-local-interface>
      <lldp-remote-chassis-id>aa:bb:cc:dd:ee:ff</lldp-remote-chassis-id>
      <lldp-remote-port-id>aa:bb:c0:dd:ee:ff</lldp-remote-port-id>
      <lldp-remote-port-description>eth0</lldp-remote-port-description>
      <lldp-remote-system-name>host2</lldp-remote-system-name>
      <lldp-system-description>
        <lldp-remote-system-description>Some Information about the remote system OS/Versions</lldp-remote-system-description>
      </lldp-system-description>
      <lldp-remote-system-capabilities-enabled>Bridge WLAN Access Point Router </lldp-remote-system-capabilities-enabled>
      <lldp-remote-management-address>10.10.10.10</lldp-remote-management-address>
      <lldp-org-specific-tlv>
        <lldp-remote-subtype-lag-portid>0</lldp-remote-subtype-lag-portid>
      </lldp-org-specific-tlv>
    </lldp-neighbor-information>
  </lldp-neighbors-information>
  <cli><banner>{master:0}</banner></cli>
</rpc-reply>`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "host1" || len(result.Neighbors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	got := result.Neighbors[0]
	if got.LocalPort != "ge-0/0/27" || got.SystemName != "host2" || got.PortID != "aa:bb:c0:dd:ee:ff" || got.ManagementAddress != "10.10.10.10" || got.Capabilities != "Bridge WLAN Access Point Router" {
		t.Fatalf("unexpected neighbor: %+v", got)
	}
}

func TestParseAristaShowCommand(t *testing.T) {
	input := `Interface Ethernet4 detected 1 LLDP neighbors:
  Chassis ID: 001c.7300.0001
  Port ID: Ethernet1/1
  Port Description: core link
  System Name: core-01
  System Description: Cisco IOS XE Software
  Enabled Capabilities: Bridge, Router
`
	result, err := Parse([]byte(input), "arista")
	if err != nil {
		t.Fatal(err)
	}
	got := result.Neighbors[0]
	if got.LocalPort != "Ethernet4" || got.PortID != "Ethernet1/1" {
		t.Fatalf("unexpected neighbor: %+v", got)
	}
}

func TestParseOpenConfigJSON(t *testing.T) {
	input := `{
  "openconfig-lldp:lldp": {
    "interfaces": {
      "interface": [{
        "name": "Ethernet1",
        "neighbors": {
          "neighbor": [{
            "id": "peer",
            "state": {
              "chassis-id": "00:11:22:33:44:55",
              "port-id": "Ethernet9",
              "port-description": "fabric",
              "system-name": "spine-01",
              "system-description": "Arista EOS"
            }
          }]
        }
      }]
    }
  }
}`
	result, err := Parse([]byte(input), "openconfig")
	if err != nil {
		t.Fatal(err)
	}
	got := result.Neighbors[0]
	if got.LocalPort != "Ethernet1" || got.PortID != "Ethernet9" || got.SystemName != "spine-01" {
		t.Fatalf("unexpected neighbor: %+v", got)
	}
}

func TestParseOpenConfigJSONUsesNamedManagementAddress(t *testing.T) {
	input := `{
	  "openconfig-lldp:lldp": {
	    "interfaces": {"interface": [{
	      "name": "Ethernet1",
	      "neighbors": {"neighbor": [{
	        "id": "peer",
	        "state": {
	          "chassis-id": "00:11:22:33:44:55",
	          "port-id": "Ethernet9",
	          "system-name": "spine-01",
	          "management-address": {"address-type": "IPV4", "address": "192.0.2.10"},
	          "system-capabilities": [{"enabled": true, "name": "ROUTER"}]
	        }
	      }]}
	    }]}
	  }
	}`
	result, err := Parse([]byte(input), "openconfig")
	if err != nil {
		t.Fatal(err)
	}
	got := result.Neighbors[0]
	if got.ManagementAddress != "192.0.2.10" || got.Capabilities != "ROUTER" {
		t.Fatalf("unexpected structured OpenConfig values: %+v", got)
	}
}

func TestToDocumentRequiresLocalNameAndIgnoresIncompleteNeighbors(t *testing.T) {
	_, err := ToDocument(Result{Neighbors: []Neighbor{{LocalPort: "Eth1", PortID: "Eth2", SystemName: "peer"}}}, "")
	if err == nil || !strings.Contains(err.Error(), "--local") {
		t.Fatalf("expected local-name error, got %v", err)
	}
	_, err = ToDocument(Result{LocalNode: "local", Neighbors: []Neighbor{{SystemName: "peer"}}}, "")
	if err == nil || !strings.Contains(err.Error(), "no complete") {
		t.Fatalf("expected incomplete-neighbor error, got %v", err)
	}
}

func TestToDocumentSetMergesDevicesAndDeduplicatesReciprocalLinks(t *testing.T) {
	results := []Result{
		{LocalNode: "leaf-01", Neighbors: []Neighbor{{
			LocalPort: "Ethernet1", PortID: "Ethernet2", SystemName: "spine-01", Capabilities: "B,R",
		}}},
		{LocalNode: "spine-01", Neighbors: []Neighbor{{
			LocalPort: "Ethernet2", PortID: "Ethernet1", SystemName: "leaf-01", Capabilities: "B",
		}}},
	}
	doc, err := ToDocumentSet(results)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 2 || len(doc.Links) != 1 {
		t.Fatalf("unexpected merged diagram: %+v", doc)
	}
	for _, id := range []string{"leaf-01", "spine-01"} {
		if doc.Nodes[id].Metadata["local"] != true {
			t.Fatalf("node %q was not marked local: %+v", id, doc.Nodes[id])
		}
	}
	report := BuildReport(results, doc)
	if report.Devices != 2 || report.Observations != 2 || report.Nodes != 2 || report.Links != 1 || report.MergedObservations != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestToDocumentSetInfersNamedPEAndPRoles(t *testing.T) {
	results := []Result{
		{LocalNode: "NYC-PE1", Neighbors: []Neighbor{{
			LocalPort: "Hu0/0/0/0", PortID: "Hu0/0/0/0", SystemName: "CORE-A-P1", Capabilities: "R",
		}}},
		{LocalNode: "CORE-A-P1", Neighbors: []Neighbor{{
			LocalPort: "Hu0/0/0/0", PortID: "Hu0/0/0/0", SystemName: "NYC-PE1", Capabilities: "R",
		}}},
	}
	doc, err := ToDocumentSet(results)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Nodes["nyc-pe1"]; got.Role != "edge-router" || got.IconLabel != "PE" {
		t.Fatalf("unexpected PE role: %+v", got)
	}
	if got := doc.Nodes["core-a-p1"]; got.Role != "core-router" || got.IconLabel != "P" {
		t.Fatalf("unexpected P role: %+v", got)
	}
}
