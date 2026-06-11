package icons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackLoadsAndCachesSafeSVG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "router.svg")
	data := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 60"><defs><linearGradient id="blue"><stop offset="0" stop-color="#fff"/></linearGradient></defs><rect width="100" height="60" fill="url(#blue)"/></svg>`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	pack := NewPack(dir)
	icon, ok := pack.Resolve("router")
	if !ok {
		t.Fatal("expected custom router icon")
	}
	if icon.ViewBox != "0 0 100 60" || icon.Prefix != "netdiag-icon-router-" || !strings.Contains(icon.Content, `id="netdiag-icon-router-blue"`) || !strings.Contains(icon.Content, `url(#netdiag-icon-router-blue)`) {
		t.Fatalf("unexpected sanitized icon: %#v", icon)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := pack.Resolve("router"); !ok {
		t.Fatal("expected parsed icon to remain cached")
	}
}

func TestPackTreatsMissingAndUnsafeSVGAsUnavailable(t *testing.T) {
	dir := t.TempDir()
	unsafe := `<svg viewBox="0 0 10 10"><script>alert(1)</script></svg>`
	if err := os.WriteFile(filepath.Join(dir, "router.svg"), []byte(unsafe), 0o644); err != nil {
		t.Fatal(err)
	}

	pack := NewPack(dir)
	for _, id := range []string{"router", "missing", "../router"} {
		if _, ok := pack.Resolve(id); ok {
			t.Fatalf("unsafe or missing icon %q must be unavailable", id)
		}
	}
}

func TestPackRejectsExternalReferencesAndEventHandlers(t *testing.T) {
	for name, data := range map[string]string{
		"external": `<svg viewBox="0 0 10 10"><rect fill="url(https://example.com/a.svg)"/></svg>`,
		"event":    `<svg viewBox="0 0 10 10"><rect onload="alert(1)"/></svg>`,
		"image":    `<svg viewBox="0 0 10 10"><image href="file:///tmp/x"/></svg>`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "router.svg")
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, ok := NewPack(filepath.Dir(path)).Resolve("router"); ok {
				t.Fatal("unsafe icon must be unavailable")
			}
		})
	}
}
