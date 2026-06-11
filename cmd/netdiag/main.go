package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/d2backend"
	"github.com/gwoodwa1/netdiag/internal/export"
	"github.com/gwoodwa1/netdiag/internal/icons"
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
				fmt.Fprintln(os.Stderr, "error: --backend requires native or d2")
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
		fmt.Fprintln(os.Stderr, "usage: netdiag render <diagram.yaml> [-o diagram.svg|diagram.png|diagram.pdf] [--renderer native|d2] [--icons directory] [--layout elk|dagre] [--report report.json]")
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
	default:
		err = fmt.Errorf("unknown backend %q; use native or d2", backend)
	}
	exitOnError(err)

	target := output
	if target == "" {
		target = strings.TrimSuffix(input, filepath.Ext(input)) + ".svg"
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
	renderer := flags.String("renderer", "", "renderer to assess: native or d2")
	backend := flags.String("backend", "", "alias for --renderer")
	jsonOutput := flags.Bool("json", false, "emit machine-readable JSON")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: netdiag plan [--renderer native|d2] [--json] <diagram.yaml>")
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
  netdiag render <diagram.yaml> [-o diagram.svg|diagram.png|diagram.pdf] [--backend native|d2] [--icons directory] [--layout elk|dagre]
  netdiag capabilities [--json]
  netdiag plan [--renderer native|d2] [--json] <diagram.yaml>
  netdiag recommend [--json] <diagram.yaml>
  netdiag validate [--json] <diagram.yaml>
  netdiag expand <diagram.yaml> [-o expanded.yaml]
  netdiag fmt [-w] <diagram.yaml>
  netdiag templates [--json]
  netdiag icons [--json]
  netdiag schema
`)
}
