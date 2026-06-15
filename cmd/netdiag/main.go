package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/d2backend"
	"github.com/gwoodwa1/netdiag/internal/discoverylayout"
	"github.com/gwoodwa1/netdiag/internal/drawio"
	"github.com/gwoodwa1/netdiag/internal/export"
	"github.com/gwoodwa1/netdiag/internal/icons"
	"github.com/gwoodwa1/netdiag/internal/interactive"
	"github.com/gwoodwa1/netdiag/internal/isis"
	"github.com/gwoodwa1/netdiag/internal/layoutoverride"
	"github.com/gwoodwa1/netdiag/internal/layoutrepair"
	"github.com/gwoodwa1/netdiag/internal/lldp"
	"github.com/gwoodwa1/netdiag/internal/model"
	"github.com/gwoodwa1/netdiag/internal/planner"
	"github.com/gwoodwa1/netdiag/internal/source"
	"github.com/gwoodwa1/netdiag/internal/spec"
	"github.com/gwoodwa1/netdiag/internal/svg"
	"github.com/gwoodwa1/netdiag/internal/templates"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "render":
		render(os.Args[2:])
	case "validate":
		validate(os.Args[2:])
	case "expand":
		expand(os.Args[2:])
	case "schema":
		schema(os.Args[2:])
	case "fmt":
		format(os.Args[2:])
	case "templates":
		listTemplates(os.Args[2:])
	case "icons":
		listIcons(os.Args[2:])
	case "capabilities":
		capabilities(os.Args[2:])
	case "plan":
		plan(os.Args[2:])
	case "recommend":
		recommend(os.Args[2:])
	case "inspect":
		inspect(os.Args[2:])
	case "improve-layout":
		improveLayout(os.Args[2:])
	case "extract-overrides":
		extractOverrides(os.Args[2:])
	case "doctor":
		doctor(os.Args[2:])
	case "diff-layout":
		diffLayout(os.Args[2:])
	case "lldp":
		convertLLDP(os.Args[2:])
	case "discover":
		discover(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func extractOverrides(args []string) {
	flags := commandFlags("extract-overrides", "usage: netdiag extract-overrides <edited.drawio> --source <diagram.yaml> [-o diagram.layout.yaml] [--report]")
	sourcePath := flags.String("source", "", "source topology YAML file")
	var output string
	flags.StringVar(&output, "o", "", "output layout YAML path")
	flags.StringVar(&output, "output", "", "output layout YAML path")
	report := flags.Bool("report", false, "print extraction diagnostics")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 || *sourcePath == "" {
		flags.Usage()
		os.Exit(2)
	}
	input := flags.Arg(0)
	sourceDoc, err := loadDocument(*sourcePath)
	exitOnError(err)
	diagram, err := model.Compile(sourceDoc)
	exitOnError(err)
	data, err := os.ReadFile(input)
	exitOnError(err)
	var overrides *layoutoverride.Document
	var extractionReport drawio.ExtractionReport
	if *report {
		overrides, extractionReport, err = drawio.ExtractOverridesWithReport(data, diagram)
	} else {
		overrides, err = drawio.ExtractOverrides(data, diagram)
	}
	exitOnError(err)
	result, err := layoutoverride.Format(overrides)
	exitOnError(err)
	if output == "" {
		output = strings.TrimSuffix(input, filepath.Ext(input)) + ".layout.yaml"
	}
	exitOnError(os.WriteFile(output, result, 0o644))
	fmt.Printf("extracted layout overrides to %s\n", output)
	if *report {
		fmt.Println()
		fmt.Print(drawio.FormatExtractionReport(extractionReport))
	}
}

func render(args []string) {
	flags := commandFlags("render", "usage: netdiag render <diagram.yaml> [-o output] [--renderer native|d2|drawio] [--layout-overrides layout.yaml] [--layout-report] [--output-report text|json] [--icons directory] [--layout elk|dagre] [--report report.json]")
	var output, backend, iconDir string
	flags.StringVar(&output, "o", "", "output path")
	flags.StringVar(&output, "output", "", "output path")
	flags.StringVar(&backend, "backend", "", "renderer backend: native, d2, or drawio")
	flags.StringVar(&backend, "renderer", "", "renderer backend: native, d2, or drawio")
	reportPath := flags.String("report", "", "write renderer capability report JSON")
	layout := flags.String("layout", "", "D2 layout engine: elk or dagre")
	flags.StringVar(&iconDir, "icons", "", "custom SVG icon directory")
	flags.StringVar(&iconDir, "icon-dir", "", "custom SVG icon directory")
	layoutOverridesPath := flags.String("layout-overrides", "", "Draw.io layout override YAML")
	layoutReport := flags.Bool("layout-report", false, "print Draw.io topology reconciliation report")
	outputReport := flags.String("output-report", "", "layout report format: text or json")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	input := flags.Arg(0)
	if iconDir == "" {
		iconDir = os.Getenv("NETDIAG_ICONS")
	}

	doc, err := loadDocument(input)
	exitOnError(err)

	diag, err := model.Compile(doc)
	exitOnError(err)
	if backend == "" {
		backend = diag.Theme.Renderer
	}
	if backend == "" {
		backend = planner.Recommend(diag)
	}
	renderPlan, err := planner.Build(diag, backend)
	exitOnError(err)
	var layoutOverrides *layoutoverride.Document
	if *layoutOverridesPath != "" {
		if backend != "drawio" {
			exitOnError(fmt.Errorf("--layout-overrides currently requires the draw.io renderer"))
		}
		layoutOverrides, err = layoutoverride.Load(*layoutOverridesPath)
		exitOnError(err)
	}
	if *layoutReport && backend != "drawio" {
		exitOnError(fmt.Errorf("--layout-report requires the draw.io renderer"))
	}
	if *outputReport != "" && !*layoutReport {
		exitOnError(fmt.Errorf("--output-report requires --layout-report"))
	}
	if *outputReport == "" {
		*outputReport = "text"
	}
	if *outputReport != "text" && *outputReport != "json" {
		exitOnError(fmt.Errorf("--output-report must be text or json"))
	}

	var result []byte
	var drawioLayoutReport drawio.LayoutReport
	switch backend {
	case "native":
		result, err = svg.RenderWithOptions(diag, svg.Options{IconDir: iconDir})
	case "d2":
		if iconDir != "" {
			err = fmt.Errorf("custom SVG icon packs require the native renderer")
		} else {
			result, err = d2backend.Render(diag, d2backend.Options{Layout: *layout})
		}
	case "drawio":
		if iconDir != "" {
			err = fmt.Errorf("custom SVG icon packs are not embedded in draw.io output")
		} else if *layoutReport {
			result, drawioLayoutReport, err = drawio.RenderWithLayoutReport(diag, drawio.Options{Overrides: layoutOverrides})
		} else {
			result, err = drawio.RenderWithOptions(diag, drawio.Options{Overrides: layoutOverrides})
		}
	default:
		err = fmt.Errorf("unknown backend %q; use native, d2, or drawio", backend)
	}
	exitOnError(err)

	target := output
	if target == "" {
		extension := ".svg"
		if backend == "drawio" {
			extension = ".drawio"
		}
		target = strings.TrimSuffix(input, filepath.Ext(input)) + extension
	}
	targetExtension := strings.ToLower(filepath.Ext(target))
	if backend == "drawio" && targetExtension != ".drawio" {
		exitOnError(fmt.Errorf("draw.io renderer output must use the .drawio extension"))
	}
	if backend != "drawio" && targetExtension == ".drawio" {
		exitOnError(fmt.Errorf(".drawio output requires --renderer drawio"))
	}
	if strings.EqualFold(filepath.Ext(target), ".html") {
		if backend != "native" {
			exitOnError(fmt.Errorf("interactive HTML export requires the native renderer"))
		}
		result, err = interactive.Render(diag, result)
		exitOnError(err)
	}
	exitOnError(export.Write(target, result))
	reportLayout := diag.Theme.Layout
	if backend == "d2" {
		reportLayout = *layout
		if reportLayout == "" {
			reportLayout = "elk"
		}
	}
	report := planner.Report(renderPlan, reportLayout, target)
	if *reportPath != "" {
		exitOnError(writeJSONFile(*reportPath, report))
	}
	if *outputReport == "json" {
		fmt.Fprintf(os.Stderr, "rendered %s using %s\n", target, backend)
		writeJSON(drawioLayoutReport)
	} else {
		fmt.Printf("rendered %s using %s\n", target, backend)
	}
	if *layoutReport && *outputReport == "text" {
		fmt.Println()
		fmt.Print(drawio.FormatLayoutReport(drawioLayoutReport))
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning [%s]: %s\n", warning.Code, warning.Message)
	}
}

func doctor(args []string) {
	if len(args) != 2 || args[0] != "drawio" {
		fmt.Fprintln(os.Stderr, "usage: netdiag doctor drawio <diagram.drawio>")
		os.Exit(2)
	}
	// #nosec G703 -- doctor intentionally inspects the Draw.io path selected by the user.
	data, err := os.ReadFile(args[1])
	exitOnError(err)
	report, err := drawio.Doctor(data)
	exitOnError(err)
	fmt.Printf("Draw.io round-trip safe: %t\n", report.RoundTripSafe)
	fmt.Printf("Managed: %d nodes, %d groups, %d links, %d labels\n", report.Managed.Nodes, report.Managed.Groups, report.Managed.Links, report.Managed.Labels)
	fmt.Printf("Unmanaged: %d annotations, %d decorative shapes, %d connectors\n", report.Unmanaged.Annotations, report.Unmanaged.DecorativeShapes, report.Unmanaged.Connectors)
	for _, warning := range report.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	for _, problem := range report.Problems {
		fmt.Printf("problem: %s\n", problem)
	}
	if !report.RoundTripSafe {
		os.Exit(1)
	}
}

func diffLayout(args []string) {
	flags := commandFlags("diff-layout", "usage: netdiag diff-layout <old.layout.yaml> <new.layout.yaml> [--json]")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 2 {
		flags.Usage()
		os.Exit(2)
	}
	oldDoc, err := layoutoverride.Load(flags.Arg(0))
	exitOnError(err)
	newDoc, err := layoutoverride.Load(flags.Arg(1))
	exitOnError(err)
	diff := layoutoverride.Compare(oldDoc, newDoc)
	if *jsonOutput {
		writeJSON(diff)
		return
	}
	printLayoutChanges := func(kind string, changes layoutoverride.Changes) {
		for _, id := range changes.Added {
			fmt.Printf("added %s: %s\n", kind, id)
		}
		for _, id := range changes.Removed {
			fmt.Printf("removed %s: %s\n", kind, id)
		}
		for _, id := range changes.Changed {
			fmt.Printf("changed %s: %s\n", kind, id)
		}
	}
	printLayoutChanges("node", diff.Nodes)
	printLayoutChanges("group", diff.Groups)
	printLayoutChanges("link", diff.Links)
	if diff.Empty() {
		fmt.Println("no layout changes")
	}
}

func capabilities(args []string) {
	flags := commandFlags("capabilities", "usage: netdiag capabilities [--json]")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 0 {
		flags.Usage()
		os.Exit(2)
	}
	result := planner.Capabilities()
	if *jsonOutput {
		writeJSON(result)
		return
	}
	for _, renderer := range result {
		fmt.Printf("%s\n", renderer.Renderer)
		for _, capability := range renderer.Capabilities {
			fmt.Printf("  %-20s %-12s %s\n", capability.Feature, capability.Level, capability.Note)
		}
	}
}

func plan(args []string) {
	flags := commandFlags("plan", "usage: netdiag plan [--renderer native|d2|drawio] [--json] <diagram.yaml>")
	renderer := flags.String("renderer", "", "renderer to assess: native, d2, or drawio")
	backend := flags.String("backend", "", "alias for --renderer")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	doc, err := loadDocument(flags.Arg(0))
	exitOnError(err)
	diag, err := model.Compile(doc)
	exitOnError(err)
	selected := *renderer
	if selected == "" {
		selected = *backend
	}
	if selected == "" {
		selected = diag.Theme.Renderer
	}
	if selected == "" {
		selected = planner.Recommend(diag)
	}
	result, err := planner.Build(diag, selected)
	exitOnError(err)
	if *jsonOutput {
		writeJSON(result)
		return
	}
	fmt.Printf("renderer: %s\nrecommended: %s\n", result.Renderer, result.RecommendedRenderer)
	printAssessments("strict", result.Strict)
	printAssessments("best effort", result.BestEffort)
	printAssessments("unsupported", result.Unsupported)
	for _, warning := range result.Warnings {
		fmt.Printf("warning [%s]: %s\n", warning.Code, warning.Message)
	}
}

func recommend(args []string) {
	flags := commandFlags("recommend", "usage: netdiag recommend [--json] <diagram.yaml>")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	doc, err := loadDocument(flags.Arg(0))
	exitOnError(err)
	diag, err := model.Compile(doc)
	exitOnError(err)
	renderer := planner.Recommend(diag)
	if *jsonOutput {
		writeJSON(map[string]string{"recommended_renderer": renderer})
		return
	}
	fmt.Println(renderer)
}

func inspect(args []string) {
	flags := commandFlags("inspect", "usage: netdiag inspect [--json] [--fail-on warning|error] [--limit count] <diagram.yaml>")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	failOn := flags.String("fail-on", "", "exit non-zero when findings reach warning or error")
	limit := flags.Int("limit", 50, "maximum findings to print in text output; 0 prints all")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	if *limit < 0 {
		fmt.Fprintln(os.Stderr, "error: --limit cannot be negative")
		os.Exit(2)
	}
	var threshold svg.InspectionSeverity
	switch strings.ToLower(*failOn) {
	case "":
	case "warning":
		threshold = svg.InspectionWarning
	case "error":
		threshold = svg.InspectionError
	default:
		fmt.Fprintln(os.Stderr, "error: --fail-on requires warning or error")
		os.Exit(2)
	}
	doc, err := loadDocument(flags.Arg(0))
	exitOnError(err)
	diagram, err := model.Compile(doc)
	exitOnError(err)
	report, err := svg.Inspect(diagram)
	exitOnError(err)
	if *jsonOutput {
		writeJSON(report)
	} else {
		fmt.Printf("layout: %s\ncanvas: %.0fx%.0f\nscore: %d/100\n", report.Layout, report.Width, report.Height, report.Score)
		fmt.Printf("findings: %d error(s), %d warning(s), %d info\n", report.Summary.Errors, report.Summary.Warnings, report.Summary.Info)
		printed := len(report.Findings)
		if *limit > 0 && printed > *limit {
			printed = *limit
		}
		for _, finding := range report.Findings[:printed] {
			fmt.Printf("%s [%s]: %s\n", finding.Severity, finding.Code, finding.Message)
			if finding.Suggestion != "" {
				fmt.Printf("  suggestion: %s\n", finding.Suggestion)
			}
		}
		if printed < len(report.Findings) {
			fmt.Printf("... %d more finding(s); use --limit 0 or --json to show all\n", len(report.Findings)-printed)
		}
	}
	if threshold != "" && report.HasAtLeast(threshold) {
		os.Exit(1)
	}
}

func improveLayout(args []string) {
	flags := commandFlags("improve-layout", "usage: netdiag improve-layout <diagram.yaml> [-o improved.yaml] [--rounds count] [--max-candidates count] [--json]")
	var output string
	flags.StringVar(&output, "o", "", "output improved YAML path")
	flags.StringVar(&output, "output", "", "output improved YAML path")
	rounds := flags.Int("rounds", 3, "maximum repair rounds")
	maxCandidates := flags.Int("max-candidates", 80, "maximum candidates per round")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	input := flags.Arg(0)
	if *rounds < 1 || *maxCandidates < 1 {
		fmt.Fprintln(os.Stderr, "error: --rounds and --max-candidates must be greater than zero")
		os.Exit(2)
	}
	if output == "" {
		output = strings.TrimSuffix(input, filepath.Ext(input)) + ".improved.yaml"
	}
	doc, err := loadDocument(input)
	exitOnError(err)
	improved, report, err := layoutrepair.Improve(doc, layoutrepair.Options{MaxRounds: *rounds, MaxCandidates: *maxCandidates})
	exitOnError(err)
	result, err := spec.Format(improved)
	exitOnError(err)
	exitOnError(os.WriteFile(output, result, 0o644))
	if *jsonOutput {
		writeJSON(report)
		return
	}
	fmt.Printf("inspection score: %d -> %d", report.Before.Quality, report.After.Quality)
	if len(report.Changes) > 0 && report.Before.Quality == report.After.Quality {
		fmt.Print(" (weighted penalty improved)")
	}
	fmt.Println()
	fmt.Printf("weighted penalty: %d -> %d\n", report.Before.Penalty, report.After.Penalty)
	fmt.Printf("errors: %d -> %d, warnings: %d -> %d\n", report.Before.Errors, report.After.Errors, report.Before.Warnings, report.After.Warnings)
	fmt.Printf("evaluated %d candidate(s), accepted %d change(s)\n", report.CandidatesEvaluated, len(report.Changes))
	for _, change := range report.Changes {
		fmt.Printf("round %d: %s\n", change.Round, change.Description)
	}
	fmt.Printf("wrote %s\n", output)
}

func convertLLDP(args []string) {
	flags := commandFlags("discover lldp", "usage: netdiag discover lldp <output.txt|output.json|directory|-> [--format auto|openconfig|juniper-xml|cisco|juniper|arista] [--local hostname] [--auto-layout] [-o diagram.yaml]")
	format := flags.String("format", "auto", "input format")
	local := flags.String("local", "", "local device name")
	autoLayout := flags.Bool("auto-layout", false, "apply deterministic discovery layout")
	var output string
	flags.StringVar(&output, "o", "", "output topology YAML path")
	flags.StringVar(&output, "output", "", "output topology YAML path")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	input := flags.Arg(0)
	results, err := loadLLDPResults(input, *format, *local)
	exitOnError(err)
	doc, err := lldp.ToDocumentSet(results)
	exitOnError(err)
	var layoutReport discoverylayout.Report
	if *autoLayout {
		layoutReport = discoverylayout.Apply(doc)
	}
	exitOnError(spec.Prepare(doc))
	encoded, err := spec.Format(doc)
	exitOnError(err)
	if output == "" {
		fmt.Print(string(encoded))
		return
	}
	exitOnError(os.WriteFile(output, encoded, 0o644))
	report := lldp.BuildReport(results, doc)
	fmt.Printf("converted %d LLDP observations from %d device(s) into %d nodes and %d links at %s\n", report.Observations, report.Devices, report.Nodes, report.Links, output)
	if report.MergedObservations > 0 {
		fmt.Printf("merged %d reciprocal or duplicate observation(s)\n", report.MergedObservations)
	}
	printAutoLayoutReport(*autoLayout, layoutReport)
}

func discover(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: netdiag discover lldp <output.txt|output.json|directory|-> [options]")
		os.Exit(2)
	}
	switch args[0] {
	case "lldp":
		convertLLDP(args[1:])
	case "isis":
		convertISIS(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown discovery protocol %q; use lldp or isis\n", args[0])
		os.Exit(2)
	}
}

func convertISIS(args []string) {
	flags := commandFlags("discover isis", "usage: netdiag discover isis <output.txt|output.json|directory|-> [--format auto|iosxr|juniper-xml|openconfig] [--local hostname] [--auto-layout] [-o diagram.yaml]")
	format := flags.String("format", "auto", "input format")
	local := flags.String("local", "", "local device name")
	autoLayout := flags.Bool("auto-layout", false, "apply deterministic discovery layout")
	var output string
	flags.StringVar(&output, "o", "", "output topology YAML path")
	flags.StringVar(&output, "output", "", "output topology YAML path")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	input := flags.Arg(0)
	results, err := loadISISResults(input, *format, *local)
	exitOnError(err)
	doc, err := isis.ToDocumentSet(results)
	exitOnError(err)
	var layoutReport discoverylayout.Report
	if *autoLayout {
		layoutReport = discoverylayout.Apply(doc)
	}
	exitOnError(spec.Prepare(doc))
	encoded, err := spec.Format(doc)
	exitOnError(err)
	if output == "" {
		fmt.Print(string(encoded))
		return
	}
	exitOnError(os.WriteFile(output, encoded, 0o644))
	report := isis.BuildReport(results, doc)
	fmt.Printf("converted %d IS-IS observations from %d device(s) into %d nodes and %d links at %s\n", report.Observations, report.Devices, report.Nodes, report.Links, output)
	if report.MergedObservations > 0 {
		fmt.Printf("merged %d reciprocal or duplicate observation(s)\n", report.MergedObservations)
	}
	printAutoLayoutReport(*autoLayout, layoutReport)
}

func printAutoLayoutReport(enabled bool, report discoverylayout.Report) {
	if !enabled {
		return
	}
	fmt.Printf("auto-layout selected %s with %d group(s) using %s grouping; suppressed %d repeated middle label(s)\n",
		report.Layout, report.Groups, report.Grouping, report.SuppressedMiddleLabels)
}

func loadISISResults(input, format, local string) ([]isis.Result, error) {
	if input == "-" {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return nil, err
		}
		result, err := isis.Parse(data, format)
		if err != nil {
			return nil, err
		}
		result.LocalNode = firstNonEmpty(local, result.LocalNode)
		return []isis.Result{result}, nil
	}
	info, err := os.Stat(input)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, err
		}
		result, err := isis.Parse(data, format)
		if err != nil {
			return nil, err
		}
		result.LocalNode = firstNonEmpty(local, result.LocalNode)
		return []isis.Result{result}, nil
	}
	if local != "" {
		return nil, fmt.Errorf("--local cannot be used with a directory; prompts or filenames identify each local device")
	}
	entries, err := os.ReadDir(input)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var results []isis.Result
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !isISISCapture(entry.Name()) {
			continue
		}
		path := filepath.Join(input, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read IS-IS capture %s: %w", path, err)
		}
		result, err := isis.Parse(data, format)
		if err != nil {
			return nil, fmt.Errorf("parse IS-IS capture %s: %w", path, err)
		}
		if result.LocalNode == "" {
			result.LocalNode = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("directory %s contains no IS-IS .txt, .log, .out, .json, .xml, or extensionless captures", input)
	}
	return results, nil
}

func isISISCapture(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case "", ".txt", ".log", ".out", ".json", ".xml":
		return true
	default:
		return false
	}
}

func loadLLDPResults(input, format, local string) ([]lldp.Result, error) {
	if input == "-" {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return nil, err
		}
		result, err := lldp.Parse(data, format)
		if err != nil {
			return nil, err
		}
		result.LocalNode = firstNonEmpty(local, result.LocalNode)
		return []lldp.Result{result}, nil
	}
	info, err := os.Stat(input)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, err
		}
		result, err := lldp.Parse(data, format)
		if err != nil {
			return nil, err
		}
		result.LocalNode = firstNonEmpty(local, result.LocalNode)
		return []lldp.Result{result}, nil
	}
	if local != "" {
		return nil, fmt.Errorf("--local cannot be used with a directory; prompts or filenames identify each local device")
	}
	entries, err := os.ReadDir(input)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var results []lldp.Result
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !isLLDPCapture(entry.Name()) {
			continue
		}
		path := filepath.Join(input, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read LLDP capture %s: %w", path, err)
		}
		result, err := lldp.Parse(data, format)
		if err != nil {
			return nil, fmt.Errorf("parse LLDP capture %s: %w", path, err)
		}
		if result.LocalNode == "" {
			result.LocalNode = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("directory %s contains no LLDP .txt, .log, .out, .json, or .xml captures", input)
	}
	return results, nil
}

func isLLDPCapture(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case "", ".txt", ".log", ".out", ".json", ".xml":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func printAssessments(title string, assessments []planner.Assessment) {
	if len(assessments) == 0 {
		return
	}
	fmt.Printf("%s:\n", title)
	for _, assessment := range assessments {
		fmt.Printf("  %-20s %s\n", assessment.Feature, assessment.Reason)
	}
}

func validate(args []string) {
	flags := commandFlags("validate", "usage: netdiag validate [--json] <diagram.yaml>")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)

	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}

	doc, err := loadDocument(flags.Arg(0))
	if err != nil {
		if *jsonOutput {
			writeJSON(map[string]interface{}{"valid": false, "errors": []string{err.Error()}})
			os.Exit(1)
		}
		exitOnError(err)
	}

	diag, err := model.Compile(doc)
	if err != nil {
		if *jsonOutput {
			writeJSON(map[string]interface{}{"valid": false, "errors": []string{err.Error()}})
			os.Exit(1)
		}
		exitOnError(err)
	}
	if *jsonOutput {
		writeJSON(map[string]interface{}{"valid": true, "nodes": len(diag.Nodes), "links": len(diag.Links), "errors": []string{}})
		return
	}
	fmt.Printf("valid: %d nodes, %d links\n", len(diag.Nodes), len(diag.Links))
}

func schema(args []string) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: netdiag schema")
		os.Exit(2)
	}
	result, err := spec.JSONSchema()
	exitOnError(err)
	fmt.Println(string(result))
}

func expand(args []string) {
	flags := commandFlags("expand", "usage: netdiag expand <diagram.yaml> [-o expanded.yaml]")
	var output string
	flags.StringVar(&output, "o", "", "output expanded YAML path")
	flags.StringVar(&output, "output", "", "output expanded YAML path")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	input := flags.Arg(0)
	doc, err := loadDocument(input)
	exitOnError(err)
	result, err := spec.Format(doc)
	exitOnError(err)
	if output == "" {
		fmt.Print(string(result))
		return
	}
	exitOnError(os.WriteFile(output, result, 0o644))
	fmt.Printf("expanded %s\n", output)
}

func format(args []string) {
	flags := commandFlags("fmt", "usage: netdiag fmt [-w] <diagram.yaml>")
	write := flags.Bool("w", false, "write formatted YAML back to the input file")
	parseCommandFlags(flags, args)
	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(2)
	}
	result, err := source.Format(flags.Arg(0))
	exitOnError(err)
	if *write {
		exitOnError(os.WriteFile(flags.Arg(0), result, 0o644))
		fmt.Printf("formatted %s\n", flags.Arg(0))
		return
	}
	fmt.Print(string(result))
}

func loadDocument(path string) (*spec.Document, error) {
	registry, err := templateRegistry()
	if err != nil {
		return nil, err
	}
	result, err := templates.Load(path, registry)
	if err != nil {
		return nil, err
	}
	return result.Document, nil
}

func listTemplates(args []string) {
	flags := commandFlags("templates", "usage: netdiag templates [--json]")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 0 {
		flags.Usage()
		os.Exit(2)
	}
	registry, err := templateRegistry()
	exitOnError(err)
	items := registry.List()
	if *jsonOutput {
		writeJSON(items)
		return
	}
	for _, item := range items {
		fmt.Printf("%s v%d - %s\n", item.ID, item.Version, item.Description)
		fmt.Printf("  required: %s\n", formatParamList(item.RequiredParams))
		fmt.Printf("  optional: %s\n", formatParamList(item.OptionalParams))
	}
}

func listIcons(args []string) {
	flags := commandFlags("icons", "usage: netdiag icons [--json]")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	parseCommandFlags(flags, args)
	if flags.NArg() != 0 {
		flags.Usage()
		os.Exit(2)
	}
	items := icons.List()
	if *jsonOutput {
		writeJSON(items)
		return
	}
	for _, item := range items {
		fmt.Printf("%-14s %-10s %s\n", item.ID, item.Category, item.Description)
		if len(item.Aliases) > 0 {
			fmt.Printf("  aliases: %s\n", strings.Join(item.Aliases, ", "))
		}
	}
}

func templateRegistry() (*templates.TemplateRegistry, error) {
	root := os.Getenv("NETDIAG_TEMPLATES")
	if root == "" {
		root = "templates"
	}
	return templates.NewTemplateRegistry(root)
}

func formatParamList(params []string) string {
	if len(params) == 0 {
		return "-"
	}
	return strings.Join(params, ", ")
}

func writeJSON(value interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	exitOnError(encoder.Encode(value))
}

func writeJSONFile(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func exitOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func usage() {
	fmt.Print(`netdiag renders concise YAML network diagrams.

Usage:
  netdiag render <diagram.yaml> [-o diagram.svg|diagram.html|diagram.png|diagram.pdf|diagram.drawio] [--backend native|d2|drawio] [--layout-overrides layout.yaml] [--layout-report] [--output-report text|json] [--icons directory] [--layout elk|dagre]
  netdiag capabilities [--json]
  netdiag plan [--renderer native|d2|drawio] [--json] <diagram.yaml>
  netdiag recommend [--json] <diagram.yaml>
  netdiag inspect [--json] [--fail-on warning|error] [--limit count] <diagram.yaml>
  netdiag improve-layout <diagram.yaml> [-o improved.yaml] [--rounds count] [--max-candidates count] [--json]
  netdiag extract-overrides <edited.drawio> --source <diagram.yaml> [-o diagram.layout.yaml] [--report]
  netdiag doctor drawio <diagram.drawio>
  netdiag diff-layout <old.layout.yaml> <new.layout.yaml> [--json]
  netdiag discover lldp <output.txt|output.json|directory|-> [--format auto|openconfig|juniper-xml|cisco|juniper|arista] [--local hostname] [--auto-layout] [-o diagram.yaml]
  netdiag discover isis <output.txt|output.json|directory|-> [--format auto|iosxr|juniper-xml|openconfig] [--local hostname] [--auto-layout] [-o diagram.yaml]
  netdiag lldp ...  (compatibility alias)
  netdiag validate [--json] <diagram.yaml>
  netdiag expand <diagram.yaml> [-o expanded.yaml]
  netdiag fmt [-w] <diagram.yaml>
  netdiag templates [--json]
  netdiag icons [--json]
  netdiag schema
`)
}
