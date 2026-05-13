package tui

import (
	"context"
	"strings"
	"time"

	"github.com/anvial/juju-watch/internal/diff"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.searching {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.searching = false
				m.search.Blur()
				return m, nil
			case "enter":
				m.selectFirstSearchMatch()
				m.searching = false
				m.search.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.clampPanForCurrentView()
	case tea.KeyMsg:
		return m.handleKey(msg)
	case pollTickMsg:
		cmds = append(cmds, tickCmd(m.cfg.Interval))
		if !m.paused && !m.polling {
			m.polling = true
			cmds = append(cmds, m.pollCmd())
		}
	case PollResultMsg:
		m.polling = false
		m.lastPollAt = msg.At
		if msg.Err != nil {
			m.lastErr = msg.Err
			m.addEvent(domain.Event{At: msg.At, Kind: "poll-error", Label: "poll", Message: msg.Err.Error()})
			break
		}
		m.lastErr = nil
		m.applyState(msg.State, msg.At)
		cmds = append(cmds, frameCmd())
	case frameMsg:
		active := false
		if m.hasGraph {
			active = m.animations.Step(&m.graph, m.cfg.NoAnimation)
		}
		if active {
			cmds = append(cmds, frameCmd())
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" && m.showHelp {
		m.showHelp = false
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
	case key.Matches(msg, m.keys.Refresh):
		if !m.polling {
			m.polling = true
			return m, m.pollCmd()
		}
	case key.Matches(msg, m.keys.Pause):
		m.paused = !m.paused
	case key.Matches(msg, m.keys.NextView):
		m.nextView()
		m.ensureSelection()
		m = m.scrollSelectedIntoView()
	case key.Matches(msg, m.keys.Search):
		m.searching = true
		return m, m.search.Focus()
	case key.Matches(msg, m.keys.Focus):
		m.focusSelected()
	case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Left):
		m.moveSelection(-1)
	case key.Matches(msg, m.keys.Down), key.Matches(msg, m.keys.Right):
		m.moveSelection(1)
	case key.Matches(msg, m.keys.PanLeft):
		m.panX += 4
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.PanRight):
		m.panX -= 4
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.PanUp):
		m.panY += 2
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.PanDown):
		m.panY -= 2
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.PageUp):
		m.panY += max(4, m.canvasHeight()-4)
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.PageDown):
		m.panY -= max(4, m.canvasHeight()-4)
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.Home):
		if ranges := m.viewportPanRange(m.canvasWidth(), m.canvasHeight(), m.scrollNodeIDs()); ranges.ok {
			m.panY = ranges.maxY
		}
		m = m.clampPanForCurrentView()
	case key.Matches(msg, m.keys.End):
		if ranges := m.viewportPanRange(m.canvasWidth(), m.canvasHeight(), m.scrollNodeIDs()); ranges.ok {
			m.panY = ranges.minY
		}
		m = m.clampPanForCurrentView()
	}
	return m, nil
}

func (m *Model) applyState(state domain.State, at time.Time) {
	previous := m.graph
	graph := domain.BuildGraph(state)
	var previousPtr *domain.Graph
	if m.hasGraph {
		previousPtr = &previous
	}
	result := diff.Graphs(previous, graph)
	graph, err := m.layout.Layout(context.Background(), graph, previousPtr, m.layoutOptions())
	if err != nil {
		m.layoutErr = err
		graph, _ = layout.Schema{}.Layout(context.Background(), graph, previousPtr, m.layoutOptions())
	} else {
		m.layoutErr = nil
	}

	m.graph = graph
	m.hasGraph = true
	m.changes = result
	m.changedIDs = map[string]bool{}
	for _, id := range result.ChangedNodeIDs() {
		change := result.Nodes[id]
		if change.Kind != diff.Unchanged && change.Kind != diff.Removed {
			m.changedIDs[id] = true
		}
	}
	m.animations.Sync(m.graph, m.changedIDs)
	m.appendDiffEvents(result, at)
	m.ensureSelection()
	*m = m.scrollSelectedIntoView()
	if m.cfg.Focus != "" {
		m.selectByQuery(m.cfg.Focus)
		m.focusSelected()
		m.cfg.Focus = ""
	}
}

func (m Model) layoutOptions() layout.Options {
	return layout.Options{
		Width:  m.canvasWidth(),
		Height: m.canvasHeight(),
	}
}

func (m Model) pollCmd() tea.Cmd {
	poller := m.poller
	return func() tea.Msg {
		state, err := poller.Poll(context.Background())
		return PollResultMsg{State: state, Err: err, At: time.Now()}
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return pollTickMsg(t)
	})
}

func frameCmd() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg {
		return frameMsg(t)
	})
}

func (m *Model) nextView() {
	switch m.view {
	case ViewTopology:
		m.view = ViewMachines
	case ViewMachines:
		m.view = ViewProblems
	case ViewProblems:
		m.view = ViewEvents
	default:
		m.view = ViewTopology
	}
}

func (m *Model) moveSelection(delta int) {
	ids := m.visibleIDs()
	if len(ids) == 0 {
		m.selectedID = ""
		return
	}
	index := 0
	for i, id := range ids {
		if id == m.selectedID {
			index = i
			break
		}
	}
	index = (index + delta + len(ids)) % len(ids)
	m.selectedID = ids[index]
	*m = m.scrollSelectedIntoView()
}

func (m *Model) ensureSelection() {
	ids := m.visibleIDs()
	if len(ids) == 0 {
		m.selectedID = ""
		return
	}
	for _, id := range ids {
		if id == m.selectedID {
			return
		}
	}
	m.selectedID = ids[0]
	*m = m.scrollSelectedIntoView()
}

func (m *Model) selectFirstSearchMatch() {
	m.selectByQuery(m.search.Value())
	*m = m.scrollSelectedIntoView()
}

func (m *Model) selectByQuery(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return
	}
	for _, id := range m.visibleIDs() {
		if strings.Contains(strings.ToLower(m.searchText(id)), query) {
			m.selectedID = id
			*m = m.scrollSelectedIntoView()
			return
		}
	}
}

func (m Model) searchText(id string) string {
	if node, ok := m.graph.Nodes[id]; ok {
		return node.ID + " " + node.Label + " " + string(node.Type) + " " + string(node.Status) + " " + node.StatusMessage
	}
	if edge, ok := m.graph.Edges[id]; ok {
		return edge.ID + " " + edge.Label + " " + string(edge.Type)
	}
	return id
}

func (m *Model) focusSelected() {
	if node, ok := m.graph.Nodes[m.selectedID]; ok {
		m.panX = m.canvasWidth()/2 - int(node.Current.X) - 11
		m.panY = m.canvasHeight()/2 - int(node.Current.Y) - 3
		*m = m.clampPanForCurrentView()
		return
	}
	if edge, ok := m.graph.Edges[m.selectedID]; ok {
		source := m.graph.Nodes[edge.SourceID]
		target := m.graph.Nodes[edge.TargetID]
		x := int((source.Current.X + target.Current.X) / 2)
		y := int((source.Current.Y + target.Current.Y) / 2)
		m.panX = m.canvasWidth()/2 - x
		m.panY = m.canvasHeight()/2 - y
		*m = m.clampPanForCurrentView()
	}
}

func (m Model) clampPanForCurrentView() Model {
	return m.withFitOffset(m.canvasWidth(), m.canvasHeight(), m.scrollNodeIDs())
}

func (m Model) scrollSelectedIntoView() Model {
	if m.selectedID == "" || m.width == 0 || m.height == 0 {
		return m.clampPanForCurrentView()
	}
	box, ok := m.selectedRawBox()
	if !ok {
		return m.clampPanForCurrentView()
	}
	const marginX = 2
	const marginY = 1
	width := m.canvasWidth()
	height := m.canvasHeight()
	if width <= 0 || height <= 0 {
		return m
	}
	left := box.x + m.panX
	right := left + box.width
	top := box.y + m.panY
	bottom := top + box.height
	if left < marginX {
		m.panX += marginX - left
	} else if right > width-marginX {
		m.panX -= right - (width - marginX)
	}
	if top < marginY {
		m.panY += marginY - top
	} else if bottom > height-marginY {
		m.panY -= bottom - (height - marginY)
	}
	return m.clampPanForCurrentView()
}

func (m Model) selectedRawBox() (renderBox, bool) {
	raw := m
	raw.panX = 0
	raw.panY = 0
	if node, ok := raw.graph.Nodes[raw.selectedID]; ok {
		if box, ok := raw.nodeRenderBox(raw.selectedID); ok {
			return box, true
		}
		x := int(node.Current.X)
		y := int(node.Current.Y)
		width := max(1, min(32, len([]rune(node.Label))+4))
		return renderBox{x: x, y: y, width: width, height: 1}, true
	}
	if edge, ok := raw.graph.Edges[raw.selectedID]; ok {
		source, sourceOK := raw.graph.Nodes[edge.SourceID]
		target, targetOK := raw.graph.Nodes[edge.TargetID]
		if !sourceOK || !targetOK {
			return renderBox{}, false
		}
		x1 := int(source.Current.X)
		y1 := int(source.Current.Y)
		x2 := int(target.Current.X)
		y2 := int(target.Current.Y)
		x := min(x1, x2)
		y := min(y1, y2)
		return renderBox{x: x, y: y, width: abs(x2-x1) + 1, height: abs(y2-y1) + 1}, true
	}
	return renderBox{}, false
}

func (m Model) visibleIDs() []string {
	ids := []string{}
	if m.view == ViewTopology {
		for _, appID := range m.graph.Order {
			node := m.graph.Nodes[appID]
			if node.Type != domain.NodeApplication {
				continue
			}
			ids = append(ids, appID)
			for _, unitID := range m.unitIDsForApp(appID) {
				ids = append(ids, unitID)
				ids = append(ids, m.storageIDsForUnit(unitID)...)
			}
		}
		for _, id := range m.graph.Order {
			node := m.graph.Nodes[id]
			if node.Type == domain.NodeMachine || (node.Type == domain.NodeStorage && !m.storageIsAttached(id)) {
				ids = append(ids, id)
			}
		}
		for _, id := range m.graph.EdgeOrder {
			edge := m.graph.Edges[id]
			if edge.Type == domain.EdgeRelation {
				source := m.graph.Nodes[edge.SourceID]
				target := m.graph.Nodes[edge.TargetID]
				if source.Type == domain.NodeApplication && target.Type == domain.NodeApplication {
					ids = append(ids, id)
				}
			}
		}
		return ids
	}
	for _, id := range m.graph.Order {
		node := m.graph.Nodes[id]
		switch m.view {
		case ViewMachines:
			if node.Type == domain.NodeMachine || node.Type == domain.NodeUnit {
				ids = append(ids, id)
			}
		case ViewProblems:
			if node.Status.Interesting() {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func (m *Model) appendDiffEvents(result diff.Result, at time.Time) {
	for _, id := range result.ChangedNodeIDs() {
		change := result.Nodes[id]
		if change.Kind == diff.Unchanged {
			continue
		}
		event := domain.Event{At: at, ObjectID: id, Kind: string(change.Kind)}
		switch change.Kind {
		case diff.Added:
			event.Label = change.After.Label
			event.To = string(change.After.Status)
			event.Message = change.After.StatusMessage
		case diff.Removed:
			event.Label = change.Before.Label
			event.From = string(change.Before.Status)
			event.Message = "removed"
		case diff.Updated:
			event.Label = change.After.Label
			event.From = string(change.Before.Status)
			event.To = string(change.After.Status)
			event.Message = change.After.StatusMessage
		}
		m.addEvent(event)
	}
}

func (m *Model) addEvent(event domain.Event) {
	if event.At.IsZero() {
		event.At = time.Now()
	}
	m.events = append([]domain.Event{event}, m.events...)
	if len(m.events) > 80 {
		m.events = m.events[:80]
	}
}
