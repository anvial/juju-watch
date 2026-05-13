package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/anvial/juju-watch/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

const (
	appBoxWidth                = 24
	selectedAppBoxWidth        = 40
	applicationIcon            = "⬢"
	cardWidth                  = 28
	storageBoxWidth            = 28
	selectedStorageBoxWidth    = 42
	storageIcon                = "⛁"
	selectedStorageBoxHeight   = 6
	selectedMachineBoxWidth    = 42
	machineIcon                = "💻"
	relationLabelWidth         = 14
	selectedRelationLabelWidth = 32
	selectionSweepLength       = 5
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}
	if m.width < 56 || m.height < 16 {
		return m.styles.Error.Render("terminal is too small for juju-watch")
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	helpView := ""
	if m.showHelp {
		helpView = m.help.View(m.keys)
	}
	searchView := ""
	if m.searching {
		searchView = m.search.View()
	}

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if helpView != "" {
		bodyHeight -= lipgloss.Height(helpView)
	}
	if searchView != "" {
		bodyHeight -= lipgloss.Height(searchView)
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	body := m.renderBody(bodyHeight)
	body = m.renderSSHOverlay(body, m.width, bodyHeight)
	parts := []string{header, body}
	if searchView != "" {
		parts = append(parts, searchView)
	}
	if helpView != "" {
		parts = append(parts, helpView)
	}
	parts = append(parts, footer)
	return strings.Join(parts, "\n")
}

func (m Model) renderHeader() string {
	state := "live"
	if m.paused {
		state = "paused"
	}
	if m.polling {
		state = m.spin.View() + " polling"
	}
	title := fmt.Sprintf("juju-watch  model:%s  view:%s  %s", m.cfg.Model, m.view, state)
	if m.selectedID != "" {
		title += "  selected:" + truncate(m.graph.NodeLabel(m.selectedID), 28)
	}
	return m.styles.Header.Width(m.width).Render(title)
}

func (m Model) renderFooter() string {
	parts := []string{fmt.Sprintf("interval %s", m.cfg.Interval)}
	if !m.lastPollAt.IsZero() {
		parts = append(parts, "last poll "+m.lastPollAt.Format("15:04:05"))
	}
	if m.lastErr != nil {
		parts = append(parts, m.styles.Error.Render("error "+truncate(m.lastErr.Error(), 80)))
	}
	if m.layoutErr != nil {
		parts = append(parts, m.styles.Waiting.Render("layout "+truncate(m.layoutErr.Error(), 80)))
	}
	if scroll := m.scrollStatus(); scroll != "" {
		parts = append(parts, scroll)
	}
	parts = append(parts, "?: help")
	return m.styles.Footer.Width(m.width).Render(strings.Join(parts, "  "))
}

func (m Model) renderBody(height int) string {
	inspectorWidth := 0
	if m.width >= 96 {
		inspectorWidth = 34
	}
	canvasWidth := m.width - inspectorWidth
	if inspectorWidth > 0 {
		canvasWidth--
	}
	if canvasWidth < 1 {
		canvasWidth = 1
	}

	main := m.renderMain(canvasWidth, height)
	if inspectorWidth == 0 {
		return main
	}
	inspector := m.renderInspector(inspectorWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, main, m.styles.Dim.Render("│"), inspector)
}

func (m Model) renderMain(width, height int) string {
	if !m.hasGraph {
		msg := "waiting for first Juju poll"
		if m.lastErr != nil {
			msg = "poll failed: " + m.lastErr.Error()
		}
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, truncate(msg, width-4))
	}
	switch m.view {
	case ViewMachines:
		return m.renderMachines(width, height)
	case ViewProblems:
		return m.renderProblems(width, height)
	case ViewEvents:
		return m.renderEvents(width, height)
	default:
		return m.renderTopology(width, height)
	}
}

func (m Model) renderTopology(width, height int) string {
	m = m.withFitOffset(width, height, m.topologyNodeIDs())
	canvas := NewCanvas(width, height)
	for _, id := range m.graph.EdgeOrder {
		edge := m.graph.Edges[id]
		switch {
		case edge.Type == domain.EdgeRelation && m.graph.Nodes[edge.SourceID].Type == domain.NodeApplication && m.graph.Nodes[edge.TargetID].Type == domain.NodeApplication:
			m.drawRelation(&canvas, edge)
		}
	}
	for _, edge := range m.placementEdges() {
		m.drawPlacement(&canvas, edge)
	}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type == domain.NodeApplication {
			m.drawApplication(&canvas, node)
		}
	}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type == domain.NodeMachine {
			m.drawMachine(&canvas, node)
		}
	}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type == domain.NodeStorage && !m.storageIsAttached(node.ID) {
			m.drawStorage(&canvas, node)
		}
	}
	return canvas.Render()
}

func (m Model) renderMachines(width, height int) string {
	m = m.withFitOffset(width, height, m.machineNodeIDs())
	canvas := NewCanvas(width, height)
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type == domain.NodeMachine {
			m.drawMachine(&canvas, node)
		}
	}
	return canvas.Render()
}

func (m Model) renderProblems(width, height int) string {
	canvas := NewCanvas(width, height)
	ids := []string{}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Status.Interesting() {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.styles.Active.Render("no interesting problems"))
	}
	for index, id := range ids {
		node := m.graph.Nodes[id]
		x := 2 + (index%2)*(cardWidth+4)
		y := 2 + (index/2)*6
		m.drawCompactNode(&canvas, node, x, y)
	}
	return canvas.Render()
}

func (m Model) renderEvents(width, height int) string {
	if len(m.events) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, "no events yet")
	}
	lines := []string{m.styles.Title.Render("Recent changes")}
	limit := height - 1
	if limit > len(m.events) {
		limit = len(m.events)
	}
	for i := 0; i < limit; i++ {
		event := m.events[i]
		when := event.At.Format("15:04:05")
		text := fmt.Sprintf("%s  %-9s  %-20s  %s -> %s  %s", when, event.Kind, truncate(event.Label, 20), event.From, event.To, event.Message)
		if event.Kind == "poll-error" {
			text = m.styles.Error.Render(truncate(text, width-2))
		} else {
			text = truncate(text, width-2)
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

func (m Model) drawApplication(canvas *Canvas, node domain.Node) {
	units := m.unitIDsForApp(node.ID)
	box, ok := m.nodeRenderBoxIn(node.ID, canvas.width, canvas.height)
	if !ok {
		return
	}
	x, y := box.x, box.y
	expanded := box.width > appBoxWidth || box.height > m.applicationCompactHeight(node.ID)
	style := m.nodeStyle(node)
	selected := m.selectedID == node.ID
	canvas.Box(x, y, box.width, box.height, style, selected)
	m.drawBoxSweep(canvas, box, DoubleBorderRunes, selected)
	m.drawApplicationTitle(canvas, x, y, box.width, applicationIcon+" "+node.Label, style)

	status := string(node.Status)
	if node.StatusMessage != "" {
		status += " " + node.StatusMessage
	}
	statusLine := fmt.Sprintf("%s %s", status, StatusSymbol(node.Status))
	canvas.Text(x+2, y+1, truncate(statusLine, box.width-4), m.styles.Status(node.Status))
	row := y + 2
	if expanded {
		detail := m.applicationDetail(node)
		if detail != "" {
			canvas.Text(x+2, row, truncate(detail, box.width-4), m.styles.Dim)
			row++
		}
	}
	canvas.Text(x+2, row, fmt.Sprintf("units: %d", len(units)), m.styles.Dim)
	row++
	for _, unitID := range units {
		if row >= y+box.height-1 {
			break
		}
		unit := m.graph.Nodes[unitID]
		rowStyle := m.styles.Status(unit.Status)
		if m.selectedID == unitID {
			rowStyle = m.styles.Selected
		}
		text := fmt.Sprintf("%s %s", StatusSymbol(unit.Status), unit.Label)
		if expanded && m.selectedID == unitID && unit.StatusMessage != "" {
			text += " " + unit.StatusMessage
		}
		canvas.Text(x+2, row, truncate(text, box.width-4), rowStyle)
		if m.selectedID == unitID {
			m.drawRowSweepMarker(canvas, x+1, row)
		}
		row++

		storageIDs := m.storageIDsForUnit(unitID)
		for index, storageID := range storageIDs {
			if row >= y+box.height-1 {
				break
			}
			storage := m.graph.Nodes[storageID]
			rowStyle := m.styles.Status(storage.Status)
			if m.selectedID == storageID {
				rowStyle = m.styles.Selected
			}
			prefix := "├─"
			if index == len(storageIDs)-1 {
				prefix = "╰─"
			}
			text := fmt.Sprintf("%s %s %s %s %s", prefix, storageIcon, storage.Label, StatusSymbol(storage.Status), string(storage.Status))
			if expanded && m.selectedID == storageID {
				if detail := firstNonEmpty(storage.Metadata["kind"], storage.Metadata["location"]); detail != "" {
					text += " " + detail
				}
			}
			canvas.Text(x+4, row, truncate(text, box.width-6), rowStyle)
			if m.selectedID == storageID {
				m.drawRowSweepMarker(canvas, x+3, row)
			}
			row++
		}
	}
}

func (m Model) drawMachine(canvas *Canvas, node domain.Node) {
	units := m.unitIDsForMachine(node.ID)
	box, ok := m.nodeRenderBoxIn(node.ID, canvas.width, canvas.height)
	if !ok {
		return
	}
	x, y := box.x, box.y
	expanded := box.width > appBoxWidth+4 || box.height > max(5, 5+len(units))
	style := m.nodeStyle(node)
	selected := m.selectedID == node.ID
	canvas.Box(x, y, box.width, box.height, style, selected)
	m.drawBoxSweep(canvas, box, DoubleBorderRunes, selected)
	m.drawApplicationTitle(canvas, x, y, box.width, machineIcon+" "+node.Label, style)
	status := string(node.Status)
	if node.StatusMessage != "" {
		status += " " + node.StatusMessage
	}
	canvas.Text(x+2, y+1, truncate(fmt.Sprintf("%s %s", status, StatusSymbol(node.Status)), box.width-4), m.styles.Status(node.Status))
	row := y + 2
	if addr := node.Metadata["ip_address"]; addr != "" {
		canvas.Text(x+2, row, truncate(addr, box.width-4), m.styles.Dim)
		row++
	}
	if expanded {
		detail := m.machineDetail(node)
		if detail != "" && row < y+box.height-1 {
			canvas.Text(x+2, row, truncate(detail, box.width-4), m.styles.Dim)
			row++
		}
	}
	for index, unitID := range units {
		if row+index >= y+box.height-1 {
			break
		}
		unit := m.graph.Nodes[unitID]
		rowStyle := m.styles.Status(unit.Status)
		if m.selectedID == unitID {
			rowStyle = m.styles.Selected
		}
		canvas.Text(x+2, row+index, truncate(StatusSymbol(unit.Status)+" "+unit.Label, box.width-4), rowStyle)
		if m.selectedID == unitID {
			m.drawRowSweepMarker(canvas, x+1, row+index)
		}
	}
}

func (m Model) drawCompactNode(canvas *Canvas, node domain.Node, x, y int) {
	style := m.nodeStyle(node)
	selected := m.selectedID == node.ID
	canvas.Box(x, y, cardWidth, 5, style, selected)
	m.drawBoxSweep(canvas, renderBox{x: x, y: y, width: cardWidth, height: 5}, DoubleBorderRunes, selected)
	canvas.Text(x+2, y+1, truncate(node.Label+" "+StatusSymbol(node.Status), cardWidth-4), style)
	canvas.Text(x+2, y+2, truncate(string(node.Type), cardWidth-4), m.styles.Dim)
	canvas.Text(x+2, y+3, truncate(string(node.Status)+" "+node.StatusMessage, cardWidth-4), m.styles.Status(node.Status))
}

func (m Model) drawStorage(canvas *Canvas, node domain.Node) {
	box, ok := m.nodeRenderBoxIn(node.ID, canvas.width, canvas.height)
	if !ok {
		return
	}
	x, y := box.x, box.y
	style := m.nodeStyle(node)
	border := RoundedBorderRunes
	if m.selectedID == node.ID {
		border = HeavyBorderRunes
	}
	canvas.BoxWithBorder(x, y, box.width, box.height, style, border)
	m.drawBoxSweep(canvas, box, border, m.selectedID == node.ID)
	m.drawStorageTitle(canvas, x, y, box.width, storageIcon+" "+node.Label, style, border.Horizontal)

	detail := firstNonEmpty(node.Metadata["location"], node.Metadata["unit"], node.Metadata["kind"])
	if detail == "" {
		detail = "storage"
	}
	canvas.Text(x+2, y+1, truncate(detail, box.width-4), m.styles.Dim)

	footer := strings.TrimSpace(node.Metadata["kind"] + " " + StatusSymbol(node.Status) + " " + string(node.Status))
	if box.height > 4 {
		canvas.Text(x+2, y+2, truncate("unit: "+firstNonEmpty(node.Metadata["unit"], "unattached"), box.width-4), m.styles.Dim)
		canvas.Text(x+2, y+3, truncate("kind: "+firstNonEmpty(node.Metadata["kind"], "storage"), box.width-4), m.styles.Dim)
		canvas.Text(x+2, y+4, truncate("status: "+string(node.Status)+" "+node.StatusMessage, box.width-4), m.styles.Status(node.Status))
		return
	}
	canvas.Text(x+2, y+2, truncate(footer, box.width-4), m.styles.Status(node.Status))
}

func (m Model) drawApplicationTitle(canvas *Canvas, x, y, width int, title string, style lipgloss.Style) {
	if width < 6 {
		return
	}
	title = truncate(title+" ", width-4)
	canvas.Text(x+2, y, title, style)
}

func (m Model) drawStorageTitle(canvas *Canvas, x, y, width int, title string, style lipgloss.Style, horizontal rune) {
	if width < 8 {
		return
	}
	canvas.Put(x+2, y, '╼', style)
	canvas.Text(x+4, y, truncate(title, width-8), style)
	titleWidth := len([]rune(truncate(title, width-8)))
	rightCap := x + 5 + titleWidth
	if rightCap < x+width-2 {
		canvas.Put(rightCap, y, '╾', style)
		canvas.HLine(rightCap+1, x+width-2, y, horizontal, style)
	}
}

func (m Model) drawRelation(canvas *Canvas, edge domain.Edge) {
	route, ok := m.relationRoute(edge, canvas.width, canvas.height)
	if !ok {
		return
	}
	style := m.styles.Relation
	if m.selectedID == edge.ID {
		style = m.styles.RelationSelected
	}
	for _, segment := range route.segments() {
		if segment.from.y == segment.to.y {
			canvas.HLine(segment.from.x, segment.to.x, segment.from.y, '━', style)
			continue
		}
		if segment.from.x == segment.to.x {
			canvas.VLine(segment.from.x, segment.from.y, segment.to.y, '┃', style)
		}
	}
	for _, point := range route.joints() {
		canvas.Put(point.x, point.y, '╋', style)
	}
	canvas.Put(route.arrowAt.x, route.arrowAt.y, route.arrow, style)
	if m.selectedID == edge.ID {
		m.drawRouteSweep(canvas, route, relationSweepGlyphs(), true)
	}
	if route.label != "" && route.labelAt != nil {
		canvas.Text(route.labelAt.x, route.labelAt.y, route.label, style)
	}
}

type placementEdge struct {
	appID      string
	unitID     string
	machineID  string
	label      string
	status     domain.Status
	groupIndex int
	groupSize  int
}

type routePoint struct {
	x int
	y int
}

type routeSegment struct {
	from routePoint
	to   routePoint
}

type relationRoute struct {
	points  []routePoint
	arrow   rune
	arrowAt routePoint
	label   string
	labelAt *routePoint
}

type relationAnchor struct {
	point routePoint
	arrow rune
}

type sweepCell struct {
	point routePoint
	ch    rune
}

type routeGlyphs struct {
	horizontal rune
	vertical   rune
	joint      func(relationRoute, routePoint) rune
}

func relationSweepGlyphs() routeGlyphs {
	return routeGlyphs{
		horizontal: '━',
		vertical:   '┃',
		joint: func(relationRoute, routePoint) rune {
			return '╋'
		},
	}
}

func placementSweepGlyphs() routeGlyphs {
	return routeGlyphs{
		horizontal: '─',
		vertical:   '│',
		joint:      placementJoint,
	}
}

func (m Model) drawBoxSweep(canvas *Canvas, box renderBox, border BorderRunes, selected bool) {
	if !selected || !m.selectionAnimationActive() {
		return
	}
	for _, cell := range boxSweepCells(box, border, m.selectionFrame) {
		canvas.Put(cell.point.x, cell.point.y, cell.ch, m.styles.SelectionSweep)
	}
}

func (m Model) drawRouteSweep(canvas *Canvas, route relationRoute, glyphs routeGlyphs, active bool) {
	if !active || !m.selectionAnimationActive() {
		return
	}
	for _, cell := range routeSweepCells(route, glyphs, m.selectionFrame) {
		canvas.Put(cell.point.x, cell.point.y, cell.ch, m.styles.SelectionSweep)
	}
}

func (m Model) drawRowSweepMarker(canvas *Canvas, x, y int) {
	if !m.selectionAnimationActive() {
		canvas.Put(x, y, '▸', m.styles.Selected)
		return
	}
	markers := []rune{'▸', '▸', '▶', '▸'}
	canvas.Put(x, y, markers[(m.selectionFrame/4)%len(markers)], m.styles.SelectionSweep)
}

func boxSweepCells(box renderBox, border BorderRunes, frame int) []sweepCell {
	return sweepCells(boxBorderPath(box, border), frame)
}

func boxBorderPath(box renderBox, border BorderRunes) []sweepCell {
	if box.width < 2 || box.height < 2 {
		return nil
	}
	right := box.x + box.width - 1
	bottom := box.y + box.height - 1
	path := []sweepCell{}
	for x := box.x; x <= right; x++ {
		ch := border.Horizontal
		if x == box.x {
			ch = border.TopLeft
		} else if x == right {
			ch = border.TopRight
		}
		path = append(path, sweepCell{point: routePoint{x: x, y: box.y}, ch: ch})
	}
	for y := box.y + 1; y <= bottom-1; y++ {
		path = append(path, sweepCell{point: routePoint{x: right, y: y}, ch: border.Vertical})
	}
	for x := right; x >= box.x; x-- {
		ch := border.Horizontal
		if x == right {
			ch = border.BottomRight
		} else if x == box.x {
			ch = border.BottomLeft
		}
		path = append(path, sweepCell{point: routePoint{x: x, y: bottom}, ch: ch})
	}
	for y := bottom - 1; y >= box.y+1; y-- {
		path = append(path, sweepCell{point: routePoint{x: box.x, y: y}, ch: border.Vertical})
	}
	return path
}

func routeSweepCells(route relationRoute, glyphs routeGlyphs, frame int) []sweepCell {
	return sweepCells(routePathCells(route, glyphs), frame)
}

func routePathCells(route relationRoute, glyphs routeGlyphs) []sweepCell {
	points := routePathPoints(route)
	cells := make([]sweepCell, 0, len(points))
	for index, point := range points {
		cells = append(cells, sweepCell{point: point, ch: routePathGlyph(route, points, index, glyphs)})
	}
	return cells
}

func routePathPoints(route relationRoute) []routePoint {
	points := []routePoint{}
	for i := 0; i < len(route.points)-1; i++ {
		from := route.points[i]
		to := route.points[i+1]
		if from == to {
			continue
		}
		if len(points) == 0 {
			points = append(points, from)
		}
		current := from
		stepX := sign(to.x - from.x)
		stepY := sign(to.y - from.y)
		for current != to {
			if current.x != to.x {
				current.x += stepX
			} else if current.y != to.y {
				current.y += stepY
			} else {
				break
			}
			points = append(points, current)
		}
	}
	return points
}

func routePathGlyph(route relationRoute, points []routePoint, index int, glyphs routeGlyphs) rune {
	point := points[index]
	if point == route.arrowAt {
		return route.arrow
	}
	for _, joint := range route.joints() {
		if point == joint {
			return glyphs.joint(route, point)
		}
	}
	vertical := (index > 0 && points[index-1].x == point.x && points[index-1].y != point.y) ||
		(index+1 < len(points) && points[index+1].x == point.x && points[index+1].y != point.y)
	if vertical {
		return glyphs.vertical
	}
	return glyphs.horizontal
}

func sweepCells(path []sweepCell, frame int) []sweepCell {
	if len(path) == 0 {
		return nil
	}
	start := frame % len(path)
	if start < 0 {
		start += len(path)
	}
	length := min(selectionSweepLength, len(path))
	cells := make([]sweepCell, 0, length)
	for i := 0; i < length; i++ {
		cells = append(cells, path[(start+i)%len(path)])
	}
	return cells
}

func (r relationRoute) segments() []routeSegment {
	segments := []routeSegment{}
	for i := 0; i < len(r.points)-1; i++ {
		if r.points[i] == r.points[i+1] {
			continue
		}
		segments = append(segments, routeSegment{from: r.points[i], to: r.points[i+1]})
	}
	return segments
}

func (r relationRoute) joints() []routePoint {
	if len(r.points) <= 2 {
		return nil
	}
	return append([]routePoint{}, r.points[1:len(r.points)-1]...)
}

func (m Model) relationRoute(edge domain.Edge, width, height int) (relationRoute, bool) {
	sourceBox, ok := m.nodeRenderBoxIn(edge.SourceID, width, height)
	if !ok {
		return relationRoute{}, false
	}
	targetBox, ok := m.nodeRenderBoxIn(edge.TargetID, width, height)
	if !ok {
		return relationRoute{}, false
	}

	boxes := m.topologyRenderBoxesIn(width, height)
	if route, ok := m.straightRelationRoute(edge, sourceBox, targetBox, boxes, width, height); ok {
		return route, true
	}

	labelLimit := m.relationLabelLimit(edge)
	wantedLabel := truncate(strings.TrimSpace(edge.Label), labelLimit)
	bestRoute := relationRoute{}
	bestScore := -1
	considerRoute := func(route relationRoute) bool {
		if !routeClear(route, boxes, width, height) {
			return false
		}
		route.label, route.labelAt = relationLabel(edge.Label, route, boxes, labelLimit)
		score := relationRouteLabelScore(route, wantedLabel)
		if score > bestScore {
			bestRoute = route
			bestScore = score
		}
		return wantedLabel != "" && route.label == wantedLabel
	}

	for _, start := range sourceRelationAnchors(sourceBox, targetBox) {
		for _, target := range targetRelationAnchors(targetBox, sourceBox) {
			for _, laneY := range relationLaneCandidates(start.y, target.point.y, sourceBox, targetBox, height) {
				points := cleanRoutePoints([]routePoint{
					start,
					{x: start.x, y: laneY},
					{x: target.point.x, y: laneY},
					target.point,
				})
				route := relationRoute{
					points:  points,
					arrow:   target.arrow,
					arrowAt: target.point,
				}
				if considerRoute(route) {
					return bestRoute, true
				}
			}
		}
	}
	if bestScore >= 0 {
		return bestRoute, true
	}

	leftToRight := sourceBox.x+sourceBox.width/2 <= targetBox.x+targetBox.width/2
	start := sourceRelationAnchors(sourceBox, targetBox)[0]
	arrow := '▶'
	if !leftToRight {
		arrow = '◀'
	}
	return m.relationFallbackRoute(edge, start, arrow, leftToRight, boxes, width, height)
}

func (m Model) straightRelationRoute(edge domain.Edge, sourceBox, targetBox renderBox, boxes []renderBox, width, height int) (relationRoute, bool) {
	y := relationAnchorY(sourceBox)
	if y != relationAnchorY(targetBox) {
		return relationRoute{}, false
	}
	sourceLeft := sourceBox.x+sourceBox.width/2 <= targetBox.x+targetBox.width/2
	start := routePoint{x: sourceBox.x + sourceBox.width, y: y}
	target := relationAnchor{point: routePoint{x: targetBox.x - 1, y: y}, arrow: '▶'}
	if !sourceLeft {
		start = routePoint{x: sourceBox.x - 1, y: y}
		target = relationAnchor{point: routePoint{x: targetBox.x + targetBox.width, y: y}, arrow: '◀'}
	}
	route := relationRoute{
		points:  cleanRoutePoints([]routePoint{start, target.point}),
		arrow:   target.arrow,
		arrowAt: target.point,
	}
	if !routeClear(route, boxes, width, height) {
		return relationRoute{}, false
	}
	route.label, route.labelAt = relationLabel(edge.Label, route, boxes, m.relationLabelLimit(edge))
	return route, true
}

func (m Model) relationFallbackRoute(edge domain.Edge, start routePoint, arrow rune, leftToRight bool, boxes []renderBox, width, height int) (relationRoute, bool) {
	end := start
	if leftToRight {
		end.x = min(width-2, start.x+8)
	} else {
		end.x = max(1, start.x-8)
	}
	route := relationRoute{
		points:  cleanRoutePoints([]routePoint{start, end}),
		arrow:   arrow,
		arrowAt: end,
	}
	if !routeClear(route, boxes, width, height) {
		return relationRoute{}, false
	}
	route.label, route.labelAt = relationLabel(edge.Label, route, boxes, m.relationLabelLimit(edge))
	return route, true
}

func relationAnchorY(box renderBox) int {
	if box.height <= 3 {
		return box.y + box.height/2
	}
	return box.y + min(2, box.height-2)
}

func sourceRelationAnchors(sourceBox, targetBox renderBox) []routePoint {
	facingRight := sourceBox.x+sourceBox.width/2 <= targetBox.x+targetBox.width/2
	sideY := relationAnchorY(sourceBox)
	top := routePoint{x: sourceBox.x + sourceBox.width/2, y: sourceBox.y - 1}
	bottom := routePoint{x: sourceBox.x + sourceBox.width/2, y: sourceBox.y + sourceBox.height}
	right := routePoint{x: sourceBox.x + sourceBox.width, y: sideY}
	left := routePoint{x: sourceBox.x - 1, y: sideY}
	if facingRight {
		return []routePoint{right, top, bottom, left}
	}
	return []routePoint{left, top, bottom, right}
}

func targetRelationAnchors(targetBox, sourceBox renderBox) []relationAnchor {
	sourceLeft := sourceBox.x+sourceBox.width/2 <= targetBox.x+targetBox.width/2
	sideY := relationAnchorY(targetBox)
	top := relationAnchor{point: routePoint{x: targetBox.x + targetBox.width/2, y: targetBox.y - 1}, arrow: '▼'}
	bottom := relationAnchor{point: routePoint{x: targetBox.x + targetBox.width/2, y: targetBox.y + targetBox.height}, arrow: '▲'}
	left := relationAnchor{point: routePoint{x: targetBox.x - 1, y: sideY}, arrow: '▶'}
	right := relationAnchor{point: routePoint{x: targetBox.x + targetBox.width, y: sideY}, arrow: '◀'}
	if sourceLeft {
		return []relationAnchor{left, top, bottom, right}
	}
	return []relationAnchor{right, top, bottom, left}
}

func relationLaneCandidates(startY, targetY int, sourceBox, targetBox renderBox, height int) []int {
	preferred := []int{
		startY,
		targetY,
		(startY + targetY) / 2,
		min(sourceBox.y, targetBox.y) - 2,
		max(sourceBox.y+sourceBox.height, targetBox.y+targetBox.height) + 1,
	}
	mid := (startY + targetY) / 2
	for delta := 0; delta < height; delta++ {
		preferred = append(preferred, mid-delta, mid+delta)
	}
	seen := map[int]bool{}
	out := []int{}
	for _, y := range preferred {
		if y < 0 || y >= height || seen[y] {
			continue
		}
		seen[y] = true
		out = append(out, y)
	}
	return out
}

func cleanRoutePoints(points []routePoint) []routePoint {
	out := []routePoint{}
	for _, point := range points {
		if len(out) > 0 && out[len(out)-1] == point {
			continue
		}
		out = append(out, point)
	}
	return out
}

func routeLength(route relationRoute) int {
	total := 0
	for _, segment := range route.segments() {
		total += abs(segment.to.x-segment.from.x) + abs(segment.to.y-segment.from.y)
	}
	return total
}

func relationRouteLabelScore(route relationRoute, wantedLabel string) int {
	if route.label == "" {
		return -routeLength(route)
	}
	score := len([]rune(route.label))*1000 - routeLength(route)
	if route.label == wantedLabel {
		score += 100000
	}
	return score
}

func routeClear(route relationRoute, boxes []renderBox, width, height int) bool {
	for _, segment := range route.segments() {
		if !segmentInCanvas(segment, width, height) {
			return false
		}
		for _, box := range boxes {
			if segmentIntersectsBox(segment, box) {
				return false
			}
		}
	}
	return true
}

func segmentInCanvas(segment routeSegment, width, height int) bool {
	return pointInCanvas(segment.from, width, height) && pointInCanvas(segment.to, width, height)
}

func pointInCanvas(point routePoint, width, height int) bool {
	return point.x >= 0 && point.y >= 0 && point.x < width && point.y < height
}

func segmentIntersectsBox(segment routeSegment, box renderBox) bool {
	if segment.from.y == segment.to.y {
		y := segment.from.y
		if y < box.y || y >= box.y+box.height {
			return false
		}
		minX := min(segment.from.x, segment.to.x)
		maxX := max(segment.from.x, segment.to.x)
		return maxX >= box.x && minX < box.x+box.width
	}
	if segment.from.x == segment.to.x {
		x := segment.from.x
		if x < box.x || x >= box.x+box.width {
			return false
		}
		minY := min(segment.from.y, segment.to.y)
		maxY := max(segment.from.y, segment.to.y)
		return maxY >= box.y && minY < box.y+box.height
	}
	return true
}

func relationLabel(label string, route relationRoute, boxes []renderBox, limit int) (string, *routePoint) {
	label = truncate(strings.TrimSpace(label), limit)
	if label == "" {
		return "", nil
	}
	labelWidth := len([]rune(label))
	for _, segment := range route.segments() {
		if segment.from.y != segment.to.y {
			continue
		}
		minX := min(segment.from.x, segment.to.x)
		maxX := max(segment.from.x, segment.to.x)
		available := maxX - minX - 2
		if available < 1 {
			continue
		}
		segmentLabel := label
		segmentLabelWidth := labelWidth
		if available < segmentLabelWidth {
			segmentLabel = truncate(label, available)
			segmentLabelWidth = len([]rune(segmentLabel))
		}
		if segmentLabel == "" {
			continue
		}
		x := minX + 1 + (available-segmentLabelWidth)/2
		y := segment.from.y
		labelSegment := routeSegment{from: routePoint{x: x, y: y}, to: routePoint{x: x + segmentLabelWidth - 1, y: y}}
		blocked := false
		for _, box := range boxes {
			if segmentIntersectsBox(labelSegment, box) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		point := routePoint{x: x, y: y}
		return segmentLabel, &point
	}
	return "", nil
}

func (m Model) drawPlacement(canvas *Canvas, edge placementEdge) {
	route, ok := m.placementRoute(edge, canvas.width, canvas.height)
	if !ok {
		return
	}
	style := m.placementStyle(edge)
	for _, segment := range route.segments() {
		if segment.from.y == segment.to.y {
			canvas.HLine(segment.from.x, segment.to.x, segment.from.y, '─', style)
			continue
		}
		if segment.from.x == segment.to.x {
			canvas.VLine(segment.from.x, segment.from.y, segment.to.y, '│', style)
		}
	}
	for _, point := range route.joints() {
		canvas.Put(point.x, point.y, placementJoint(route, point), style)
	}
	canvas.Put(route.arrowAt.x, route.arrowAt.y, route.arrow, style)
	m.drawRouteSweep(canvas, route, placementSweepGlyphs(), m.placementSweepActive(edge))
	if route.label != "" && route.labelAt != nil {
		canvas.Text(route.labelAt.x, route.labelAt.y, route.label, style)
	}
}

func (m Model) placementStyle(edge placementEdge) lipgloss.Style {
	if m.selectedID == edge.unitID {
		return m.styles.PlacementSelected
	}
	style := m.styles.PlacementPalette(edge.groupIndex)
	if m.selectedID == edge.machineID || m.selectedID == edge.appID {
		style = style.Bold(true)
	}
	return style
}

func placementJoint(route relationRoute, point routePoint) rune {
	for i := 1; i < len(route.points)-1; i++ {
		if route.points[i] != point {
			continue
		}
		prev := route.points[i-1]
		next := route.points[i+1]
		left := prev.x < point.x || next.x < point.x
		right := prev.x > point.x || next.x > point.x
		up := prev.y < point.y || next.y < point.y
		down := prev.y > point.y || next.y > point.y
		switch {
		case right && down:
			return '╭'
		case left && down:
			return '╮'
		case right && up:
			return '╰'
		case left && up:
			return '╯'
		case left || right:
			return '─'
		case up || down:
			return '│'
		}
	}
	return '─'
}

func (m Model) placementRoute(edge placementEdge, width, height int) (relationRoute, bool) {
	sourceBox, ok := m.nodeRenderBoxIn(edge.appID, width, height)
	if !ok {
		return relationRoute{}, false
	}
	targetBox, ok := m.nodeRenderBoxIn(edge.machineID, width, height)
	if !ok {
		return relationRoute{}, false
	}
	sourceY, ok := m.applicationUnitRow(edge.appID, edge.unitID, sourceBox)
	if !ok {
		return relationRoute{}, false
	}
	targetY, ok := m.machineUnitRow(edge.machineID, edge.unitID, targetBox)
	if !ok {
		return relationRoute{}, false
	}
	sourceLeft := sourceBox.x+sourceBox.width/2 <= targetBox.x+targetBox.width/2
	start := routePoint{x: sourceBox.x + sourceBox.width, y: sourceY}
	target := routePoint{x: targetBox.x - 1, y: targetY}
	arrow := '▶'
	midpointX := (start.x + target.x) / 2
	if !sourceLeft {
		start = routePoint{x: sourceBox.x - 1, y: sourceY}
		target = routePoint{x: targetBox.x + targetBox.width, y: targetY}
		arrow = '◀'
		midpointX = (target.x + start.x) / 2
	}
	boxes := m.topologyRenderBoxesIn(width, height)

	for _, laneX := range placementLaneXCandidates(midpointX, edge.groupIndex, edge.groupSize, width) {
		route := relationRoute{
			points: cleanRoutePoints([]routePoint{
				start,
				{x: laneX, y: start.y},
				{x: laneX, y: target.y},
				target,
			}),
			arrow:   arrow,
			arrowAt: target,
		}
		if !routeClear(route, boxes, width, height) {
			continue
		}
		route.label, route.labelAt = placementLabel(edge.label, route, boxes, width, height)
		return route, true
	}

	for _, laneY := range placementLaneYCandidates(start.y, target.y, sourceBox, targetBox, edge.groupIndex, edge.groupSize, height) {
		route := relationRoute{
			points: cleanRoutePoints([]routePoint{
				start,
				{x: start.x, y: laneY},
				{x: target.x, y: laneY},
				target,
			}),
			arrow:   arrow,
			arrowAt: target,
		}
		if !routeClear(route, boxes, width, height) {
			continue
		}
		route.label, route.labelAt = placementLabel(edge.label, route, boxes, width, height)
		return route, true
	}
	return relationRoute{}, false
}

func placementLabel(label string, route relationRoute, boxes []renderBox, width, height int) (string, *routePoint) {
	wanted := truncate(strings.TrimSpace(label), 12)
	if fitted, at := relationLabel(label, route, boxes, 12); fitted == wanted && at != nil {
		return fitted, at
	}
	label = wanted
	if label == "" {
		return "", nil
	}
	labelWidth := len([]rune(label))
	for _, segment := range route.segments() {
		if segment.from.x != segment.to.x {
			continue
		}
		minY := min(segment.from.y, segment.to.y)
		maxY := max(segment.from.y, segment.to.y)
		y := minY + (maxY-minY)/2
		for _, x := range []int{segment.from.x + 2, segment.from.x - labelWidth - 2} {
			if x < 0 || x+labelWidth >= width || y < 0 || y >= height {
				continue
			}
			labelSegment := routeSegment{from: routePoint{x: x, y: y}, to: routePoint{x: x + labelWidth - 1, y: y}}
			blocked := false
			for _, box := range boxes {
				if segmentIntersectsBox(labelSegment, box) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			point := routePoint{x: x, y: y}
			return label, &point
		}
	}
	return "", nil
}

func placementLaneXCandidates(midpoint, groupIndex, groupSize, width int) []int {
	return placementStaggeredLaneCandidates([]int{midpoint}, groupIndex, groupSize, width)
}

func placementLaneYCandidates(startY, targetY int, sourceBox, targetBox renderBox, groupIndex, groupSize, height int) []int {
	mid := (startY + targetY) / 2
	preferred := []int{
		min(sourceBox.y, targetBox.y) - 2,
		max(sourceBox.y+sourceBox.height, targetBox.y+targetBox.height) + 1,
		mid,
		startY + 1,
		targetY - 1,
	}
	for delta := 0; delta < height; delta++ {
		preferred = append(preferred, mid-delta, mid+delta)
	}
	return placementStaggeredLaneCandidates(preferred, groupIndex, groupSize, height)
}

func placementStaggeredLaneCandidates(bases []int, groupIndex, groupSize, limit int) []int {
	if limit <= 0 {
		return nil
	}
	seen := map[int]bool{}
	out := []int{}
	maxRadius := limit
	for _, base := range bases {
		for _, lane := range placementLanesAround(base, groupIndex, groupSize, maxRadius) {
			if lane < 0 || lane >= limit || seen[lane] {
				continue
			}
			seen[lane] = true
			out = append(out, lane)
		}
	}
	return out
}

func placementLanesAround(base, groupIndex, groupSize, maxRadius int) []int {
	offset := placementGroupOffset(groupIndex, groupSize)
	if groupSize <= 1 {
		out := []int{base}
		for radius := 1; radius <= maxRadius; radius++ {
			out = append(out, base-radius, base+radius)
		}
		return out
	}
	stride := groupSize * 2
	maxSteps := maxRadius/stride + 2
	offsets := []int{offset}
	for step := 1; step <= maxSteps; step++ {
		right := offset + step*stride
		left := offset - step*stride
		if abs(right) <= abs(left) {
			offsets = append(offsets, right, left)
		} else {
			offsets = append(offsets, left, right)
		}
	}
	out := make([]int, 0, len(offsets))
	for _, candidateOffset := range offsets {
		out = append(out, base+candidateOffset)
	}
	return out
}

func placementGroupOffset(groupIndex, groupSize int) int {
	if groupSize <= 1 {
		return 0
	}
	if groupIndex < 0 {
		groupIndex = 0
	}
	if groupIndex >= groupSize {
		groupIndex = groupSize - 1
	}
	return 2*groupIndex - (groupSize - 1)
}

func (m Model) renderInspector(width, height int) string {
	lines := []string{m.styles.Title.Render("Inspector")}
	if !m.hasGraph {
		lines = append(lines, "No state yet")
		return m.styles.Panel.Width(width - 2).Height(height - 2).Render(strings.Join(lines, "\n"))
	}
	if m.selectedID == "" {
		lines = append(lines, "No selection")
		return m.styles.Panel.Width(width - 2).Height(height - 2).Render(strings.Join(lines, "\n"))
	}
	if node, ok := m.graph.Nodes[m.selectedID]; ok {
		lines = append(lines, m.nodeInspectorLines(node, width-4)...)
	} else if edge, ok := m.graph.Edges[m.selectedID]; ok {
		lines = append(lines, m.edgeInspectorLines(edge, width-4)...)
	}
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}
	return m.styles.Panel.Width(width - 2).Height(height - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) nodeInspectorLines(node domain.Node, width int) []string {
	lines := []string{
		truncate(node.Label, width),
		"type: " + string(node.Type),
		"id: " + truncate(node.ID, width-4),
		"status: " + string(node.Status),
	}
	if node.StatusMessage != "" {
		lines = append(lines, "message: "+truncate(node.StatusMessage, width-9))
	}
	keys := make([]string, 0, len(node.Metadata))
	for key := range node.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if node.Metadata[key] == "" {
			continue
		}
		lines = append(lines, truncate(key+": "+node.Metadata[key], width))
	}
	neighbors := m.graph.Neighbors(node.ID)
	if len(neighbors) > 0 {
		lines = append(lines, "", "neighbors:")
		for _, neighbor := range neighbors {
			lines = append(lines, "  "+truncate(m.graph.NodeLabel(neighbor), width-2))
		}
	}
	recent := m.recentEventsFor(node.ID, 4)
	if len(recent) > 0 {
		lines = append(lines, "", "recent:")
		for _, event := range recent {
			lines = append(lines, "  "+truncate(event.Kind+" "+event.From+"->"+event.To, width-2))
		}
	}
	return lines
}

func (m Model) edgeInspectorLines(edge domain.Edge, width int) []string {
	lines := []string{
		truncate(edge.Label, width),
		"type: " + string(edge.Type),
		"id: " + truncate(edge.ID, width-4),
		"source: " + truncate(m.graph.NodeLabel(edge.SourceID), width-8),
		"target: " + truncate(m.graph.NodeLabel(edge.TargetID), width-8),
	}
	keys := make([]string, 0, len(edge.Metadata))
	for key := range edge.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, truncate(key+": "+edge.Metadata[key], width))
	}
	return lines
}

func (m Model) recentEventsFor(id string, max int) []domain.Event {
	events := []domain.Event{}
	for _, event := range m.events {
		if event.ObjectID == id {
			events = append(events, event)
			if len(events) == max {
				break
			}
		}
	}
	return events
}

func (m Model) unitIDsForApp(appID string) []string {
	ids := []string{}
	for _, edge := range m.graph.Edges {
		if edge.Type == domain.EdgeAppHasUnit && edge.SourceID == appID {
			ids = append(ids, edge.TargetID)
		}
	}
	sort.Strings(ids)
	return ids
}

func (m Model) unitIDsForMachine(machineID string) []string {
	ids := []string{}
	for _, edge := range m.graph.Edges {
		if edge.Type == domain.EdgeUnitOnMachine && edge.TargetID == machineID {
			ids = append(ids, edge.SourceID)
		}
	}
	sort.Strings(ids)
	return ids
}

func (m Model) storageIDsForUnit(unitID string) []string {
	ids := []string{}
	for _, edge := range m.graph.Edges {
		if edge.Type == domain.EdgeStorageAttached && edge.SourceID == unitID {
			ids = append(ids, edge.TargetID)
		}
	}
	sort.Strings(ids)
	return ids
}

func (m Model) placementEdges() []placementEdge {
	edges := []placementEdge{}
	for _, edge := range m.graph.Edges {
		if edge.Type != domain.EdgeUnitOnMachine {
			continue
		}
		appID := m.appIDForUnit(edge.SourceID)
		if appID == "" {
			continue
		}
		unit := m.graph.Nodes[edge.SourceID]
		edges = append(edges, placementEdge{
			appID:     appID,
			unitID:    edge.SourceID,
			machineID: edge.TargetID,
			label:     unit.Label,
			status:    unit.Status,
		})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].appID != edges[j].appID {
			return edges[i].appID < edges[j].appID
		}
		if edges[i].machineID != edges[j].machineID {
			return edges[i].machineID < edges[j].machineID
		}
		return edges[i].unitID < edges[j].unitID
	})
	for start := 0; start < len(edges); {
		end := start + 1
		for end < len(edges) && edges[end].appID == edges[start].appID && edges[end].machineID == edges[start].machineID {
			end++
		}
		groupSize := end - start
		for index := start; index < end; index++ {
			edges[index].groupIndex = index - start
			edges[index].groupSize = groupSize
		}
		start = end
	}
	return edges
}

func (m Model) appIDForUnit(unitID string) string {
	for _, edge := range m.graph.Edges {
		if edge.Type == domain.EdgeAppHasUnit && edge.TargetID == unitID {
			return edge.SourceID
		}
	}
	return ""
}

func (m Model) appIDForStorage(storageID string) string {
	for _, edge := range m.graph.Edges {
		if edge.Type != domain.EdgeStorageAttached || edge.TargetID != storageID {
			continue
		}
		return m.appIDForUnit(edge.SourceID)
	}
	return ""
}

func (m Model) storageIsAttached(storageID string) bool {
	return m.appIDForStorage(storageID) != ""
}

func (m Model) applicationContentRowCount(appID string) int {
	rows := 0
	for _, unitID := range m.unitIDsForApp(appID) {
		rows++
		rows += len(m.storageIDsForUnit(unitID))
	}
	return rows
}

func (m Model) applicationCompactHeight(appID string) int {
	return max(5, 4+m.applicationContentRowCount(appID))
}

func (m Model) placementSelected(edge placementEdge) bool {
	return m.selectedID == edge.unitID || m.selectedID == edge.machineID || m.selectedID == edge.appID
}

func (m Model) placementSweepActive(edge placementEdge) bool {
	return m.selectedID == edge.unitID
}

func (m Model) applicationUnitRow(appID, unitID string, box renderBox) (int, bool) {
	for _, row := range m.applicationSelectableRows(appID, box) {
		if row.id == unitID {
			return row.y, true
		}
	}
	return 0, false
}

func (m Model) machineUnitRow(machineID, unitID string, box renderBox) (int, bool) {
	for _, row := range m.machineSelectableRows(machineID, box) {
		if row.id == unitID {
			return row.y, true
		}
	}
	return 0, false
}

func (m Model) topologyNodeIDs() []string {
	ids := []string{}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type == domain.NodeApplication || node.Type == domain.NodeMachine || (node.Type == domain.NodeStorage && !m.storageIsAttached(id)) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m Model) topologyRenderBoxes() []renderBox {
	return m.topologyRenderBoxesIn(0, 0)
}

func (m Model) topologyRenderBoxesIn(width, height int) []renderBox {
	boxes := []renderBox{}
	for _, id := range m.topologyNodeIDs() {
		box, ok := m.nodeRenderBoxIn(id, width, height)
		if ok {
			boxes = append(boxes, box)
		}
	}
	return boxes
}

func (m Model) machineNodeIDs() []string {
	ids := []string{}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type == domain.NodeMachine {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m Model) withFitOffset(width, height int, ids []string) Model {
	ranges := m.viewportPanRange(width, height, ids)
	if !ranges.ok {
		return m
	}
	m.panX = clamp(m.panX, ranges.minX, ranges.maxX)
	m.panY = clamp(m.panY, ranges.minY, ranges.maxY)
	return m
}

func (m Model) viewportPanRange(width, height int, ids []string) panRange {
	if len(ids) == 0 || width <= 0 || height <= 0 {
		return panRange{}
	}
	bounds, ok := m.contentBounds(ids)
	if !ok {
		return panRange{}
	}
	const marginX = 2
	const marginY = 1
	contentWidth := bounds.width
	contentHeight := bounds.height
	availableWidth := max(1, width-marginX*2)
	availableHeight := max(1, height-marginY*2)
	ranges := panRange{ok: true}
	if contentWidth <= availableWidth {
		ranges.minX = marginX - bounds.x
		ranges.maxX = ranges.minX
	} else {
		ranges.minX = width - marginX - (bounds.x + bounds.width)
		ranges.maxX = marginX - bounds.x
		ranges.overflowX = true
	}
	if contentHeight <= availableHeight {
		ranges.minY = marginY - bounds.y
		ranges.maxY = ranges.minY
	} else {
		ranges.minY = height - marginY - (bounds.y + bounds.height)
		ranges.maxY = marginY - bounds.y
		ranges.overflowY = true
	}
	return ranges
}

func (m Model) contentBounds(ids []string) (renderBox, bool) {
	raw := m
	raw.panX = 0
	raw.panY = 0
	var bounds renderBox
	found := false
	for _, id := range ids {
		box, ok := raw.nodeRenderBox(id)
		if !ok {
			continue
		}
		if !found {
			bounds = box
			found = true
			continue
		}
		minX := min(bounds.x, box.x)
		minY := min(bounds.y, box.y)
		maxX := max(bounds.x+bounds.width, box.x+box.width)
		maxY := max(bounds.y+bounds.height, box.y+box.height)
		bounds = renderBox{x: minX, y: minY, width: maxX - minX, height: maxY - minY}
	}
	return bounds, found
}

type renderBox struct {
	x      int
	y      int
	width  int
	height int
}

type panRange struct {
	minX      int
	maxX      int
	minY      int
	maxY      int
	overflowX bool
	overflowY bool
	ok        bool
}

func (m Model) nodeRenderBox(id string) (renderBox, bool) {
	return m.nodeRenderBoxIn(id, 0, 0)
}

func (m Model) nodeRenderBoxIn(id string, width, height int) (renderBox, bool) {
	base, ok := m.compactNodeRenderBox(id)
	if !ok || !m.nodeWantsExpansion(id, base) {
		return base, ok
	}
	expanded, ok := m.expandedNodeRenderBox(id, base)
	if !ok {
		return base, true
	}
	var best renderBox
	bestScore := 0
	found := false
	for _, candidate := range expansionCandidates(base, expanded) {
		if !candidate.fits(width, height) {
			continue
		}
		if m.renderBoxOverlapsVisibleNode(id, candidate) {
			continue
		}
		score := m.expansionImpactScore(id, base, candidate)
		if !found || score < bestScore {
			best = candidate
			bestScore = score
			found = true
		}
	}
	if found {
		return best, true
	}
	return base, true
}

func (m Model) compactNodeRenderBox(id string) (renderBox, bool) {
	node, ok := m.graph.Nodes[id]
	if !ok {
		return renderBox{}, false
	}
	x, y := m.screen(node.Current)
	switch node.Type {
	case domain.NodeApplication:
		return renderBox{x: x, y: y, width: appBoxWidth, height: m.applicationCompactHeight(id)}, true
	case domain.NodeMachine:
		unitCount := len(m.unitIDsForMachine(id))
		return renderBox{x: x, y: y, width: appBoxWidth + 4, height: max(5, 5+unitCount)}, true
	case domain.NodeStorage:
		if m.storageIsAttached(id) {
			return renderBox{}, false
		}
		return renderBox{x: x, y: y, width: storageBoxWidth, height: 4}, true
	default:
		return renderBox{}, false
	}
}

func (m Model) expandedNodeRenderBox(id string, base renderBox) (renderBox, bool) {
	node, ok := m.graph.Nodes[id]
	if !ok {
		return renderBox{}, false
	}
	switch node.Type {
	case domain.NodeApplication:
		base.height = max(6, 5+m.applicationContentRowCount(id))
		base.width = selectedAppBoxWidth
		return base, true
	case domain.NodeMachine:
		unitCount := len(m.unitIDsForMachine(id))
		base.height = max(6, 6+unitCount)
		base.width = selectedMachineBoxWidth
		return base, true
	case domain.NodeStorage:
		base.width = selectedStorageBoxWidth
		base.height = selectedStorageBoxHeight
		return base, true
	default:
		return renderBox{}, false
	}
}

func (m Model) nodeWantsExpansion(id string, base renderBox) bool {
	node, ok := m.graph.Nodes[id]
	if !ok {
		return false
	}
	switch node.Type {
	case domain.NodeApplication:
		if m.selectedID != id && !m.unitBelongsToApplication(m.selectedID, id) && !m.storageBelongsToApplication(m.selectedID, id) {
			return false
		}
	case domain.NodeMachine:
		if m.selectedID != id && !m.unitBelongsToMachine(m.selectedID, id) {
			return false
		}
	case domain.NodeStorage:
		if m.selectedID != id {
			return false
		}
	default:
		return false
	}
	return m.nodeContentOverflows(id, base)
}

func (m Model) nodeContentOverflows(id string, box renderBox) bool {
	node := m.graph.Nodes[id]
	switch node.Type {
	case domain.NodeApplication:
		return m.applicationContentOverflows(node, box)
	case domain.NodeMachine:
		return m.machineContentOverflows(node, box)
	case domain.NodeStorage:
		return m.storageContentOverflows(node, box)
	default:
		return false
	}
}

func (m Model) applicationContentOverflows(node domain.Node, box renderBox) bool {
	innerWidth := box.width - 4
	if textOverflows(applicationIcon+" "+node.Label+" ", innerWidth) {
		return true
	}
	status := string(node.Status)
	if node.StatusMessage != "" {
		status += " " + node.StatusMessage
	}
	if textOverflows(fmt.Sprintf("%s %s", status, StatusSymbol(node.Status)), innerWidth) {
		return true
	}
	for _, unitID := range m.unitIDsForApp(node.ID) {
		unit := m.graph.Nodes[unitID]
		if textOverflows(fmt.Sprintf("%s %s", StatusSymbol(unit.Status), unit.Label), innerWidth) {
			return true
		}
		for _, storageID := range m.storageIDsForUnit(unitID) {
			storage := m.graph.Nodes[storageID]
			text := fmt.Sprintf("╰─ %s %s %s %s", storageIcon, storage.Label, StatusSymbol(storage.Status), string(storage.Status))
			if textOverflows(text, box.width-6) {
				return true
			}
		}
	}
	return false
}

func (m Model) machineContentOverflows(node domain.Node, box renderBox) bool {
	innerWidth := box.width - 4
	if textOverflows(machineIcon+" "+node.Label+" ", innerWidth) {
		return true
	}
	status := string(node.Status)
	if node.StatusMessage != "" {
		status += " " + node.StatusMessage
	}
	if textOverflows(fmt.Sprintf("%s %s", status, StatusSymbol(node.Status)), innerWidth) {
		return true
	}
	if textOverflows(node.Metadata["ip_address"], innerWidth) {
		return true
	}
	for _, unitID := range m.unitIDsForMachine(node.ID) {
		unit := m.graph.Nodes[unitID]
		if textOverflows(StatusSymbol(unit.Status)+" "+unit.Label, innerWidth) {
			return true
		}
	}
	return false
}

func (m Model) storageContentOverflows(node domain.Node, box renderBox) bool {
	if textOverflows(storageIcon+" "+node.Label, box.width-8) {
		return true
	}
	detail := firstNonEmpty(node.Metadata["location"], node.Metadata["unit"], node.Metadata["kind"])
	if detail == "" {
		detail = "storage"
	}
	if textOverflows(detail, box.width-4) {
		return true
	}
	footer := strings.TrimSpace(node.Metadata["kind"] + " " + StatusSymbol(node.Status) + " " + string(node.Status))
	return textOverflows(footer, box.width-4)
}

func (m Model) unitBelongsToApplication(unitID, appID string) bool {
	if _, ok := m.graph.Nodes[unitID]; !ok {
		return false
	}
	for _, edge := range m.graph.Edges {
		if edge.Type == domain.EdgeAppHasUnit && edge.SourceID == appID && edge.TargetID == unitID {
			return true
		}
	}
	return false
}

func (m Model) storageBelongsToApplication(storageID, appID string) bool {
	if _, ok := m.graph.Nodes[storageID]; !ok {
		return false
	}
	return m.appIDForStorage(storageID) == appID
}

func (m Model) unitBelongsToMachine(unitID, machineID string) bool {
	if _, ok := m.graph.Nodes[unitID]; !ok {
		return false
	}
	for _, edge := range m.graph.Edges {
		if edge.Type == domain.EdgeUnitOnMachine && edge.SourceID == unitID && edge.TargetID == machineID {
			return true
		}
	}
	return false
}

func (m Model) renderBoxOverlapsVisibleNode(id string, candidate renderBox) bool {
	for _, otherID := range m.visibleRenderNodeIDs() {
		if otherID == id {
			continue
		}
		other, ok := m.compactNodeRenderBox(otherID)
		if !ok {
			continue
		}
		if candidate.overlaps(other) {
			return true
		}
	}
	return false
}

func (m Model) expansionImpactScore(id string, base, candidate renderBox) int {
	score := abs(candidate.x-base.x) + abs(candidate.y-base.y)
	for _, edgeID := range m.graph.EdgeOrder {
		edge := m.graph.Edges[edgeID]
		if edge.Type != domain.EdgeRelation {
			continue
		}
		if edge.SourceID == id || edge.TargetID == id {
			continue
		}
		source, ok := m.compactNodeRenderBox(edge.SourceID)
		if !ok {
			continue
		}
		target, ok := m.compactNodeRenderBox(edge.TargetID)
		if !ok {
			continue
		}
		if candidate.intersectsAny(simpleRelationSegments(source, target)) {
			score += 100
		}
	}
	return score
}

func (m Model) visibleRenderNodeIDs() []string {
	switch m.view {
	case ViewMachines:
		return m.machineNodeIDs()
	default:
		return m.topologyNodeIDs()
	}
}

func expansionCandidates(base, expanded renderBox) []renderBox {
	leftAnchored := expanded
	rightAligned := expanded
	rightAligned.x = base.x + base.width - expanded.width
	centered := expanded
	centered.x = base.x - (expanded.width-base.width)/2
	return uniqueRenderBoxes([]renderBox{leftAnchored, rightAligned, centered})
}

func uniqueRenderBoxes(boxes []renderBox) []renderBox {
	out := []renderBox{}
	for _, box := range boxes {
		seen := false
		for _, existing := range out {
			if existing == box {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, box)
		}
	}
	return out
}

func (b renderBox) fits(width, height int) bool {
	if width <= 0 || height <= 0 {
		return true
	}
	return b.x >= 0 && b.y >= 0 && b.x+b.width <= width && b.y+b.height <= height
}

func (b renderBox) overlaps(other renderBox) bool {
	return b.x < other.x+other.width &&
		b.x+b.width > other.x &&
		b.y < other.y+other.height &&
		b.y+b.height > other.y
}

func (b renderBox) intersectsAny(segments []routeSegment) bool {
	for _, segment := range segments {
		if segmentIntersectsBox(segment, b) {
			return true
		}
	}
	return false
}

func simpleRelationSegments(sourceBox, targetBox renderBox) []routeSegment {
	source := routePoint{x: sourceBox.x + sourceBox.width/2, y: relationAnchorY(sourceBox)}
	target := routePoint{x: targetBox.x + targetBox.width/2, y: relationAnchorY(targetBox)}
	laneY := (source.y + target.y) / 2
	route := relationRoute{points: cleanRoutePoints([]routePoint{
		source,
		{x: source.x, y: laneY},
		{x: target.x, y: laneY},
		target,
	})}
	return route.segments()
}

func (m Model) relationLabelLimit(edge domain.Edge) int {
	if m.selectedID == edge.ID && textOverflows(edge.Label, relationLabelWidth) {
		return selectedRelationLabelWidth
	}
	return relationLabelWidth
}

func (m Model) applicationDetail(node domain.Node) string {
	parts := []string{}
	if charm := node.Metadata["charm"]; charm != "" {
		parts = append(parts, "charm: "+charm)
	}
	if channel := node.Metadata["charm_channel"]; channel != "" {
		parts = append(parts, "channel: "+channel)
	}
	if version := node.Metadata["charm_version"]; version != "" {
		parts = append(parts, "version: "+version)
	}
	return strings.Join(parts, "  ")
}

func (m Model) machineDetail(node domain.Node) string {
	parts := []string{}
	if instance := node.Metadata["instance_id"]; instance != "" {
		parts = append(parts, "instance: "+instance)
	}
	if dns := node.Metadata["dns_name"]; dns != "" {
		parts = append(parts, "dns: "+dns)
	}
	return strings.Join(parts, "  ")
}

func (m Model) nodeStyle(node domain.Node) lipgloss.Style {
	if m.selectedID == node.ID {
		return m.styles.Selected
	}
	if node.Status.Interesting() {
		return m.styles.Status(node.Status)
	}
	if m.animations.Pulse(node.ID) > 0.01 {
		return m.styles.Changed
	}
	return m.styles.Status(node.Status)
}

func (m Model) screen(pos domain.Position) (int, int) {
	return int(math.Round(pos.X)) + m.panX, int(math.Round(pos.Y)) + m.panY
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func textOverflows(value string, width int) bool {
	return len([]rune(strings.TrimSpace(value))) > width
}

func (m Model) scrollNodeIDs() []string {
	switch m.view {
	case ViewMachines:
		return m.machineNodeIDs()
	case ViewTopology:
		return m.topologyNodeIDs()
	default:
		return nil
	}
}

func (m Model) scrollStatus() string {
	if !m.hasGraph {
		return ""
	}
	ranges := m.viewportPanRange(m.canvasWidth(), m.canvasHeight(), m.scrollNodeIDs())
	if !ranges.ok || (!ranges.overflowX && !ranges.overflowY) {
		return ""
	}
	panX := clamp(m.panX, ranges.minX, ranges.maxX)
	panY := clamp(m.panY, ranges.minY, ranges.maxY)
	parts := []string{}
	if ranges.overflowX {
		parts = append(parts, fmt.Sprintf("x %d%%", scrollPercent(panX, ranges.minX, ranges.maxX)))
	}
	if ranges.overflowY {
		parts = append(parts, fmt.Sprintf("y %d%%", scrollPercent(panY, ranges.minY, ranges.maxY)))
	}
	return "scroll " + strings.Join(parts, " ")
}

func scrollPercent(pan, minPan, maxPan int) int {
	if maxPan == minPan {
		return 0
	}
	return clamp((maxPan-pan)*100/(maxPan-minPan), 0, 100)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func clamp(value, low, high int) int {
	if low > high {
		low, high = high, low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func (m Model) canvasWidth() int {
	if m.width >= 96 {
		return m.width - 35
	}
	return m.width
}

func (m Model) canvasHeight() int {
	height := m.height - 2
	if m.showHelp {
		height -= 3
	}
	if m.searching {
		height--
	}
	if height < 1 {
		return 1
	}
	return height
}
