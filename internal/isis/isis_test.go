package isis

import "testing"

func TestParseIOSXRAndConvert(t *testing.T) {
	input := `+++ R2_xr: executing command 'show isis neighbors' +++
show isis neighbors
Wed Apr 17 16:21:30.075 UTC

IS-IS test neighbors:
System Id      Interface        SNPA           State Holdtime Type IETF-NSF
R1_xe          Gi0/0/0/0.115    fa16.3eff.4f49 Up    24       L1L2 Capable
R3_nx          Gi0/0/0/1.115    5e00.40ff.0209 Up    25       L1L2 Capable

Total neighbor count: 2`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.LocalNode != "R2_xr" || len(result.Neighbors) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	first := result.Neighbors[0]
	if first.SystemID != "R1_xe" || first.Interface != "Gi0/0/0/0.115" || first.Instance != "test" || first.Type != "L1L2" || first.Holdtime != 24 {
		t.Fatalf("unexpected neighbor: %+v", first)
	}
	doc, err := ToDocumentSet([]Result{result})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 3 || len(doc.Links) != 2 || doc.Links[0].Protocol != "isis" {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
}

func TestParseOpenConfigAndConvert(t *testing.T) {
	input := `{
  "openconfig-network-instance:network-instances": {
    "network-instance": [{
      "name": "default",
      "protocols": {
        "protocol": [{
          "identifier": "ISIS",
          "name": "CORE",
          "isis": {
            "interfaces": {
              "interface": [{
                "interface-id": "Ethernet1",
                "levels": {
                  "level": [{
                    "level-number": 2,
                    "adjacencies": {
                      "adjacency": [{
                        "system-id": "0000.0000.0002",
                        "state": {
                          "system-id": "0000.0000.0002",
                          "adjacency-state": "UP",
                          "remaining-hold-time": 27
                        }
                      }]
                    }
                  }]
                }
              }]
            }
          }
        }]
      }
    }]
  }
}`
	result, err := Parse([]byte(input), "auto")
	if err != nil {
		t.Fatal(err)
	}
	result.LocalNode = "router-01"
	if len(result.Neighbors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	got := result.Neighbors[0]
	if got.SystemID != "0000.0000.0002" || got.Interface != "Ethernet1" || got.Type != "L2" || got.State != "up" || got.Holdtime != 27 || got.Instance != "CORE" {
		t.Fatalf("unexpected adjacency: %+v", got)
	}
	doc, err := ToDocumentSet([]Result{result})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 2 || len(doc.Links) != 1 || doc.Links[0].To.Port != "isis-adjacency" {
		t.Fatalf("unexpected diagram: %+v", doc)
	}
}
