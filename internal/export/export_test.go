package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSVG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagram.svg")
	if err := Write(path, []byte("<svg/>")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "<svg/>" {
		t.Fatalf("unexpected SVG output: %q, %v", data, err)
	}
}

func TestWriteRejectsUnknownFormat(t *testing.T) {
	err := Write(filepath.Join(t.TempDir(), "diagram.bmp"), []byte("<svg/>"))
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestWriteUsesConfiguredConverterForPNGAndPDF(t *testing.T) {
	root := t.TempDir()
	converter := filepath.Join(root, "converter")
	script := `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-f" ]; then format="$2"; shift 2; continue; fi
  if [ "$1" = "-o" ]; then output="$2"; shift 2; continue; fi
  shift
done
printf '%s' "$format" > "$output"
`
	if err := os.WriteFile(converter, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NETDIAG_CONVERTER", converter)
	for _, format := range []string{"png", "pdf"} {
		path := filepath.Join(root, "diagram."+format)
		if err := Write(path, []byte("<svg/>")); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != format {
			t.Fatalf("unexpected %s output: %q, %v", format, data, err)
		}
	}
}
