package isis

type Neighbor struct {
	SystemID  string
	Interface string
	SNPA      string
	State     string
	Holdtime  int
	Type      string
	IETFNSF   string
	Instance  string
}

type Result struct {
	LocalNode string
	Neighbors []Neighbor
}

type Report struct {
	Devices            int
	Observations       int
	Nodes              int
	Links              int
	MergedObservations int
}
