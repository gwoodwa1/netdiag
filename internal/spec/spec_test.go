package spec

import (
	"bytes"
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseEndpoint(t *testing.T) {
	got, err := ParseEndpoint("leaf-01:Ethernet1/1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Node != "leaf-01" || got.Port != "Ethernet1/1" {
		t.Fatalf("unexpected endpoint: %#v", got)
	}
}

func TestDisplayPort(t *testing.T) {
	tests := map[string]string{
		"Ethernet0/0":              "Eth0/0",
		"GigabitEthernet0/1":       "Gi0/1",
		"TenGigabitEthernet1/1":    "Te1/1",
		"TenGig 0/0":               "Te0/0",
		"HundredGigE1/0/1":         "Hu1/0/1",
		"Port-Channel10":           "Po10",
		"Management0":              "Mgmt0",
		"CustomLongInterface12/34": "Custo12/34",
		"Eth0/0":                   "Eth0/0",
	}
	for input, want := range tests {
		if got := DisplayPort(input); got != want {
			t.Errorf("DisplayPort(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateUnknownNode(t *testing.T) {
	doc := &Document{
		Version: 1,
		Nodes: map[string]Node{
			"spine-01": {Role: "spine"},
		},
		Links: []Link{{From: LinkEndpoint{Node: "spine-01", Port: "Ethernet1/1"}, To: LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/1"}}},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected unknown node validation error")
	}
}

func TestValidateRejectsUnknownLayout(t *testing.T) {
	doc := &Document{
		Version: 1,
		Diagram: Diagram{Layout: "spiral"},
		Nodes: map[string]Node{
			"router-01": {Role: "router"},
		},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected unknown layout validation error")
	}
}

func TestValidateAcceptsSiteLayout(t *testing.T) {
	doc := &Document{
		Version: 1,
		Diagram: Diagram{Layout: "sites"},
		Nodes:   map[string]Node{"router-01": {Role: "router"}},
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("site layout rejected: %v", err)
	}
}

func TestValidateAcceptsPremiumThemeAndRejectsUnknownTheme(t *testing.T) {
	doc := &Document{Version: 1, Diagram: Diagram{Theme: "premium"}, Nodes: map[string]Node{"router": {Role: "router"}}}
	if err := Validate(doc); err != nil {
		t.Fatalf("premium theme should validate: %v", err)
	}
	doc.Diagram.Theme = "glossy"
	if err := Validate(doc); err == nil {
		t.Fatal("expected unknown theme validation error")
	}
}

func TestLinkTags(t *testing.T) {
	link := Link{
		Bundle:       "Port-Channel10",
		LACP:         true,
		MultiChassis: true,
		Trunk: &Trunk{
			Encapsulation: "dot1q",
			AllowedVLANs:  []string{"10", "20", "100-120"},
		},
	}
	got := link.Tags()
	want := []string{"MC-LAG", "LACP", "Port-Channel10", "TRUNK", "DOT1Q", "VLAN 10,20,100-120"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestValidateMCLAGRequiresMultipleSourceNodes(t *testing.T) {
	doc := &Document{
		Version: 1,
		Nodes: map[string]Node{
			"leaf-01": {Role: "leaf"},
			"app-01":  {Role: "server"},
		},
		Links: []Link{
			{From: LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/1"}, To: LinkEndpoint{Node: "app-01", Port: "Ethernet0/0"}, Bundle: "Port-Channel10", LACP: true, MultiChassis: true},
			{From: LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/2"}, To: LinkEndpoint{Node: "app-01", Port: "Ethernet0/1"}, Bundle: "Port-Channel10", LACP: true, MultiChassis: true},
		},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected MC-LAG source-node validation error")
	}
}

func TestValidateLACPRequiresBundle(t *testing.T) {
	doc := &Document{
		Version: 1,
		Nodes: map[string]Node{
			"leaf-01": {Role: "leaf"},
			"app-01":  {Role: "server"},
		},
		Links: []Link{{From: LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/1"}, To: LinkEndpoint{Node: "app-01", Port: "Ethernet0/0"}, LACP: true}},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected LACP bundle validation error")
	}
}

func TestValidateBundleSettingsMustMatch(t *testing.T) {
	doc := &Document{
		Version: 1,
		Nodes: map[string]Node{
			"leaf-01": {Role: "leaf"},
			"leaf-02": {Role: "leaf"},
			"app-01":  {Role: "server"},
		},
		Links: []Link{
			{From: LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/1"}, To: LinkEndpoint{Node: "app-01", Port: "Ethernet0/0"}, Bundle: "Port-Channel10", LACP: true},
			{From: LinkEndpoint{Node: "leaf-02", Port: "Ethernet1/1"}, To: LinkEndpoint{Node: "app-01", Port: "Ethernet0/1"}, Bundle: "Port-Channel10", LACP: false},
		},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected inconsistent bundle validation error")
	}
}

func TestUnmarshalLinkEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  LinkEndpoint
	}{
		{
			input: `"leaf-01:Ethernet1/1"`,
			want:  LinkEndpoint{Node: "leaf-01", Port: "Ethernet1/1"},
		},
		{
			input: `
node: server-01
port: eth0
side: top
label: access
address: 10.0.0.1/30`,
			want: LinkEndpoint{Node: "server-01", Port: "eth0", Side: "top", Label: "access", Address: "10.0.0.1/30"},
		},
	}
	for _, tc := range tests {
		var got LinkEndpoint
		if err := yaml.Unmarshal([]byte(tc.input), &got); err != nil {
			t.Fatalf("Unmarshal(%q) failed: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("Unmarshal(%q) = %+v, want %+v", tc.input, got, tc.want)
		}
	}
}

func TestValidateRejectsInvalidEndpointAddress(t *testing.T) {
	doc := &Document{
		Version: 1,
		Nodes:   map[string]Node{"a": {Role: "router"}, "b": {Role: "router"}},
		Links: []Link{{
			From: LinkEndpoint{Node: "a", Port: "Eth0/0", Address: "10.0.0.1"},
			To:   LinkEndpoint{Node: "b", Port: "Eth0/0"},
		}},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected CIDR validation error")
	}
}

func TestValidateRejectsLongIconLabel(t *testing.T) {
	doc := &Document{
		Version: 1,
		Nodes:   map[string]Node{"router": {Role: "router", IconLabel: "TOO-LONG"}},
	}
	if err := Validate(doc); err == nil {
		t.Fatal("expected icon label length validation error")
	}
}

func TestFormatIsDeterministic(t *testing.T) {
	doc := &Document{
		Version: 1,
		Diagram: Diagram{Layout: "rows"},
		Nodes: map[string]Node{
			"router-01": {Role: "router"},
		},
		Links: []Link{},
	}
	first, err := Format(doc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Format(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("formatted YAML changed across identical runs")
	}
}

func TestJSONSchemaIsValidJSON(t *testing.T) {
	result, err := JSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(result, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["$id"] != "https://netdiag.dev/schema/v1.json" {
		t.Fatalf("unexpected schema ID: %v", schema["$id"])
	}
}
