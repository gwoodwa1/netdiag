package svg

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"strings"
	"unicode"
)

func renderDefinitions(out *bytes.Buffer, premium bool, theme string) {
	out.WriteString(`<defs><filter id="shadow" x="-20%" y="-20%" width="140%" height="150%"><feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="#0f172a" flood-opacity=".14"/></filter>`)
	if premium {
		out.WriteString(`<linearGradient id="canvasGradient" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#f8fbff"/><stop offset="1" stop-color="#eef4fa"/></linearGradient>`)
		out.WriteString(`<linearGradient id="deviceCardGradient" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ffffff"/><stop offset=".58" stop-color="#f8fafc"/><stop offset="1" stop-color="#e8eef5"/></linearGradient>`)
		out.WriteString(`<linearGradient id="siteGradient" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#eff6ff" stop-opacity=".96"/><stop offset="1" stop-color="#dbeafe" stop-opacity=".72"/></linearGradient>`)
		out.WriteString(`<linearGradient id="titleGradient" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="#0f172a"/><stop offset=".72" stop-color="#172554"/><stop offset="1" stop-color="#0c4a6e"/></linearGradient>`)
		out.WriteString(`<pattern id="technicalGrid" width="32" height="32" patternUnits="userSpaceOnUse"><path d="M32 0H0V32" fill="none" stroke="#94a3b8" stroke-width=".55" stroke-opacity=".12"/><circle cx="0" cy="0" r="1" fill="#64748b" fill-opacity=".16"/></pattern>`)
		out.WriteString(`<filter id="deviceShadow" x="-25%" y="-30%" width="150%" height="175%"><feDropShadow dx="0" dy="2" stdDeviation="2" flood-color="#0f172a" flood-opacity=".12"/><feDropShadow dx="0" dy="8" stdDeviation="10" flood-color="#0f172a" flood-opacity=".15"/></filter>`)
		out.WriteString(`<filter id="portGlow" x="-150%" y="-150%" width="400%" height="400%"><feGaussianBlur stdDeviation="2.2" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`)
		out.WriteString(`<style>.premium-link{paint-order:stroke;}.label-mask+text{paint-order:stroke fill;stroke:#fff;stroke-width:2.2px;stroke-linejoin:round}.node-title{paint-order:stroke fill;stroke:#fff;stroke-width:1.4px;stroke-linejoin:round}</style>`)
	}
	if css := themeCSS(theme); css != "" {
		fmt.Fprintf(out, `<style>%s</style>`, css)
	}
	out.WriteString(`</defs>`)
}

func defaultTheme(theme string) string {
	if theme == "" {
		return "light"
	}
	return theme
}

func themeCSS(theme string) string {
	switch theme {
	case "nord":
		return `.theme-nord [fill="#f8fafc"],.theme-nord [fill="#ffffff"]{fill:#3b4252!important}.theme-nord [fill="#eff6ff"],.theme-nord [fill="#f1f5f9"],.theme-nord [fill="#f5f3ff"],.theme-nord [fill="#ecfeff"],.theme-nord [fill="#f0fdf4"]{fill:#434c5e!important}.theme-nord [fill="#0f172a"]{fill:#2e3440!important}.theme-nord [fill="#334155"],.theme-nord [fill="#475569"],.theme-nord [fill="#64748b"]{fill:#d8dee9!important}.theme-nord [stroke="#e2e8f0"],.theme-nord [stroke="#cbd5e1"],.theme-nord [stroke="#dbeafe"]{stroke:#4c566a!important}.theme-nord text{fill:#eceff4}.theme-nord .label-mask{fill:#3b4252!important;stroke:#3b4252!important}`
	case "dracula":
		return `.theme-dracula [fill="#f8fafc"],.theme-dracula [fill="#ffffff"]{fill:#282a36!important}.theme-dracula [fill="#eff6ff"],.theme-dracula [fill="#f1f5f9"],.theme-dracula [fill="#f5f3ff"],.theme-dracula [fill="#ecfeff"],.theme-dracula [fill="#f0fdf4"]{fill:#343746!important}.theme-dracula [fill="#0f172a"]{fill:#191a21!important}.theme-dracula [fill="#334155"],.theme-dracula [fill="#475569"],.theme-dracula [fill="#64748b"]{fill:#f8f8f2!important}.theme-dracula [stroke="#e2e8f0"],.theme-dracula [stroke="#cbd5e1"],.theme-dracula [stroke="#dbeafe"]{stroke:#6272a4!important}.theme-dracula text{fill:#f8f8f2}.theme-dracula .label-mask{fill:#282a36!important;stroke:#282a36!important}`
	default:
		return ""
	}
}

func escape(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}

func escapeID(value string) string {
	var out strings.Builder
	dash := false
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.' {
			out.WriteRune(char)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		result = "item"
	}
	if result != value {
		sum := sha256.Sum256([]byte(value))
		result = fmt.Sprintf("%s-%x", result, sum[:4])
	}
	return result
}
