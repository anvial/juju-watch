package tui

import (
	"strings"
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
)

func TestWithFitOffsetShiftsVisibleTopologyIntoCanvas(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	storageID := domain.StorageID("prod", "data/0")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 10, Y: 10},
	}
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "data/0",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 10, Y: 20},
	}
	model.graph.Order = []string{appID, storageID}

	fitted := model.withFitOffset(80, 30, model.topologyNodeIDs())
	appBox, ok := fitted.nodeRenderBox(appID)
	if !ok {
		t.Fatal("missing app render box")
	}
	storageBox, ok := fitted.nodeRenderBox(storageID)
	if !ok {
		t.Fatal("missing storage render box")
	}
	if appBox.y < 1 || appBox.x < 2 {
		t.Fatalf("app not shifted into top-left canvas margin: %+v", appBox)
	}
	if storageBox.y+storageBox.height > 29 {
		t.Fatalf("storage not fitted into canvas: %+v", storageBox)
	}
}

func TestWithFitOffsetStartsOversizedContentAtTopLeft(t *testing.T) {
	model := tallTopologyModel(8)

	fitted := model.withFitOffset(90, 18, model.topologyNodeIDs())
	firstBox, ok := fitted.nodeRenderBox(domain.MachineID("prod", "0"))
	if !ok {
		t.Fatal("missing first machine")
	}
	if firstBox.x != 2 || firstBox.y != 1 {
		t.Fatalf("oversized content should start at top-left viewport margin: %+v", firstBox)
	}
}

func TestWithFitOffsetClampsOversizedContentAtBottom(t *testing.T) {
	model := tallTopologyModel(8)
	model.panY = -1000

	fitted := model.withFitOffset(90, 18, model.topologyNodeIDs())
	lastBox, ok := fitted.nodeRenderBox(domain.MachineID("prod", "7"))
	if !ok {
		t.Fatal("missing last machine")
	}
	if lastBox.y+lastBox.height > 17 {
		t.Fatalf("bottom clamp should keep last machine visible: %+v", lastBox)
	}
}

func TestPageDownScrollsAndClamps(t *testing.T) {
	model := tallTopologyModel(8)
	model.width = 90
	model.height = 20
	model = model.clampPanForCurrentView()
	startPanY := model.panY

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	if model.panY >= startPanY {
		t.Fatalf("page down should move viewport down: start=%d after=%d", startPanY, model.panY)
	}

	for i := 0; i < 20; i++ {
		updated, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyPgDown})
		model = updated.(Model)
	}
	ranges := model.viewportPanRange(model.canvasWidth(), model.canvasHeight(), model.scrollNodeIDs())
	if model.panY != ranges.minY {
		t.Fatalf("page down should clamp at bottom: panY=%d bottom=%d", model.panY, ranges.minY)
	}
}

func TestHomeEndJumpToVerticalBounds(t *testing.T) {
	model := tallTopologyModel(8)
	model.width = 90
	model.height = 20
	model = model.clampPanForCurrentView()
	ranges := model.viewportPanRange(model.canvasWidth(), model.canvasHeight(), model.scrollNodeIDs())

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(Model)
	if model.panY != ranges.minY {
		t.Fatalf("end should jump to bottom: panY=%d bottom=%d", model.panY, ranges.minY)
	}

	updated, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyHome})
	model = updated.(Model)
	if model.panY != ranges.maxY {
		t.Fatalf("home should jump to top: panY=%d top=%d", model.panY, ranges.maxY)
	}
}

func TestFooterShowsScrollStateWhenTopologyOverflows(t *testing.T) {
	model := tallTopologyModel(8)
	model.width = 90
	model.height = 20
	model = model.clampPanForCurrentView()

	footer := model.renderFooter()
	if !strings.Contains(footer, "scroll y 0%") {
		t.Fatalf("footer should show vertical scroll state: %q", footer)
	}
}

func TestSelectedAttachedStorageScrollsIntoView(t *testing.T) {
	model := tallTopologyModel(8)
	appID := domain.AppID("prod", "app7")
	unitID := domain.UnitID("prod", "app7/0")
	storageID := domain.StorageID("prod", "data/7")
	storageEdgeID := domain.EdgeID(domain.EdgeStorageAttached, unitID, storageID)
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "data/7",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: model.graph.Nodes[appID].Current.X + 4, Y: model.graph.Nodes[appID].Current.Y + 4},
	}
	model.graph.Edges[storageEdgeID] = domain.Edge{
		ID:       storageEdgeID,
		SourceID: unitID,
		TargetID: storageID,
		Type:     domain.EdgeStorageAttached,
	}
	model.graph.Order = append(model.graph.Order, storageID)
	model.selectedID = storageID
	model.width = 90
	model.height = 20
	model = model.clampPanForCurrentView()
	model = model.scrollSelectedIntoView()

	box, ok := model.selectedRawBox()
	if !ok {
		t.Fatal("missing selected storage row")
	}
	screenY := box.y + model.panY
	if screenY < 1 || screenY >= model.canvasHeight()-1 {
		t.Fatalf("selected nested storage should be visible: raw=%+v panY=%d screenY=%d", box, model.panY, screenY)
	}
}

func tallTopologyModel(count int) Model {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	model.graph = domain.NewGraph("prod")
	for i := 0; i < count; i++ {
		machineID := domain.MachineID("prod", string(rune('0'+i)))
		appName := "app" + string(rune('0'+i))
		appID := domain.AppID("prod", appName)
		unitID := domain.UnitID("prod", appName+"/0")
		y := float64(2 + i*8)
		model.graph.Nodes[machineID] = domain.Node{
			ID:      machineID,
			Label:   "machine " + string(rune('0'+i)),
			Type:    domain.NodeMachine,
			Status:  domain.StatusActive,
			Current: domain.Position{X: 4, Y: y},
		}
		model.graph.Nodes[appID] = domain.Node{
			ID:      appID,
			Label:   appName,
			Type:    domain.NodeApplication,
			Status:  domain.StatusActive,
			Current: domain.Position{X: 44, Y: y},
		}
		model.graph.Nodes[unitID] = domain.Node{
			ID:      unitID,
			Label:   appName + "/0",
			Type:    domain.NodeUnit,
			Status:  domain.StatusActive,
			Current: domain.Position{X: 46, Y: y + 3},
		}
		appUnitEdgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
		machineEdgeID := domain.EdgeID(domain.EdgeUnitOnMachine, unitID, machineID)
		model.graph.Edges[appUnitEdgeID] = domain.Edge{
			ID:       appUnitEdgeID,
			SourceID: appID,
			TargetID: unitID,
			Type:     domain.EdgeAppHasUnit,
		}
		model.graph.Edges[machineEdgeID] = domain.Edge{
			ID:       machineEdgeID,
			SourceID: unitID,
			TargetID: machineID,
			Type:     domain.EdgeUnitOnMachine,
		}
		model.graph.Order = append(model.graph.Order, machineID, appID, unitID)
		model.graph.EdgeOrder = append(model.graph.EdgeOrder, appUnitEdgeID, machineEdgeID)
	}
	model.hasGraph = true
	model.view = ViewTopology
	return model
}
