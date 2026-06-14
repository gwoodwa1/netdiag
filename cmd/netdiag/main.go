package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/d2backend"
	"github.com/gwoodwa1/netdiag/internal/discoverylayout"
	"github.com/gwoodwa1/netdiag/internal/drawio"
	"github.com/gwoodwa1/netdiag/internal/export"
	"github.com/gwoodwa1/netdiag/internal/icons"
	"github.com/gwoodwa1/netdiag/internal/interactive"
	"github.com/gwoodwa1/netdiag/internal/isis"
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

func render(args []string) {
	var input, output, backend, layout, reportPath, iconDir string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -o requires an output path")
				os.Exit(2)
			}
			i++
			output = args[i]
		case "--backend", "--renderer":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --backend requires native, d2, or drawio")
				os.Exit(2)
			}
			i++
			backend = args[i]
		case "--report":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --report requires an output path")
				os.Exit(2)
			}
			i++
			reportPath = args[i]
		case "--layout":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --layout requires elk or dagre")
				os.Exit(2)
			}
			i++
			layout = args[i]
		case "--icons", "--icon-dir":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --icons requires a directory")
				os.Exit(2)
			}
			i++
			iconDir = args[i]
		default:
			if strings.HasPrefix(args[i], "-") || input != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[i])
				os.Exit(2)
			}
			input = args[i]
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "usage: netdiag render <diagram.yaml> [-o diagram.svg|diagram.html|diagram.png|diagram.pdf|diagram.drawio] [--renderer native|d2|drawio] [--icons directory] [--layout elk|dagre] [--report report.json]")
		os.Exit(2)
	}
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

	var result []byte
	switch backend {
	case "native":
		result, err = svg.RenderWithOptions(diag, svg.Options{IconDir: iconDir})
	case "d2":
		if iconDir != "" {
			err = fmt.Errorf("custom SVG icon packs require the native renderer")
		} else {
			result, err = d2backend.Render(diag, d2backend.Options{Layout: layout})
		}
	case "drawio":
		if iconDir != "" {
			err = fmt.Errorf("custom SVG icon packs are not embedded in draw.io output")
		} else {
			result, err = drawio.Render(diag)
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
		reportLayout = layout
		if reportLayout == "" {
			reportLayout = "elk"
		}
	}
	report := planner.Report(renderPlan, reportLayout, target)
	if reportPath != "" {
		exitOnError(writeJSONFile(reportPath, report))
	}
	fmt.Printf("rendered %s using %s\n", target, backend)
	for _, warning := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning [%s]: %s\n", warning.Code, warning.Message)
	}
}

func capabilities(args []string) {
	flags := flag.NewFlagSet("capabilities", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: netdiag capabilities [--json]")
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
	flags := flag.NewFlagSet("plan", flag.ExitOnError)
	renderer := flags.String("renderer", "", "renderer to assess: native, d2, or drawio")
	backend := flags.String("backend", "", "alias for --renderer")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: netdiag plan [--renderer native|d2|drawio] [--json] <diagram.yaml>")
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
	flags := flag.NewFlagSet("recommend", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: netdiag recommend [--json] <diagram.yaml>")
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
	flags := flag.NewFlagSet("inspect", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	failOn := flags.String("fail-on", "", "exit non-zero when findings reach warning or error")
	limit := flags.Int("limit", 50, "maximum findings to print in text output; 0 prints all")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: netdiag inspect [--json] [--fail-on warning|error] [--limit count] <diagram.yaml>")
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
	input, output := "", ""
	rounds, maxCandidates := 3, 80
	jsonOutput := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-o", "--output":
			if index+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -o requires an output path")
				os.Exit(2)
			}
			index++
			output = args[index]
		case "--rounds", "--max-candidates":
			if index+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: %s requires a count\n", args[index])
				os.Exit(2)
			}
			name := args[index]
			index++
			value, err := strconv.Atoi(args[index])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s requires an integer\n", name)
				os.Exit(2)
			}
			if name == "--rounds" {
				rounds = value
			} else {
				maxCandidates = value
			}
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(args[index], "-") || input != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[index])
				os.Exit(2)
			}
			input = args[index]
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "usage: netdiag improve-layout <diagram.yaml> [-o improved.yaml] [--rounds count] [--max-candidates count] [--json]")
		os.Exit(2)
	}
	if rounds < 1 || maxCandidates < 1 {
		fmt.Fprintln(os.Stderr, "error: --rounds and --max-candidates must be greater than zero")
		os.Exit(2)
	}
	if output == "" {
		output = strings.TrimSuffix(input, filepath.Ext(input)) + ".improved.yaml"
	}
	doc, err := loadDocument(input)
	exitOnError(err)
	improved, report, err := layoutrepair.Improve(doc, layoutrepair.Options{MaxRounds: rounds, MaxCandidates: maxCandidates})
	exitOnError(err)
	result, err := spec.Format(improved)
	exitOnError(err)
	exitOnError(os.WriteFile(output, result, 0o644))
	if jsonOutput {
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
	format, input, local, output := "auto", "", "", ""
	autoLayout := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --format requires auto, openconfig, juniper-xml, cisco, juniper, or arista")
				os.Exit(2)
			}
			i++
			format = args[i]
		case "--local":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --local requires a device name")
				os.Exit(2)
			}
			i++
			local = args[i]
		case "--auto-layout":
			autoLayout = true
		case "-o", "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -o requires an output path")
				os.Exit(2)
			}
			i++
			output = args[i]
		default:
			if (strings.HasPrefix(args[i], "-") && args[i] != "-") || input != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[i])
				os.Exit(2)
			}
			input = args[i]
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "usage: netdiag discover lldp <output.txt|output.json|directory|-> [--format auto|openconfig|juniper-xml|cisco|juniper|arista] [--local hostname] [--auto-layout] [-o diagram.yaml]")
		os.Exit(2)
	}
	results, err := loadLLDPResults(input, format, local)
	exitOnError(err)
	doc, err := lldp.ToDocumentSet(results)
	exitOnError(err)
	var layoutReport discoverylayout.Report
	if autoLayout {
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
	printAutoLayoutReport(autoLayout, layoutReport)
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
	format, input, local, output := "auto", "", "", ""
	autoLayout := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --format requires auto, iosxr, juniper-xml, or openconfig")
				os.Exit(2)
			}
			i++
			format = args[i]
		case "--local":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --local requires a device name")
				os.Exit(2)
			}
			i++
			local = args[i]
		case "--auto-layout":
			autoLayout = true
		case "-o", "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -o requires an output path")
				os.Exit(2)
			}
			i++
			output = args[i]
		default:
			if (strings.HasPrefix(args[i], "-") && args[i] != "-") || input != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[i])
				os.Exit(2)
			}
			input = args[i]
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "usage: netdiag discover isis <output.txt|output.json|directory|-> [--format auto|iosxr|juniper-xml|openconfig] [--local hostname] [--auto-layout] [-o diagram.yaml]")
		os.Exit(2)
	}
	results, err := loadISISResults(input, format, local)
	exitOnError(err)
	doc, err := isis.ToDocumentSet(results)
	exitOnError(err)
	var layoutReport discoverylayout.Report
	if autoLayout {
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
	printAutoLayoutReport(autoLayout, layoutReport)
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
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)

	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: netdiag validate [--json] <diagram.yaml>")
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
	var input, output string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: -o requires an output path")
				os.Exit(2)
			}
			i++
			output = args[i]
		default:
			if strings.HasPrefix(args[i], "-") || input != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", args[i])
				os.Exit(2)
			}
			input = args[i]
		}
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "usage: netdiag expand <diagram.yaml> [-o expanded.yaml]")
		os.Exit(2)
	}
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
	flags := flag.NewFlagSet("fmt", flag.ExitOnError)
	write := flags.Bool("w", false, "write formatted YAML back to the input file")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: netdiag fmt [-w] <diagram.yaml>")
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
	flags := flag.NewFlagSet("templates", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: netdiag templates [--json]")
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
	flags := flag.NewFlagSet("icons", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: netdiag icons [--json]")
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
  netdiag render <diagram.yaml> [-o diagram.svg|diagram.html|diagram.png|diagram.pdf|diagram.drawio] [--backend native|d2|drawio] [--icons directory] [--layout elk|dagre]
  netdiag capabilities [--json]
  netdiag plan [--renderer native|d2|drawio] [--json] <diagram.yaml>
  netdiag recommend [--json] <diagram.yaml>
  netdiag inspect [--json] [--fail-on warning|error] [--limit count] <diagram.yaml>
  netdiag improve-layout <diagram.yaml> [-o improved.yaml] [--rounds count] [--max-candidates count] [--json]
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
