package svg

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gwoodwa1/netdiag/internal/model"
	"github.com/gwoodwa1/netdiag/internal/spec"
)

func renderEndpointLabel(out *bytes.Buffer, endpoint point, label, side string, lane int, style model.InterfaceLabelStyle) {
	renderRotatedEndpointLabel(out, endpoint, label, side, lane, 0, style)
}

func renderRotatedEndpointLabel(out *bytes.Buffer, endpoint point, label, side string, lane, rotation int, style model.InterfaceLabelStyle) {
	x := endpoint.X
	y := endpoint.Y - 12
	if side == "bottom" {
		y = endpoint.Y + 25
	} else if side == "left" {
		x = endpoint.X - horizontalLabelOffset
		y = endpoint.Y - 12
	} else if side == "right" {
		x = endpoint.X + horizontalLabelOffset
		y = endpoint.Y - 12
	}
	renderRotatedInterfaceLabel(out, x, y, label, rotation, style)
}

func renderRouteEndpointLabel(out *bytes.Buffer, route linkRoute, label string, source bool, degree, lane int, style model.InterfaceLabelStyle) {
	renderRotatedRouteEndpointLabel(out, route, label, source, degree, lane, 0, style)
}

func renderRotatedRouteEndpointLabel(out *bytes.Buffer, route linkRoute, label string, source bool, degree, lane, rotation int, style model.InterfaceLabelStyle) {
	if len(route.Points) < 2 {
		return
	}
	if location, ok := routeStubLabelPoint(route, source); ok {
		renderRotatedInterfaceLabel(out, location.X, location.Y+4, label, rotation, style)
		return
	}
	position := 0.13 + float64(lane%3)*0.025
	if degree > 4 {
		position = 0.28 + float64(lane%3)*0.025
	}
	if !source {
		position = 1 - position
	}
	location := pointAlongRoute(route, position)
	renderRotatedInterfaceLabel(out, location.X, location.Y+4, label, rotation, style)
}

func routeStubLabelPoint(route linkRoute, source bool) (point, bool) {
	if len(route.Points) != 5 {
		return point{}, false
	}
	start, end := route.Points[0], route.Points[1]
	if !source {
		start, end = route.Points[4], route.Points[3]
	}
	if samePoint(start, end) {
		return point{}, false
	}
	return pointAlongLine(start, end, 0.55), true
}

func renderEndpointAddress(out *bytes.Buffer, endpoint point, address, side string, lane int, color string) {
	if address == "" {
		return
	}
	x := endpoint.X
	y := endpoint.Y - 33
	if side == "bottom" {
		y = endpoint.Y + 46
	} else if side == "left" {
		x = endpoint.X - horizontalLabelOffset
		y = endpoint.Y + 17
	} else if side == "right" {
		x = endpoint.X + horizontalLabelOffset
		y = endpoint.Y + 17
	}
	renderLabel(out, x, y, address, color, "middle", 10, false)
}

const horizontalLabelOffset = 55.0

func renderInterfaceLabel(out *bytes.Buffer, x, y float64, label string, style model.InterfaceLabelStyle) {
	renderRotatedInterfaceLabel(out, x, y, label, 0, style)
}

func renderRotatedInterfaceLabel(out *bytes.Buffer, x, y float64, label string, rotation int, style model.InterfaceLabelStyle) {
	fill := style.Fill
	if fill == "" {
		fill = "#ffffff"
	}
	color := style.Color
	if color == "" {
		color = "#334155"
	}
	border := style.Border
	if border == "" {
		border = "#cbd5e1"
	}
	radius := style.Radius
	paddingX := style.PaddingX
	paddingY := style.PaddingY
	const size = 11
	width := math.Max(38, float64(len([]rune(label)))*size*0.61+paddingX*2)
	height := size + paddingY*2
	if rotation != 0 {
		fmt.Fprintf(out, `<g class="interface-label-rotation" transform="rotate(%d %.1f %.1f)">`, rotation, x, y-height/2)
	}
	fmt.Fprintf(out, `<rect class="interface-label-badge" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" fill="%s" stroke="%s" stroke-width="1"/>`, x-width/2, y-size-paddingY, width, height, radius, escape(fill), escape(border))
	fmt.Fprintf(out, `<text class="interface-label-text" x="%.1f" y="%.1f" text-anchor="middle" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="%d" font-weight="650">%s</text>`, x, y, escape(color), size, escape(label))
	if rotation != 0 {
		out.WriteString(`</g>`)
	}
}

func renderPortMarker(out *bytes.Buffer, endpoint point, color string, premium bool) {
	filter := ""
	if premium {
		filter = ` filter="url(#portGlow)"`
	}
	fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="3.2" fill="#ffffff" stroke="%s" stroke-width="2"%s/>`, endpoint.X, endpoint.Y, color, filter)
}

func renderCenterLabel(out *bytes.Buffer, start, end point, startSide, endSide, label, color string, index int) {
	positions := []float64{0.42, 0.5, 0.58}
	t := positions[index%len(positions)]
	x := start.X + (end.X-start.X)*t
	y := start.Y + (end.Y-start.Y)*t - 9
	horizontal := (startSide == "left" || startSide == "right") && (endSide == "left" || endSide == "right")
	if horizontal {
		y = start.Y + (end.Y-start.Y)*t + 31
	}
	renderLabel(out, x, y, label, color, "middle", 11, false)
}

func renderRouteLabel(out *bytes.Buffer, location point, horizontal bool, label, color string, index int) {
	offsets := []float64{-18, -36, 18}
	offset := offsets[index%len(offsets)]
	if horizontal {
		location.Y += offset
	} else {
		location.X += offset
	}
	renderLabel(out, location.X, location.Y, label, color, "middle", 11, false)
}

func renderLinkTags(out *bytes.Buffer, start, end point, tags []string, color string, index int) {
	if len(tags) == 0 {
		return
	}
	positions := []float64{0.42, 0.5, 0.58}
	t := positions[index%len(positions)]
	x := start.X + (end.X-start.X)*t
	y := start.Y + (end.Y-start.Y)*t + 14

	widths := make([]float64, len(tags))
	totalWidth := 0.0
	for i, tag := range tags {
		widths[i] = math.Max(35, float64(len([]rune(tag)))*6.1+14)
		totalWidth += widths[i]
	}
	totalWidth += float64(len(tags)-1) * 4
	cursor := x - totalWidth/2
	for i, tag := range tags {
		width := widths[i]
		fmt.Fprintf(out, `<rect x="%.1f" y="%.1f" width="%.1f" height="18" rx="9" fill="%s" fill-opacity=".12" stroke="%s" stroke-opacity=".42"/>`, cursor, y-13, width, color, color)
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="middle" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="9" font-weight="700">%s</text>`, cursor+width/2, y, color, escape(tag))
		cursor += width + 4
	}
}

func renderRouteTags(out *bytes.Buffer, location point, tags []string, color string, index int) {
	if len(tags) == 0 {
		return
	}
	renderLinkTags(out, location, location, tags, color, index)
}

func renderBundleMarker(out *bytes.Buffer, x, y float64, visual *bundleVisual) {
	fmt.Fprintf(out, `<g class="bundle-marker" filter="url(#shadow)">`)
	fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="22" fill="#ffffff" stroke="%s" stroke-width="3"/>`, x, y, visual.Color)
	fmt.Fprintf(out, `<circle cx="%.1f" cy="%.1f" r="14" fill="%s" fill-opacity=".13"/>`, x, y, visual.Color)
	fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="middle" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="10" font-weight="800">%s</text>`, x, y+3.5, visual.Color, escape(spec.DisplayPort(visual.Bundle)))
	out.WriteString(`</g>`)
}

func renderBundleLegend(out *bytes.Buffer, doc *model.Diagram) {
	type legendBundle struct {
		Name  string
		Label string
		Tags  []string
		Count int
	}
	bundles := make(map[string]*legendBundle)
	for _, link := range doc.Links {
		if link.Bundle == "" {
			continue
		}
		entry := bundles[link.Bundle]
		if entry == nil {
			entry = &legendBundle{Name: link.Bundle, Label: link.MiddleLabel(), Tags: link.Tags()}
			bundles[link.Bundle] = entry
		}
		entry.Count++
	}
	if len(bundles) == 0 {
		return
	}
	ids := make([]string, 0, len(bundles))
	for id := range bundles {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	const (
		x         = 78.0
		y         = 1010.0
		width     = 332.0
		rowHeight = 47.0
	)
	out.WriteString(`<g id="bundle-legend">`)
	fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#475569" font-family="Inter,Segoe UI,sans-serif" font-size="11" font-weight="800" letter-spacing="1.5">BUNDLE DETAILS</text>`, x, y)
	fmt.Fprintf(out, `<path d="M%.1f %.1f H%.1f" stroke="#cbd5e1"/>`, x, y+12, x+width)
	for i, id := range ids {
		entry := bundles[id]
		rowY := y + 34 + float64(i)*rowHeight
		detail := fmt.Sprintf("%dx%s", entry.Count, entry.Label)
		if entry.Label == "" {
			detail = fmt.Sprintf("%d links", entry.Count)
		}
		tags := make([]string, 0, len(entry.Tags))
		vlan := ""
		for _, tag := range entry.Tags {
			if tag == entry.Name {
				continue
			}
			if strings.HasPrefix(tag, "VLAN ") {
				vlan = tag
				continue
			}
			tags = append(tags, tag)
		}
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#15803d" font-family="ui-monospace,SFMono-Regular,monospace" font-size="11" font-weight="800">%s</text>`, x, rowY, escape(spec.DisplayPort(entry.Name)))
		fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#334155" font-family="ui-monospace,SFMono-Regular,monospace" font-size="8.5" font-weight="650">%s · %s</text>`, x+48, rowY, escape(detail), escape(strings.Join(tags, " · ")))
		if vlan != "" {
			fmt.Fprintf(out, `<text x="%.1f" y="%.1f" fill="#64748b" font-family="ui-monospace,SFMono-Regular,monospace" font-size="8.5" font-weight="600">%s</text>`, x+48, rowY+15, escape(vlan))
		}
	}
	out.WriteString(`</g>`)
}

func renderLabel(out *bytes.Buffer, x, y float64, label, color, anchor string, size int, strong bool) {
	width := math.Max(38, float64(len([]rune(label)))*float64(size)*0.61+18)
	weight := 500
	if strong {
		weight = 650
	}
	fmt.Fprintf(out, `<rect class="label-mask" x="%.1f" y="%.1f" width="%.1f" height="%d" rx="5" fill="#f8fafc" stroke="#f8fafc" stroke-width="4"/>`, x-width/2, y-float64(size)-5, width, size+12)
	fmt.Fprintf(out, `<text x="%.1f" y="%.1f" text-anchor="%s" fill="%s" font-family="ui-monospace,SFMono-Regular,monospace" font-size="%d" font-weight="%d">%s</text>`, x, y, anchor, color, size, weight, escape(label))
}
