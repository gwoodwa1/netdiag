package icons

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxSVGIconSize = 1 << 20

var safeIconID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var allowedSVGElements = map[string]bool{
	"circle": true, "clipPath": true, "defs": true, "ellipse": true,
	"g": true, "line": true, "linearGradient": true, "path": true,
	"polygon": true, "polyline": true, "radialGradient": true, "rect": true,
	"stop": true,
}

var allowedSVGAttributes = map[string]bool{
	"clip-path": true, "clip-rule": true, "cx": true, "cy": true, "d": true,
	"fill": true, "fill-opacity": true, "fill-rule": true, "fx": true, "fy": true,
	"gradientTransform": true, "gradientUnits": true, "height": true, "id": true,
	"offset": true, "opacity": true, "points": true, "r": true, "rx": true, "ry": true,
	"spreadMethod": true, "stop-color": true, "stop-opacity": true, "stroke": true,
	"stroke-dasharray": true, "stroke-dashoffset": true, "stroke-linecap": true,
	"stroke-linejoin": true, "stroke-miterlimit": true, "stroke-opacity": true,
	"stroke-width": true, "transform": true, "viewBox": true, "width": true,
	"x": true, "x1": true, "x2": true, "y": true, "y1": true, "y2": true,
}

type SVG struct {
	ViewBox string
	Content string
	Prefix  string
}

type cachedSVG struct {
	icon SVG
	ok   bool
}

// Pack lazily discovers and parses SVG icons from one local directory.
type Pack struct {
	dir   string
	cache map[string]cachedSVG
}

func NewPack(dir string) *Pack {
	return &Pack{dir: dir, cache: make(map[string]cachedSVG)}
}

// Resolve returns a safe, embeddable SVG icon. Missing and invalid files are
// deliberately indistinguishable so callers can fall back to built-in icons.
func (p *Pack) Resolve(id string) (SVG, bool) {
	if p == nil || p.dir == "" || !safeIconID.MatchString(id) {
		return SVG{}, false
	}
	if cached, ok := p.cache[id]; ok {
		return cached.icon, cached.ok
	}

	path := filepath.Join(p.dir, id+".svg")
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxSVGIconSize {
		p.cache[id] = cachedSVG{}
		return SVG{}, false
	}
	icon, err := sanitizeSVG(data, "netdiag-icon-"+id+"-")
	if err != nil {
		p.cache[id] = cachedSVG{}
		return SVG{}, false
	}
	p.cache[id] = cachedSVG{icon: icon, ok: true}
	return icon, true
}

func sanitizeSVG(data []byte, idPrefix string) (SVG, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var out strings.Builder
	var viewBox string
	depth := 0

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return SVG{}, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			name := item.Name.Local
			if depth == 0 {
				if name != "svg" {
					return SVG{}, &xml.SyntaxError{Msg: "icon root must be svg"}
				}
				for _, attr := range item.Attr {
					if attr.Name.Local == "viewBox" {
						viewBox = strings.TrimSpace(attr.Value)
					}
				}
				if viewBox == "" {
					return SVG{}, &xml.SyntaxError{Msg: "icon svg requires viewBox"}
				}
				depth++
				continue
			}
			if !allowedSVGElements[name] {
				return SVG{}, &xml.SyntaxError{Msg: "unsafe svg element " + name}
			}
			out.WriteByte('<')
			out.WriteString(name)
			for _, attr := range item.Attr {
				attrName := attr.Name.Local
				if strings.HasPrefix(strings.ToLower(attrName), "on") || !allowedSVGAttributes[attrName] {
					return SVG{}, &xml.SyntaxError{Msg: "unsafe svg attribute " + attrName}
				}
				value, ok := safeAttributeValue(attrName, attr.Value, idPrefix)
				if !ok {
					return SVG{}, &xml.SyntaxError{Msg: "unsafe svg attribute value"}
				}
				out.WriteByte(' ')
				out.WriteString(attrName)
				out.WriteString(`="`)
				out.WriteString(escapeAttribute(value))
				out.WriteByte('"')
			}
			out.WriteByte('>')
			depth++
		case xml.EndElement:
			depth--
			if depth > 0 {
				out.WriteString("</")
				out.WriteString(item.Name.Local)
				out.WriteByte('>')
			}
		case xml.CharData:
			if depth > 1 && strings.TrimSpace(string(item)) != "" {
				return SVG{}, &xml.SyntaxError{Msg: "svg text content is not allowed"}
			}
		case xml.ProcInst, xml.Directive:
			return SVG{}, &xml.SyntaxError{Msg: "svg directives are not allowed"}
		}
	}
	if depth != 0 {
		return SVG{}, &xml.SyntaxError{Msg: "unclosed svg element"}
	}
	return SVG{ViewBox: viewBox, Content: out.String(), Prefix: idPrefix}, nil
}

func safeAttributeValue(name, value, idPrefix string) (string, bool) {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") || strings.Contains(lower, "http:") || strings.Contains(lower, "https:") || strings.Contains(lower, "file:") {
		return "", false
	}
	if name == "id" {
		if !safeIconID.MatchString(value) {
			return "", false
		}
		return idPrefix + value, true
	}
	if strings.Contains(lower, "url(") {
		if !strings.HasPrefix(value, "url(#") || !strings.HasSuffix(value, ")") {
			return "", false
		}
		ref := strings.TrimSuffix(strings.TrimPrefix(value, "url(#"), ")")
		if !safeIconID.MatchString(ref) {
			return "", false
		}
		return "url(#" + idPrefix + ref + ")", true
	}
	return value, true
}

func escapeAttribute(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}
