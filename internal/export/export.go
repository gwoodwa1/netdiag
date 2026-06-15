package export

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Write(path string, svg []byte) error {
	format := strings.ToLower(filepath.Ext(path))
	switch format {
	case "", ".svg", ".html", ".drawio":
		return os.WriteFile(path, svg, 0o644)
	case ".png", ".pdf":
		return convert(path, format[1:], svg)
	default:
		return fmt.Errorf("unsupported output format %q; use .svg, .html, .drawio, .png, or .pdf", format)
	}
}

func convert(path, format string, svg []byte) error {
	temp, err := os.CreateTemp("", "netdiag-*.svg")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if _, err := temp.Write(svg); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}

	command, args, err := converterCommand(tempPath, path, format)
	if err != nil {
		return err
	}
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("export %s using %s: %w: %s", format, command, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func converterCommand(input, output, format string) (string, []string, error) {
	if configured := os.Getenv("NETDIAG_CONVERTER"); configured != "" {
		return configured, []string{"-f", format, "-o", output, input}, nil
	}
	if command, err := exec.LookPath("rsvg-convert"); err == nil {
		return command, []string{"-f", format, "-o", output, input}, nil
	}
	if command, err := exec.LookPath("inkscape"); err == nil {
		return command, []string{input, "--export-type=" + format, "--export-filename=" + output}, nil
	}
	for _, name := range []string{"magick", "convert"} {
		if command, err := exec.LookPath(name); err == nil {
			return command, []string{input, output}, nil
		}
	}
	return "", nil, fmt.Errorf("exporting %s requires rsvg-convert, inkscape, or ImageMagick; set NETDIAG_CONVERTER to an rsvg-convert-compatible executable", format)
}
