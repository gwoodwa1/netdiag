package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNestedIncludesInDeclarationOrder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.yaml", `
version: 1
include: [parts/sites.yaml, parts/core.yaml]
diagram: {title: Included WAN}
links:
  - from: site:Eth0
    to: core:Eth0
`)
	writeFile(t, root, "parts/sites.yaml", `
version: 1
include: [shared.yaml]
nodes:
  site: {role: edge-router}
links:
  - from: shared:Eth0
    to: site:Eth1
`)
	writeFile(t, root, "parts/shared.yaml", `
version: 1
nodes:
  shared: {role: router}
`)
	writeFile(t, root, "parts/core.yaml", `
version: 1
nodes:
  core: {role: core-router}
links:
  - from: site:Eth2
    to: core:Eth2
`)

	doc, err := Load(filepath.Join(root, "main.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 3 || len(doc.Links) != 3 {
		t.Fatalf("unexpected merge counts: %d nodes, %d links", len(doc.Nodes), len(doc.Links))
	}
	if doc.Links[0].From.Node != "shared" || doc.Links[1].From.Node != "site" || doc.Links[2].From.Node != "site" {
		t.Fatalf("links merged out of order: %#v", doc.Links)
	}
	if doc.Diagram.Title != "Included WAN" || len(doc.Include) != 0 {
		t.Fatalf("unexpected root metadata: %#v", doc)
	}
}

func TestLoadRejectsDuplicateIDs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.yaml", "version: 1\ninclude: [part.yaml]\nnodes:\n  router: {role: router}\n")
	writeFile(t, root, "part.yaml", "version: 1\nnodes:\n  router: {role: router}\n")

	_, err := Load(filepath.Join(root, "main.yaml"))
	if err == nil || !strings.Contains(err.Error(), `duplicate node ID "router"`) {
		t.Fatalf("expected duplicate node error, got %v", err)
	}
}

func TestLoadRejectsIncludeCycle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.yaml", "version: 1\ninclude: [b.yaml]\n")
	writeFile(t, root, "b.yaml", "version: 1\ninclude: [a.yaml]\n")

	_, err := Load(filepath.Join(root, "a.yaml"))
	if err == nil || !strings.Contains(err.Error(), "include cycle: a.yaml -> b.yaml -> a.yaml") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestLoadShowsContextForInvalidYAML(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.yaml", `
version: 1
diagram:
  title: Broken
  unknown_option: true
nodes:
  router: {role: router}
`)

	_, err := Load(filepath.Join(root, "main.yaml"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	for _, expected := range []string{"line 4:", ">    4 |   unknown_option: true", "field unknown_option not found"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("diagnostic missing %q:\n%s", expected, err)
		}
	}
}

func TestLoadRejectsPathOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, parent, "outside.yaml", "version: 1\n")
	writeFile(t, root, "main.yaml", "version: 1\ninclude: [../outside.yaml]\n")

	_, err := Load(filepath.Join(root, "main.yaml"))
	if err == nil || !strings.Contains(err.Error(), "escapes project root") {
		t.Fatalf("expected path escape error, got %v", err)
	}
}

func TestLoadRejectsAbsoluteInclude(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.yaml", "version: 1\ninclude: [/tmp/part.yaml]\n")

	_, err := Load(filepath.Join(root, "main.yaml"))
	if err == nil || !strings.Contains(err.Error(), "absolute paths are not allowed") {
		t.Fatalf("expected absolute path error, got %v", err)
	}
}

func TestLoadRejectsSymlinkOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, parent, "outside.yaml", "version: 1\n")
	if err := os.Symlink(filepath.Join(parent, "outside.yaml"), filepath.Join(root, "outside-link.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeFile(t, root, "main.yaml", "version: 1\ninclude: [outside-link.yaml]\n")

	_, err := Load(filepath.Join(root, "main.yaml"))
	if err == nil || !strings.Contains(err.Error(), "escapes project root") {
		t.Fatalf("expected symlink path escape error, got %v", err)
	}
}

func TestFormatPreservesAuthoredTemplateStructure(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "diagram.yaml", `
version: 1
include: [parts/core.yaml]
use:
  - template: site.dual-pe
    as: london
    params: {site_label: London}
connect:
  - from: london-pe1:Ethernet0/0
    to: core-p1:Ethernet0/0
    label: 100G
`)

	formatted, err := Format(filepath.Join(root, "diagram.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(formatted)
	for _, authoredKey := range []string{"include:", "use:", "connect:", "template: site.dual-pe", "from: london-pe1:Ethernet0/0"} {
		if !strings.Contains(text, authoredKey) {
			t.Fatalf("formatted source lost %q:\n%s", authoredKey, text)
		}
	}
	if strings.Contains(text, "nodes:") || strings.Contains(text, "groups:") {
		t.Fatalf("formatted source was expanded:\n%s", text)
	}
}

func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
