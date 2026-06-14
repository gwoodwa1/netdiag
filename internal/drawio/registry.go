package drawio

type shapeStyle struct {
	Shape  string
	Fill   string
	Stroke string
}

var roleShapes = map[string]shapeStyle{
	"router":              {Shape: "mxgraph.cisco19.router", Fill: "#dae8fc", Stroke: "#6c8ebf"},
	"edge-router":         {Shape: "mxgraph.cisco19.router", Fill: "#fff2cc", Stroke: "#d6b656"},
	"core-router":         {Shape: "mxgraph.cisco19.router", Fill: "#e1d5e7", Stroke: "#9673a6"},
	"route-reflector":     {Shape: "mxgraph.cisco19.router", Fill: "#e1d5e7", Stroke: "#9673a6"},
	"rr-client":           {Shape: "mxgraph.cisco19.router", Fill: "#dae8fc", Stroke: "#6c8ebf"},
	"external-peer":       {Shape: "mxgraph.cisco19.router", Fill: "#fff2cc", Stroke: "#d6b656"},
	"isis-level-1":        {Shape: "mxgraph.cisco19.router", Fill: "#d5e8d4", Stroke: "#82b366"},
	"isis-level-2":        {Shape: "mxgraph.cisco19.router", Fill: "#e1d5e7", Stroke: "#9673a6"},
	"switch":              {Shape: "mxgraph.cisco19.layer_3_switch", Fill: "#d5e8d4", Stroke: "#82b366"},
	"core-switch":         {Shape: "mxgraph.cisco19.layer_3_switch", Fill: "#e1d5e7", Stroke: "#9673a6"},
	"distribution-switch": {Shape: "mxgraph.cisco19.layer_3_switch", Fill: "#d5e8d4", Stroke: "#82b366"},
	"access-switch":       {Shape: "mxgraph.cisco19.layer_2_switch", Fill: "#d5e8d4", Stroke: "#82b366"},
	"metro-switch":        {Shape: "mxgraph.cisco19.layer_2_switch", Fill: "#d5e8d4", Stroke: "#82b366"},
	"leaf":                {Shape: "mxgraph.cisco19.layer_3_switch", Fill: "#d5e8d4", Stroke: "#82b366"},
	"spine":               {Shape: "mxgraph.cisco19.layer_3_switch", Fill: "#e1d5e7", Stroke: "#9673a6"},
	"firewall":            {Shape: "mxgraph.cisco19.firewall", Fill: "#f8cecc", Stroke: "#b85450"},
	"server":              {Shape: "mxgraph.cisco19.server", Fill: "#f5f5f5", Stroke: "#666666"},
	"endpoint":            {Shape: "mxgraph.cisco19.server", Fill: "#f5f5f5", Stroke: "#666666"},
	"wireless":            {Shape: "mxgraph.cisco19.wireless_router", Fill: "#d5e8d4", Stroke: "#82b366"},
	"internet":            {Shape: "cloud", Fill: "#dae8fc", Stroke: "#6c8ebf"},
	"public-cloud":        {Shape: "cloud", Fill: "#dae8fc", Stroke: "#6c8ebf"},
	"wan-cloud":           {Shape: "cloud", Fill: "#dae8fc", Stroke: "#6c8ebf"},
}

func styleForRole(role string) shapeStyle {
	if style, ok := roleShapes[role]; ok {
		return style
	}
	return shapeStyle{Shape: "rectangle", Fill: "#ffffff", Stroke: "#64748b"}
}
