// Package constraint defines solver-neutral layout intent.
package constraint

// Strength tells a layout engine whether it may relax a constraint.
type Strength uint8

const (
	Weak Strength = iota + 1
	Medium
	Strong
	Required
)

type Axis string

const (
	Horizontal Axis = "horizontal"
	Vertical   Axis = "vertical"
)

type Endpoint string

const (
	Source Endpoint = "source"
	Target Endpoint = "target"
)

// Rank keeps a set of nodes on the same layout layer. Order controls the
// sequence of ranks, not positions within a rank.
type Rank struct {
	ID       string
	Order    int
	Nodes    []string
	Strength Strength
}

// OrderedItem gives a node an authored position within a scope. Gaps are
// meaningful, so engines can insert unconstrained nodes between items.
type OrderedItem struct {
	NodeID   string
	Position int
}

type Order struct {
	Scope    string
	Axis     Axis
	Items    []OrderedItem
	Strength Strength
}

// Containment records either a node in a group or a nested group.
type Containment struct {
	ChildID  string
	ParentID string
	IsGroup  bool
	Strength Strength
}

// Port fixes an edge endpoint to a node side and, optionally, a normalized
// position on that side.
type Port struct {
	LinkID   string
	Endpoint Endpoint
	NodeID   string
	PortID   string
	Side     string
	Position *float64
	Strength Strength
}

// Scalar represents an optional coordinate or dimension without sentinel
// values; zero remains a valid coordinate.
type Scalar struct {
	Value float64
	Set   bool
}

type Geometry struct {
	TargetID string
	IsGroup  bool
	X        Scalar
	Y        Scalar
	Width    Scalar
	Height   Scalar
	Locked   bool
	Strength Strength
}

type Point struct {
	X float64
	Y float64
}

type Route struct {
	LinkID    string
	Waypoints []Point
	Locked    bool
	Strength  Strength
}

// Set is the complete layout intent presented to a layout engine.
type Set struct {
	Ranks       []Rank
	Orders      []Order
	Containment []Containment
	Ports       []Port
	Geometry    []Geometry
	Routes      []Route
}

// Clone returns an independently mutable copy of the set.
func (set Set) Clone() Set {
	result := Set{
		Ranks:       append([]Rank(nil), set.Ranks...),
		Orders:      append([]Order(nil), set.Orders...),
		Containment: append([]Containment(nil), set.Containment...),
		Ports:       append([]Port(nil), set.Ports...),
		Geometry:    append([]Geometry(nil), set.Geometry...),
		Routes:      append([]Route(nil), set.Routes...),
	}
	for index := range result.Ranks {
		result.Ranks[index].Nodes = append([]string(nil), result.Ranks[index].Nodes...)
	}
	for index := range result.Orders {
		result.Orders[index].Items = append([]OrderedItem(nil), result.Orders[index].Items...)
	}
	for index := range result.Routes {
		result.Routes[index].Waypoints = append([]Point(nil), result.Routes[index].Waypoints...)
	}
	return result
}

func (set Set) RankFor(nodeID string) (Rank, bool) {
	for _, rank := range set.Ranks {
		for _, id := range rank.Nodes {
			if id == nodeID {
				return rank, true
			}
		}
	}
	return Rank{}, false
}

func (set Set) OrderFor(scope, nodeID string) (int, bool) {
	for _, order := range set.Orders {
		if order.Scope != scope {
			continue
		}
		for _, item := range order.Items {
			if item.NodeID == nodeID {
				return item.Position, true
			}
		}
	}
	return 0, false
}

func (set Set) PortFor(linkID string, endpoint Endpoint) (Port, bool) {
	for _, port := range set.Ports {
		if port.LinkID == linkID && port.Endpoint == endpoint {
			return port, true
		}
	}
	return Port{}, false
}
