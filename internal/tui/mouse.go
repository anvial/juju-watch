package tui

import (
	"github.com/anvial/juju-watch/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.searching || msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	point, width, height, ok := m.mouseCanvasPoint(msg)
	if !ok || !m.hasGraph {
		return m, nil
	}
	id, ok := m.hitTestCanvasPoint(point, width, height)
	if !ok {
		return m, nil
	}
	m.setSelectedID(id)
	m = m.scrollSelectedIntoView()
	return m, nil
}

func (m Model) mouseCanvasPoint(msg tea.MouseMsg) (routePoint, int, int, bool) {
	if m.width <= 0 || m.height <= 0 {
		return routePoint{}, 0, 0, false
	}
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := lipgloss.Height(m.renderFooter())
	helpHeight := 0
	if m.showHelp {
		helpHeight = lipgloss.Height(m.help.View(m.keys))
	}
	searchHeight := 0
	if m.searching {
		searchHeight = lipgloss.Height(m.search.View())
	}
	bodyHeight := m.height - headerHeight - footerHeight - helpHeight - searchHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	canvasWidth := m.canvasWidth()
	if canvasWidth < 1 {
		canvasWidth = 1
	}
	x := msg.X
	y := msg.Y - headerHeight
	if x < 0 || y < 0 || x >= canvasWidth || y >= bodyHeight {
		return routePoint{}, 0, 0, false
	}
	return routePoint{x: x, y: y}, canvasWidth, bodyHeight, true
}

func (m Model) hitTestCanvasPoint(point routePoint, width, height int) (string, bool) {
	switch m.view {
	case ViewMachines:
		return m.hitTestMachines(point, width, height)
	case ViewProblems:
		return m.hitTestProblems(point)
	case ViewEvents:
		return "", false
	default:
		return m.hitTestTopology(point, width, height)
	}
}

func (m Model) hitTestTopology(point routePoint, width, height int) (string, bool) {
	m = m.withFitOffset(width, height, m.topologyNodeIDs())
	if id, ok := m.hitTestTopologyRows(point, width, height); ok {
		return id, true
	}
	if id, ok := m.hitTestTopologyBoxes(point, width, height); ok {
		return id, true
	}
	if id, ok := m.hitTestTopologyRoutes(point, width, height); ok {
		return id, true
	}
	return "", false
}

func (m Model) hitTestTopologyRows(point routePoint, width, height int) (string, bool) {
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		switch node.Type {
		case domain.NodeApplication:
			box, ok := m.nodeRenderBoxIn(id, width, height)
			if !ok {
				continue
			}
			if rowID, ok := m.hitTestApplicationRows(point, id, box); ok {
				return rowID, true
			}
		case domain.NodeMachine:
			box, ok := m.nodeRenderBoxIn(id, width, height)
			if !ok {
				continue
			}
			if rowID, ok := m.hitTestMachineRows(point, id, box); ok {
				return rowID, true
			}
		}
	}
	return "", false
}

func (m Model) hitTestTopologyBoxes(point routePoint, width, height int) (string, bool) {
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		switch {
		case node.Type == domain.NodeApplication || node.Type == domain.NodeMachine:
		case node.Type == domain.NodeStorage && !m.storageIsAttached(id):
		default:
			continue
		}
		box, ok := m.nodeRenderBoxIn(id, width, height)
		if ok && pointInRenderBox(point, box) {
			return id, true
		}
	}
	return "", false
}

func (m Model) hitTestTopologyRoutes(point routePoint, width, height int) (string, bool) {
	for _, edge := range m.placementEdges() {
		route, ok := m.placementRoute(edge, width, height)
		if ok && routeHit(point, route) {
			return edge.unitID, true
		}
	}
	for _, id := range m.graph.EdgeOrder {
		edge := m.graph.Edges[id]
		if edge.Type != domain.EdgeRelation {
			continue
		}
		source := m.graph.Nodes[edge.SourceID]
		target := m.graph.Nodes[edge.TargetID]
		if source.Type != domain.NodeApplication || target.Type != domain.NodeApplication {
			continue
		}
		route, ok := m.relationRoute(edge, width, height)
		if ok && routeHit(point, route) {
			return edge.ID, true
		}
	}
	return "", false
}

func (m Model) hitTestMachines(point routePoint, width, height int) (string, bool) {
	m = m.withFitOffset(width, height, m.machineNodeIDs())
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type != domain.NodeMachine {
			continue
		}
		box, ok := m.nodeRenderBoxIn(id, width, height)
		if !ok {
			continue
		}
		if rowID, ok := m.hitTestMachineRows(point, id, box); ok {
			return rowID, true
		}
	}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if node.Type != domain.NodeMachine {
			continue
		}
		box, ok := m.nodeRenderBoxIn(id, width, height)
		if ok && pointInRenderBox(point, box) {
			return id, true
		}
	}
	return "", false
}

func (m Model) hitTestProblems(point routePoint) (string, bool) {
	index := 0
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		if !node.Status.Interesting() {
			continue
		}
		box := renderBox{
			x:      2 + (index%2)*(cardWidth+4),
			y:      2 + (index/2)*6,
			width:  cardWidth,
			height: 5,
		}
		if pointInRenderBox(point, box) {
			return id, true
		}
		index++
	}
	return "", false
}

func (m Model) hitTestApplicationRows(point routePoint, appID string, box renderBox) (string, bool) {
	if !pointInRenderBox(point, box) {
		return "", false
	}
	for _, row := range m.applicationSelectableRows(appID, box) {
		if point.y == row.y && pointInsideRow(point, box) {
			return row.id, true
		}
	}
	return "", false
}

func (m Model) hitTestMachineRows(point routePoint, machineID string, box renderBox) (string, bool) {
	if !pointInRenderBox(point, box) {
		return "", false
	}
	for _, row := range m.machineSelectableRows(machineID, box) {
		if point.y == row.y && pointInsideRow(point, box) {
			return row.id, true
		}
	}
	return "", false
}

type selectableRow struct {
	id string
	y  int
}

func (m Model) applicationSelectableRows(appID string, box renderBox) []selectableRow {
	node, ok := m.graph.Nodes[appID]
	if !ok {
		return nil
	}
	rows := []selectableRow{}
	row := box.y + 2
	expanded := box.width > appBoxWidth || box.height > m.applicationCompactHeight(appID)
	if expanded {
		if detail := m.applicationDetail(node); detail != "" {
			row++
		}
	}
	row++
	for _, unitID := range m.unitIDsForApp(appID) {
		if row >= box.y+box.height-1 {
			break
		}
		rows = append(rows, selectableRow{id: unitID, y: row})
		row++
		for _, storageID := range m.storageIDsForUnit(unitID) {
			if row >= box.y+box.height-1 {
				break
			}
			rows = append(rows, selectableRow{id: storageID, y: row})
			row++
		}
	}
	return rows
}

func (m Model) machineSelectableRows(machineID string, box renderBox) []selectableRow {
	node, ok := m.graph.Nodes[machineID]
	if !ok {
		return nil
	}
	rows := []selectableRow{}
	units := m.unitIDsForMachine(machineID)
	row := box.y + 2
	if node.Metadata["ip_address"] != "" {
		row++
	}
	expanded := box.width > appBoxWidth+4 || box.height > max(5, 5+len(units))
	if expanded {
		if detail := m.machineDetail(node); detail != "" && row < box.y+box.height-1 {
			row++
		}
	}
	for index, unitID := range units {
		unitRow := row + index
		if unitRow >= box.y+box.height-1 {
			break
		}
		rows = append(rows, selectableRow{id: unitID, y: unitRow})
	}
	return rows
}

func routeHit(point routePoint, route relationRoute) bool {
	for _, routePoint := range routePathPoints(route) {
		if point == routePoint {
			return true
		}
	}
	return labelHit(point, route.labelAt, route.label)
}

func labelHit(point routePoint, labelAt *routePoint, label string) bool {
	if labelAt == nil || point.y != labelAt.y {
		return false
	}
	x := labelAt.x
	for _, ch := range label {
		width := runewidth.RuneWidth(ch)
		if width <= 0 {
			width = 1
		}
		if point.x >= x && point.x < x+width {
			return true
		}
		x += width
	}
	return false
}

func pointInRenderBox(point routePoint, box renderBox) bool {
	return point.x >= box.x && point.x < box.x+box.width && point.y >= box.y && point.y < box.y+box.height
}

func pointInsideRow(point routePoint, box renderBox) bool {
	return point.x > box.x && point.x < box.x+box.width-1
}
