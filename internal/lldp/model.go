package lldp

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
