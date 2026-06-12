package lldp

import "github.com/gwoodwa1/netdiag/internal/spec"

// Report summarizes the quality and merge effect of an LLDP discovery run.
type Report struct {
	Devices            int `json:"devices"`
	Observations       int `json:"observations"`
	Nodes              int `json:"nodes"`
	Links              int `json:"links"`
	MergedObservations int `json:"merged_observations"`
}

func BuildReport(results []Result, doc *spec.Document) Report {
	report := Report{Devices: len(results)}
	for _, result := range results {
		report.Observations += len(result.Neighbors)
	}
	if doc != nil {
		report.Nodes = len(doc.Nodes)
		report.Links = len(doc.Links)
	}
	if report.Observations > report.Links {
		report.MergedObservations = report.Observations - report.Links
	}
	return report
}
