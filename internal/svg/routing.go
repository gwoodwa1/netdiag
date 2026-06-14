package svg

import (
	"fmt"
	"math"
	"strings"
)

type linkRoute struct {
	Points          []point
	Path            string
	Label           point
	LabelHorizontal bool
}

func planDiagonalRoutes(links []routedLink) map[int]linkRoute {
	return planDiagonalRoutesWithClearance(links, 24)
}

func planDiagonalRoutesWithClearance(links []routedLink, clearance float64) map[int]linkRoute {
	const (
		candidateCount = 13
		passes         = 4
	)
	routes := make(map[int]linkRoute, len(links))
	for _, link := range links {
		routes[link.Index] = routedDiagonalRoute(link, 0)
	}
	for pass := 0; pass < passes; pass++ {
		for _, link := range links {
			best := routes[link.Index]
			bestScore := diagonalRouteScore(link, best, links, routes, clearance)
			for lane := 1; lane < candidateCount; lane++ {
				candidate := routedDiagonalRoute(link, lane)
				score := diagonalRouteScore(link, candidate, links, routes, clearance)
				if score < bestScore {
					best, bestScore = candidate, score
				}
			}
			routes[link.Index] = best
		}
	}
	return routes
}

type routedLink struct {
	Index     int
	FromNode  string
	ToNode    string
	Start     point
	End       point
	StartSide string
	EndSide   string
	StartStub float64
	EndStub   float64
}

func routedDiagonalRoute(link routedLink, lane int) linkRoute {
	if link.StartStub <= 0 && link.EndStub <= 0 {
		return diagonalRoute(link.Start, link.End, lane)
	}
	startStub := movePoint(link.Start, link.StartSide, link.StartStub)
	endStub := movePoint(link.End, link.EndSide, link.EndStub)
	curve := diagonalRoute(startStub, endStub, lane)
	points := []point{link.Start, startStub, curve.Points[1], endStub, link.End}
	return linkRoute{
		Points: points,
		Path: fmt.Sprintf(
			"M %.1f %.1f L %.1f %.1f Q %.1f %.1f %.1f %.1f L %.1f %.1f",
			link.Start.X, link.Start.Y, startStub.X, startStub.Y,
			curve.Points[1].X, curve.Points[1].Y, endStub.X, endStub.Y,
			link.End.X, link.End.Y,
		),
		Label: curve.Label,
	}
}

func diagonalRouteScore(link routedLink, candidate linkRoute, links []routedLink, routes map[int]linkRoute, clearance float64) float64 {
	score := math.Abs(diagonalRouteOffset(candidate)) * 1.5
	for _, other := range links {
		if other.Index == link.Index || linksMeetAtSamePoint(link, other) {
			continue
		}
		otherRoute := routes[other.Index]
		score += float64(routeIntersectionCount(candidate, otherRoute)) * 100000
		score += routeProximityPenalty(candidate, otherRoute, clearance)
	}
	return score
}

func routeProximityPenalty(a, b linkRoute, clearance float64) float64 {
	aPoints := sampleRoute(a, 16)
	bPoints := sampleRoute(b, 16)
	minimum := math.Inf(1)
	for i := 1; i < len(aPoints); i++ {
		for j := 1; j < len(bPoints); j++ {
			distance := segmentDistance(aPoints[i-1], aPoints[i], bPoints[j-1], bPoints[j])
			minimum = math.Min(minimum, distance)
		}
	}
	if minimum >= clearance {
		return 0
	}
	gap := clearance - minimum
	return gap * gap * 0.75
}

func segmentDistance(a, b, c, d point) float64 {
	if segmentsCross(a, b, c, d) {
		return 0
	}
	return math.Min(
		math.Min(pointSegmentDistance(a, c, d), pointSegmentDistance(b, c, d)),
		math.Min(pointSegmentDistance(c, a, b), pointSegmentDistance(d, a, b)),
	)
}

func pointSegmentDistance(value, start, end point) float64 {
	dx, dy := end.X-start.X, end.Y-start.Y
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return math.Hypot(value.X-start.X, value.Y-start.Y)
	}
	position := ((value.X-start.X)*dx + (value.Y-start.Y)*dy) / lengthSquared
	position = math.Max(0, math.Min(1, position))
	closest := point{X: start.X + position*dx, Y: start.Y + position*dy}
	return math.Hypot(value.X-closest.X, value.Y-closest.Y)
}

func linksMeetAtSamePoint(a, b routedLink) bool {
	return a.FromNode == b.FromNode && samePoint(a.Start, b.Start) ||
		a.FromNode == b.ToNode && samePoint(a.Start, b.End) ||
		a.ToNode == b.FromNode && samePoint(a.End, b.Start) ||
		a.ToNode == b.ToNode && samePoint(a.End, b.End)
}

func diagonalRouteOffset(route linkRoute) float64 {
	if len(route.Points) == 5 {
		middle := pointAlongLine(route.Points[1], route.Points[3], 0.5)
		return math.Hypot(route.Points[2].X-middle.X, route.Points[2].Y-middle.Y)
	}
	if len(route.Points) != 3 {
		return 0
	}
	start, control, end := route.Points[0], route.Points[1], route.Points[2]
	middle := pointAlongLine(start, end, 0.5)
	return math.Hypot(control.X-middle.X, control.Y-middle.Y)
}

func routeIntersectionCount(a, b linkRoute) int {
	aPoints := sampleRoute(a, 16)
	bPoints := sampleRoute(b, 16)
	count := 0
	for i := 1; i < len(aPoints); i++ {
		for j := 1; j < len(bPoints); j++ {
			if segmentsCross(aPoints[i-1], aPoints[i], bPoints[j-1], bPoints[j]) {
				count++
			}
		}
	}
	return count
}

func sampleRoute(route linkRoute, segments int) []point {
	if len(route.Points) == 5 && strings.Contains(route.Path, " Q ") {
		result := []point{route.Points[0], route.Points[1]}
		for index := 1; index < segments; index++ {
			result = append(result, quadraticPoint(route.Points[1], route.Points[2], route.Points[3], float64(index)/float64(segments)))
		}
		return append(result, route.Points[3], route.Points[4])
	}
	if len(route.Points) != 3 || !strings.Contains(route.Path, " Q ") {
		return route.Points
	}
	result := make([]point, 0, segments+1)
	for index := 0; index <= segments; index++ {
		result = append(result, quadraticPoint(route.Points[0], route.Points[1], route.Points[2], float64(index)/float64(segments)))
	}
	return result
}

func segmentsCross(a, b, c, d point) bool {
	const epsilon = 0.001
	abC := crossProduct(a, b, c)
	abD := crossProduct(a, b, d)
	cdA := crossProduct(c, d, a)
	cdB := crossProduct(c, d, b)
	if ((abC > epsilon && abD < -epsilon) || (abC < -epsilon && abD > epsilon)) &&
		((cdA > epsilon && cdB < -epsilon) || (cdA < -epsilon && cdB > epsilon)) {
		return true
	}
	return math.Abs(abC) <= epsilon && pointOnSegment(c, a, b) ||
		math.Abs(abD) <= epsilon && pointOnSegment(d, a, b) ||
		math.Abs(cdA) <= epsilon && pointOnSegment(a, c, d) ||
		math.Abs(cdB) <= epsilon && pointOnSegment(b, c, d)
}

func crossProduct(a, b, c point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func pointOnSegment(value, start, end point) bool {
	const epsilon = 0.001
	return value.X >= math.Min(start.X, end.X)-epsilon && value.X <= math.Max(start.X, end.X)+epsilon &&
		value.Y >= math.Min(start.Y, end.Y)-epsilon && value.Y <= math.Max(start.Y, end.Y)+epsilon
}

func orthogonalRoute(start, end point, startSide, endSide string, nodes map[string]placedNode, lane int) linkRoute {
	stub := 42.0 + float64(lane%4)*12
	startStub := movePoint(start, startSide, stub)
	endStub := movePoint(end, endSide, stub)
	margin := 34.0 + float64(lane%5)*18

	minX, minY, maxX, maxY := start.X, start.Y, start.X, start.Y
	for _, node := range nodes {
		minX = math.Min(minX, node.Box.X)
		minY = math.Min(minY, node.Box.Y)
		maxX = math.Max(maxX, node.Box.X+node.Box.W)
		maxY = math.Max(maxY, node.Box.Y+node.Box.H)
	}

	candidates := [][]point{
		{start, startStub, {X: endStub.X, Y: startStub.Y}, endStub, end},
		{start, startStub, {X: startStub.X, Y: endStub.Y}, endStub, end},
		{start, startStub, {X: startStub.X, Y: minY - margin}, {X: endStub.X, Y: minY - margin}, endStub, end},
		{start, startStub, {X: startStub.X, Y: maxY + margin}, {X: endStub.X, Y: maxY + margin}, endStub, end},
		{start, startStub, {X: minX - margin, Y: startStub.Y}, {X: minX - margin, Y: endStub.Y}, endStub, end},
		{start, startStub, {X: maxX + margin, Y: startStub.Y}, {X: maxX + margin, Y: endStub.Y}, endStub, end},
	}

	best := simplifyOrthogonal(candidates[0])
	bestScore := routeScore(best, nodes, start, end)
	for _, candidate := range candidates[1:] {
		candidate = simplifyOrthogonal(candidate)
		score := routeScore(candidate, nodes, start, end)
		if score < bestScore {
			best, bestScore = candidate, score
		}
	}
	label, horizontal := longestSegmentLabel(best)
	return linkRoute{Points: best, Path: orthogonalPath(best), Label: label, LabelHorizontal: horizontal}
}

func directRoute(start, end point, startSide, endSide, style string) linkRoute {
	return linkRoute{Points: []point{start, end}, Path: pathData(start, end, startSide, endSide, style), Label: point{X: (start.X + end.X) / 2, Y: (start.Y + end.Y) / 2}}
}

func diagonalRoute(start, end point, lane int) linkRoute {
	middle := pointAlongLine(start, end, 0.5)
	dx, dy := end.X-start.X, end.Y-start.Y
	length := math.Hypot(dx, dy)
	offset := diagonalLaneOffset(lane)
	if length > 0 {
		middle.X += -dy / length * offset
		middle.Y += dx / length * offset
	}
	return linkRoute{
		Points: []point{start, middle, end},
		Path:   fmt.Sprintf("M %.1f %.1f Q %.1f %.1f %.1f %.1f", start.X, start.Y, middle.X, middle.Y, end.X, end.Y),
		Label:  quadraticPoint(start, middle, end, 0.5),
	}
}

func diagonalLaneOffset(lane int) float64 {
	const spacing = 52.0
	if lane == 0 {
		return 0
	}
	step := float64((lane + 1) / 2)
	if lane%2 == 0 {
		step = -step
	}
	return step * spacing
}

func pointAlongLine(start, end point, position float64) point {
	return point{
		X: start.X + (end.X-start.X)*position,
		Y: start.Y + (end.Y-start.Y)*position,
	}
}

func quadraticPoint(start, control, end point, position float64) point {
	inverse := 1 - position
	return point{
		X: inverse*inverse*start.X + 2*inverse*position*control.X + position*position*end.X,
		Y: inverse*inverse*start.Y + 2*inverse*position*control.Y + position*position*end.Y,
	}
}

func pointAlongRoute(route linkRoute, position float64) point {
	if len(route.Points) == 5 && strings.Contains(route.Path, " Q ") {
		return pointAlongSampledRoute(sampleRoute(route, 32), position)
	}
	if len(route.Points) == 3 && strings.Contains(route.Path, " Q ") {
		return quadraticPoint(route.Points[0], route.Points[1], route.Points[2], position)
	}
	return pointAlongLine(route.Points[0], route.Points[len(route.Points)-1], position)
}

func pointAlongSampledRoute(points []point, position float64) point {
	if len(points) == 0 {
		return point{}
	}
	total := 0.0
	for index := 1; index < len(points); index++ {
		total += math.Hypot(points[index].X-points[index-1].X, points[index].Y-points[index-1].Y)
	}
	target := total * position
	walked := 0.0
	for index := 1; index < len(points); index++ {
		start, end := points[index-1], points[index]
		length := math.Hypot(end.X-start.X, end.Y-start.Y)
		if walked+length >= target {
			return pointAlongLine(start, end, (target-walked)/length)
		}
		walked += length
	}
	return points[len(points)-1]
}

func movePoint(value point, side string, distance float64) point {
	switch side {
	case "top":
		value.Y -= distance
	case "right":
		value.X += distance
	case "bottom":
		value.Y += distance
	case "left":
		value.X -= distance
	}
	return value
}

func simplifyOrthogonal(points []point) []point {
	var result []point
	for _, current := range points {
		if len(result) > 0 && samePoint(result[len(result)-1], current) {
			continue
		}
		if len(result) >= 2 {
			a, b := result[len(result)-2], result[len(result)-1]
			if (a.X == b.X && b.X == current.X) || (a.Y == b.Y && b.Y == current.Y) {
				result[len(result)-1] = current
				continue
			}
		}
		result = append(result, current)
	}
	return result
}

func routeScore(points []point, nodes map[string]placedNode, start, end point) float64 {
	score := float64(len(points)-2) * 35
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		score += math.Abs(a.X-b.X) + math.Abs(a.Y-b.Y)
		for _, node := range nodes {
			if pointOnBoxBoundary(start, node.Box) || pointOnBoxBoundary(end, node.Box) {
				continue
			}
			if segmentIntersectsBox(a, b, expandBox(node.Box, 22)) {
				score += 100000
			}
		}
	}
	return score
}

func segmentIntersectsBox(a, b point, obstacle box) bool {
	if a.X == b.X {
		return a.X >= obstacle.X && a.X <= obstacle.X+obstacle.W &&
			math.Max(a.Y, b.Y) >= obstacle.Y && math.Min(a.Y, b.Y) <= obstacle.Y+obstacle.H
	}
	if a.Y == b.Y {
		return a.Y >= obstacle.Y && a.Y <= obstacle.Y+obstacle.H &&
			math.Max(a.X, b.X) >= obstacle.X && math.Min(a.X, b.X) <= obstacle.X+obstacle.W
	}
	return false
}

func pointOnBoxBoundary(value point, b box) bool {
	const tolerance = 0.2
	onX := value.X >= b.X-tolerance && value.X <= b.X+b.W+tolerance
	onY := value.Y >= b.Y-tolerance && value.Y <= b.Y+b.H+tolerance
	return (onX && (math.Abs(value.Y-b.Y) < tolerance || math.Abs(value.Y-(b.Y+b.H)) < tolerance)) ||
		(onY && (math.Abs(value.X-b.X) < tolerance || math.Abs(value.X-(b.X+b.W)) < tolerance))
}

func expandBox(value box, padding float64) box {
	return box{X: value.X - padding, Y: value.Y - padding, W: value.W + padding*2, H: value.H + padding*2}
}

func orthogonalPath(points []point) string {
	if len(points) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "M %.1f %.1f", points[0].X, points[0].Y)
	for i := 1; i < len(points); i++ {
		if points[i].Y == points[i-1].Y {
			fmt.Fprintf(&out, " H %.1f", points[i].X)
		} else if points[i].X == points[i-1].X {
			fmt.Fprintf(&out, " V %.1f", points[i].Y)
		} else {
			fmt.Fprintf(&out, " L %.1f %.1f", points[i].X, points[i].Y)
		}
	}
	return out.String()
}

func polylineMidpoint(points []point) point {
	if len(points) == 0 {
		return point{}
	}
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += math.Abs(points[i].X-points[i-1].X) + math.Abs(points[i].Y-points[i-1].Y)
	}
	target := total / 2
	walked := 0.0
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		length := math.Abs(b.X-a.X) + math.Abs(b.Y-a.Y)
		if walked+length >= target {
			remaining := target - walked
			if a.X == b.X {
				direction := 1.0
				if b.Y < a.Y {
					direction = -1
				}
				return point{X: a.X, Y: a.Y + direction*remaining}
			}
			direction := 1.0
			if b.X < a.X {
				direction = -1
			}
			return point{X: a.X + direction*remaining, Y: a.Y}
		}
		walked += length
	}
	return points[len(points)-1]
}

func longestSegmentLabel(points []point) (point, bool) {
	if len(points) < 2 {
		return polylineMidpoint(points), true
	}
	bestIndex := 1
	bestLength := -1.0
	for i := 1; i < len(points); i++ {
		length := math.Abs(points[i].X-points[i-1].X) + math.Abs(points[i].Y-points[i-1].Y)
		// Endpoint stubs are deliberately deprioritized so labels stay away
		// from interface and address cards.
		if i == 1 || i == len(points)-1 {
			length *= 0.25
		}
		if length > bestLength {
			bestIndex, bestLength = i, length
		}
	}
	a, b := points[bestIndex-1], points[bestIndex]
	return point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}, a.Y == b.Y
}

func samePoint(a, b point) bool {
	return a.X == b.X && a.Y == b.Y
}
