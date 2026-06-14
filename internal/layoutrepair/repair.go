package layoutrepair

import (
	"fmt"
	"runtime"
	"sort"
	"sync"

	"github.com/gwoodwa1/netdiag/internal/model"
	"github.com/gwoodwa1/netdiag/internal/spec"
	"github.com/gwoodwa1/netdiag/internal/svg"
	"gopkg.in/yaml.v3"
)

type Options struct {
	MaxRounds     int
	MaxCandidates int
}

type Change struct {
	Round       int    `json:"round"`
	Description string `json:"description"`
	Before      Score  `json:"before"`
	After       Score  `json:"after"`
}

type Score struct {
	Quality  int `json:"quality"`
	Penalty  int `json:"penalty"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Findings int `json:"findings"`
}

type Report struct {
	Before              Score    `json:"before"`
	After               Score    `json:"after"`
	CandidatesEvaluated int      `json:"candidates_evaluated"`
	Changes             []Change `json:"changes"`
}

type candidate struct {
	description string
	apply       func(*spec.Document)
}

// Improve searches a bounded set of authored YAML changes and accepts only
// candidates that strictly improve the deterministic native-layout report.
func Improve(input *spec.Document, options Options) (*spec.Document, Report, error) {
	if options.MaxRounds <= 0 {
		options.MaxRounds = 3
	}
	if options.MaxCandidates <= 0 {
		options.MaxCandidates = 80
	}
	current, err := cloneDocument(input)
	if err != nil {
		return nil, Report{}, err
	}
	currentReport, err := inspect(current)
	if err != nil {
		return nil, Report{}, err
	}
	result := Report{Before: score(currentReport), After: score(currentReport), Changes: []Change{}}
	for round := 1; round <= options.MaxRounds && result.CandidatesEvaluated < options.MaxCandidates; round++ {
		global := limitCandidates(globalCandidates(current), options.MaxCandidates-result.CandidatesEvaluated)
		bestDoc, bestReport, bestDescription, err := selectImprovement(current, currentReport, global)
		if err != nil {
			return nil, result, err
		}
		result.CandidatesEvaluated += len(global)
		if bestDescription == "" && result.CandidatesEvaluated < options.MaxCandidates {
			targeted := limitCandidates(targetedCandidates(current, currentReport), options.MaxCandidates-result.CandidatesEvaluated)
			bestDoc, bestReport, bestDescription, err = selectImprovement(current, currentReport, targeted)
			if err != nil {
				return nil, result, err
			}
			result.CandidatesEvaluated += len(targeted)
		}
		if bestDescription == "" {
			break
		}
		before := score(currentReport)
		current, currentReport = bestDoc, bestReport
		after := score(currentReport)
		result.Changes = append(result.Changes, Change{Round: round, Description: bestDescription, Before: before, After: after})
		result.After = after
	}
	return current, result, nil
}

func limitCandidates(candidates []candidate, limit int) []candidate {
	if len(candidates) > limit {
		return candidates[:limit]
	}
	return candidates
}

func selectImprovement(current *spec.Document, currentReport svg.InspectionReport, candidates []candidate) (*spec.Document, svg.InspectionReport, string, error) {
	bestDoc, bestReport, bestDescription := current, currentReport, ""
	evaluations, err := evaluateCandidates(current, candidates)
	if err != nil {
		return nil, svg.InspectionReport{}, "", err
	}
	for index, evaluation := range evaluations {
		if !evaluation.valid {
			continue
		}
		if better(score(evaluation.report), score(bestReport)) {
			bestDoc, bestReport, bestDescription = evaluation.doc, evaluation.report, candidates[index].description
		}
	}
	return bestDoc, bestReport, bestDescription, nil
}

type evaluation struct {
	doc    *spec.Document
	report svg.InspectionReport
	valid  bool
}

func evaluateCandidates(current *spec.Document, candidates []candidate) ([]evaluation, error) {
	result := make([]evaluation, len(candidates))
	jobs := make(chan int)
	workerCount := min(4, runtime.GOMAXPROCS(0), len(candidates))
	var wait sync.WaitGroup
	wait.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer wait.Done()
			for index := range jobs {
				trial, err := cloneDocument(current)
				if err != nil {
					continue
				}
				candidates[index].apply(trial)
				if err := spec.Prepare(trial); err != nil {
					continue
				}
				trialReport, err := inspect(trial)
				if err != nil {
					continue
				}
				result[index] = evaluation{doc: trial, report: trialReport, valid: true}
			}
		}()
	}
	for index := range candidates {
		jobs <- index
	}
	close(jobs)
	wait.Wait()
	return result, nil
}

func inspect(doc *spec.Document) (svg.InspectionReport, error) {
	diagram, err := model.Compile(doc)
	if err != nil {
		return svg.InspectionReport{}, err
	}
	return svg.Inspect(diagram)
}

func score(report svg.InspectionReport) Score {
	return Score{
		Quality: report.Score, Penalty: report.Summary.Errors*20 + report.Summary.Warnings*5 + report.Summary.Info,
		Errors: report.Summary.Errors, Warnings: report.Summary.Warnings, Findings: len(report.Findings),
	}
}

func better(left, right Score) bool {
	if left.Penalty != right.Penalty {
		return left.Penalty < right.Penalty
	}
	if left.Errors != right.Errors {
		return left.Errors < right.Errors
	}
	if left.Warnings != right.Warnings {
		return left.Warnings < right.Warnings
	}
	if left.Findings != right.Findings {
		return left.Findings < right.Findings
	}
	return left.Quality > right.Quality
}

func globalCandidates(doc *spec.Document) []candidate {
	var result []candidate
	for _, style := range []string{"orthogonal", "clean"} {
		if style == doc.Diagram.LinkStyle {
			continue
		}
		value := style
		result = append(result, candidate{
			description: "set diagram.link_style to " + value,
			apply:       func(trial *spec.Document) { trial.Diagram.LinkStyle = value },
		})
	}
	for _, clearance := range []float64{48, 72, 96, 128} {
		if clearance == doc.Diagram.RouteClearance {
			continue
		}
		value := clearance
		result = append(result, candidate{
			description: fmt.Sprintf("set diagram.route_clearance to %.0f", value),
			apply:       func(trial *spec.Document) { trial.Diagram.RouteClearance = value },
		})
	}
	for _, clearance := range []float64{56, 72, 96, 128} {
		if clearance == doc.Diagram.EndpointClearance {
			continue
		}
		value := clearance
		result = append(result, candidate{
			description: fmt.Sprintf("set diagram.endpoint_clearance to %.0f", value),
			apply:       func(trial *spec.Document) { trial.Diagram.EndpointClearance = value },
		})
	}
	result = append(result, peerOrderCandidates(doc)...)
	return result
}

func peerOrderCandidates(doc *spec.Document) []candidate {
	byRole := make(map[string][]string)
	for id, node := range doc.Nodes {
		byRole[node.Role] = append(byRole[node.Role], id)
	}
	neighbors := make(map[string][]string)
	for _, link := range doc.Links {
		neighbors[link.From.Node] = append(neighbors[link.From.Node], link.To.Node)
		neighbors[link.To.Node] = append(neighbors[link.To.Node], link.From.Node)
	}
	var roles []string
	for role, ids := range byRole {
		if len(ids) > 1 {
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)
	var result []candidate
	for _, role := range roles {
		ids := append([]string(nil), byRole[role]...)
		sort.Strings(ids)
		rank := make(map[string]int, len(doc.Nodes))
		allIDs := make([]string, 0, len(doc.Nodes))
		for id := range doc.Nodes {
			allIDs = append(allIDs, id)
		}
		sort.Strings(allIDs)
		for index, id := range allIDs {
			rank[id] = index
		}
		sort.SliceStable(ids, func(i, j int) bool {
			left, right := averagePeerRank(neighbors[ids[i]], rank), averagePeerRank(neighbors[ids[j]], rank)
			if left == right {
				return ids[i] < ids[j]
			}
			return left < right
		})
		if roleOrderMatches(doc, ids) {
			continue
		}
		ordered := append([]string(nil), ids...)
		result = append(result, candidate{
			description: fmt.Sprintf("order %s nodes by connected peers", role),
			apply: func(trial *spec.Document) {
				for index, id := range ordered {
					node := trial.Nodes[id]
					node.Order = index + 1
					trial.Nodes[id] = node
				}
			},
		})
	}
	return result
}

func averagePeerRank(peers []string, rank map[string]int) float64 {
	if len(peers) == 0 {
		return float64(len(rank))
	}
	total := 0
	for _, peer := range peers {
		total += rank[peer]
	}
	return float64(total) / float64(len(peers))
}

func roleOrderMatches(doc *spec.Document, ordered []string) bool {
	current := append([]string(nil), ordered...)
	sort.SliceStable(current, func(i, j int) bool {
		left, right := doc.Nodes[current[i]].Order, doc.Nodes[current[j]].Order
		if left == 0 {
			left = int(^uint(0) >> 1)
		}
		if right == 0 {
			right = int(^uint(0) >> 1)
		}
		if left == right {
			return current[i] < current[j]
		}
		return left < right
	})
	for index := range ordered {
		if ordered[index] != current[index] {
			return false
		}
	}
	return true
}

func targetedCandidates(doc *spec.Document, report svg.InspectionReport) []candidate {
	linkIDs := problemLinkIDs(report, len(doc.Links))
	var result []candidate
	sidePairs := [][2]string{{"top", "bottom"}, {"bottom", "top"}, {"left", "right"}, {"right", "left"}}
	for _, linkID := range linkIDs {
		index := linkID - 1
		for _, pair := range sidePairs {
			fromSide, toSide := pair[0], pair[1]
			result = append(result, candidate{
				description: fmt.Sprintf("route link %d (%s -> %s) from %s to %s", linkID, doc.Links[index].From.Node, doc.Links[index].To.Node, fromSide, toSide),
				apply: func(trial *spec.Document) {
					setEndpointRoute(&trial.Links[index].From, fromSide, nil, 140)
					setEndpointRoute(&trial.Links[index].To, toSide, nil, 140)
				},
			})
		}
		for _, positionPair := range [][2]float64{{0.25, 0.75}, {0.75, 0.25}} {
			fromPosition, toPosition := positionPair[0], positionPair[1]
			result = append(result, candidate{
				description: fmt.Sprintf("separate endpoint positions for link %d (%s -> %s)", linkID, doc.Links[index].From.Node, doc.Links[index].To.Node),
				apply: func(trial *spec.Document) {
					fromSide := defaultSide(trial.Links[index].From.Side, "top")
					toSide := defaultSide(trial.Links[index].To.Side, "bottom")
					setEndpointRoute(&trial.Links[index].From, fromSide, &fromPosition, 180)
					setEndpointRoute(&trial.Links[index].To, toSide, &toPosition, 180)
				},
			})
		}
		for _, rotation := range []int{0, 90} {
			value := rotation
			result = append(result, candidate{
				description: fmt.Sprintf("rotate interface labels on link %d to %d degrees", linkID, value),
				apply: func(trial *spec.Document) {
					trial.Links[index].From.LabelRotation = value
					trial.Links[index].To.LabelRotation = value
				},
			})
		}
	}
	return result
}

func problemLinkIDs(report svg.InspectionReport, linkCount int) []int {
	type priority struct {
		linkID int
		errors int
		total  int
	}
	byLink := make(map[int]*priority)
	for _, finding := range report.Findings {
		for _, linkID := range finding.Links {
			if linkID > 0 && linkID <= linkCount {
				item := byLink[linkID]
				if item == nil {
					item = &priority{linkID: linkID}
					byLink[linkID] = item
				}
				item.total++
				if finding.Severity == svg.InspectionError {
					item.errors++
				}
			}
		}
	}
	items := make([]priority, 0, len(byLink))
	for _, item := range byLink {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].errors != items[j].errors {
			return items[i].errors > items[j].errors
		}
		if items[i].total != items[j].total {
			return items[i].total > items[j].total
		}
		return items[i].linkID < items[j].linkID
	})
	if len(items) > 8 {
		items = items[:8]
	}
	result := make([]int, len(items))
	for index, item := range items {
		result[index] = item.linkID
	}
	return result
}

func setEndpointRoute(endpoint *spec.LinkEndpoint, side string, position *float64, stub float64) {
	endpoint.Side = side
	endpoint.Position = position
	endpoint.Stub = stub
}

func defaultSide(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func cloneDocument(input *spec.Document) (*spec.Document, error) {
	data, err := yaml.Marshal(input)
	if err != nil {
		return nil, err
	}
	var result spec.Document
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
