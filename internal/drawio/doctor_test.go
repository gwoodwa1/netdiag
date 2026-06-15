package drawio

import "testing"

func TestDoctorReportsRoundTripSafety(t *testing.T) {
	safe := []byte(`<mxfile><diagram><mxGraphModel><root>
		<mxCell id="0"></mxCell>
		<mxCell netdiag-id="core-a" netdiag-kind="node" vertex="1"><mxGeometry x="1" y="2" width="3" height="4"></mxGeometry></mxCell>
		<mxCell netdiag-id="core-link" netdiag-kind="link" edge="1"></mxCell>
		<mxCell style="text;" vertex="1"></mxCell>
	</root></mxGraphModel></diagram></mxfile>`)
	report, err := Doctor(safe)
	if err != nil {
		t.Fatal(err)
	}
	if !report.RoundTripSafe || report.Managed.Nodes != 1 || report.Managed.Links != 1 || report.Unmanaged.Annotations != 1 {
		t.Fatalf("unexpected safe report: %+v", report)
	}

	unsafe := []byte(`<mxfile><diagram><mxGraphModel><root>
		<mxCell netdiag-id="broken" vertex="1"></mxCell>
	</root></mxGraphModel></diagram></mxfile>`)
	report, err = Doctor(unsafe)
	if err != nil {
		t.Fatal(err)
	}
	if report.RoundTripSafe || len(report.Problems) == 0 {
		t.Fatalf("unexpected unsafe report: %+v", report)
	}
}
