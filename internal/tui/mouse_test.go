package tui

import (
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestMouseClickSelectsTopologyBoxesAndRows(t *testing.T) {
	model := mouseTopologyTestModel()
	width, height := model.canvasWidth(), model.canvasHeight()
	fitted := model.withFitOffset(width, height, model.topologyNodeIDs())
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	attachedStorageID := domain.StorageID("prod", "data/0")
	machineID := domain.MachineID("prod", "0")
	standaloneStorageID := domain.StorageID("prod", "logs/0")

	appBox := mustNodeBox(t, fitted, appID, width, height)
	machineBox := mustNodeBox(t, fitted, machineID, width, height)
	storageBox := mustNodeBox(t, fitted, standaloneStorageID, width, height)
	unitRow := mustApplicationRow(t, fitted, appID, unitID, appBox)
	storageRow := mustApplicationRow(t, fitted, appID, attachedStorageID, appBox)

	assertLeftClickSelects(t, model, routePoint{x: appBox.x + 1, y: appBox.y}, appID)
	assertLeftClickSelects(t, model, routePoint{x: machineBox.x + 2, y: machineBox.y + 1}, machineID)
	assertLeftClickSelects(t, model, routePoint{x: storageBox.x + 2, y: storageBox.y + 1}, standaloneStorageID)
	assertLeftClickSelects(t, model, routePoint{x: appBox.x + 2, y: unitRow}, unitID)
	assertLeftClickSelects(t, model, routePoint{x: appBox.x + 4, y: storageRow}, attachedStorageID)
}

func TestMouseClickSelectsRelationRouteAndLabel(t *testing.T) {
	model := relationTestModel()
	model.width = 100
	model.height = 22
	width, height := model.canvasWidth(), model.canvasHeight()
	fitted := model.withFitOffset(width, height, model.topologyNodeIDs())
	edgeID := domain.RelationID("prod", "postgresql:db", "api:database")
	edge := fitted.graph.Edges[edgeID]
	route, ok := fitted.relationRoute(edge, width, height)
	if !ok {
		t.Fatal("expected relation route")
	}
	points := routePathPoints(route)
	if len(points) == 0 {
		t.Fatalf("expected relation path points: %+v", route)
	}
	assertLeftClickSelects(t, model, points[len(points)/2], edgeID)
	if route.labelAt == nil {
		t.Fatalf("expected relation label: %+v", route)
	}
	assertLeftClickSelects(t, model, *route.labelAt, edgeID)
}

func TestMouseClickSelectsPlacementRouteAndLabel(t *testing.T) {
	model := placementTestModel(1)
	model.width = 94
	model.height = 26
	width, height := model.canvasWidth(), model.canvasHeight()
	fitted := model.withFitOffset(width, height, model.topologyNodeIDs())
	edge := fitted.placementEdges()[0]
	route, ok := fitted.placementRoute(edge, width, height)
	if !ok {
		t.Fatal("expected placement route")
	}
	points := routePathPoints(route)
	if len(points) == 0 {
		t.Fatalf("expected placement path points: %+v", route)
	}
	assertLeftClickSelects(t, model, points[len(points)/2], edge.unitID)
	if route.labelAt == nil {
		t.Fatalf("expected placement label: %+v", route)
	}
	assertLeftClickSelects(t, model, *route.labelAt, edge.unitID)
}

func TestMouseClickIgnoresNonCanvasAndEmptyCells(t *testing.T) {
	model := mouseTopologyTestModel()
	model.selectedID = domain.AppID("prod", "postgresql")

	assertMouseDoesNotChangeSelection(t, model, leftClickCanvas(model, routePoint{x: model.canvasWidth() - 1, y: model.canvasHeight() - 1}))
	assertMouseDoesNotChangeSelection(t, model, tea.MouseMsg{
		X:      2,
		Y:      model.height - 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	wide := model
	wide.width = 120
	wide.selectedID = model.selectedID
	assertMouseDoesNotChangeSelection(t, wide, tea.MouseMsg{
		X:      wide.canvasWidth(),
		Y:      lipgloss.Height(wide.renderHeader()) + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	assertMouseDoesNotChangeSelection(t, wide, tea.MouseMsg{
		X:      wide.canvasWidth() + 1,
		Y:      lipgloss.Height(wide.renderHeader()) + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	withHelp := model
	withHelp.showHelp = true
	withHelp.selectedID = model.selectedID
	helpY := lipgloss.Height(withHelp.renderHeader()) + withHelp.canvasHeight()
	assertMouseDoesNotChangeSelection(t, withHelp, tea.MouseMsg{
		X:      2,
		Y:      helpY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
}

func TestMouseClickRequiresLeftPressAndIgnoresSearch(t *testing.T) {
	model := mouseTopologyTestModel()
	model.selectedID = domain.MachineID("prod", "0")
	appID := domain.AppID("prod", "postgresql")
	width, height := model.canvasWidth(), model.canvasHeight()
	fitted := model.withFitOffset(width, height, model.topologyNodeIDs())
	appBox := mustNodeBox(t, fitted, appID, width, height)
	point := routePoint{x: appBox.x + 1, y: appBox.y}

	for name, msg := range map[string]tea.MouseMsg{
		"right": {
			X:      point.x,
			Y:      canvasToTerminalY(model, point.y),
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonRight,
		},
		"middle": {
			X:      point.x,
			Y:      canvasToTerminalY(model, point.y),
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonMiddle,
		},
		"release": {
			X:      point.x,
			Y:      canvasToTerminalY(model, point.y),
			Action: tea.MouseActionRelease,
			Button: tea.MouseButtonLeft,
		},
		"motion": {
			X:      point.x,
			Y:      canvasToTerminalY(model, point.y),
			Action: tea.MouseActionMotion,
			Button: tea.MouseButtonLeft,
		},
		"wheel": {
			X:      point.x,
			Y:      canvasToTerminalY(model, point.y),
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelDown,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertMouseDoesNotChangeSelection(t, model, msg)
		})
	}

	searching := model
	searching.searching = true
	assertMouseDoesNotChangeSelection(t, searching, leftClickCanvas(searching, point))
}

func TestMouseRouteHitRequiresExactRouteOrLabelCell(t *testing.T) {
	model := relationTestModel()
	model.width = 100
	model.height = 22
	model.selectedID = domain.AppID("prod", "postgresql")
	width, height := model.canvasWidth(), model.canvasHeight()
	fitted := model.withFitOffset(width, height, model.topologyNodeIDs())
	edge := fitted.graph.Edges[domain.RelationID("prod", "postgresql:db", "api:database")]
	route, ok := fitted.relationRoute(edge, width, height)
	if !ok {
		t.Fatal("expected relation route")
	}
	blank, ok := nearbyBlankRoutePoint(fitted, route, width, height)
	if !ok {
		t.Fatalf("could not find a nearby blank cell for route %+v", route)
	}
	assertMouseDoesNotChangeSelection(t, model, leftClickCanvas(model, blank))
}

func TestMouseClickSelectsMachinesViewRowsAndBoxes(t *testing.T) {
	model := mouseTopologyTestModel()
	model.view = ViewMachines
	width, height := model.canvasWidth(), model.canvasHeight()
	fitted := model.withFitOffset(width, height, model.machineNodeIDs())
	machineID := domain.MachineID("prod", "0")
	unitID := domain.UnitID("prod", "postgresql/0")
	box := mustNodeBox(t, fitted, machineID, width, height)
	row := mustMachineRow(t, fitted, machineID, unitID, box)

	assertLeftClickSelects(t, model, routePoint{x: box.x + 2, y: box.y + 1}, machineID)
	assertLeftClickSelects(t, model, routePoint{x: box.x + 2, y: row}, unitID)
}

func TestMouseClickSelectsProblemsViewCards(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	model.width = 80
	model.height = 20
	model.view = ViewProblems
	model.hasGraph = true
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{ID: appID, Label: "postgresql", Type: domain.NodeApplication, Status: domain.StatusBlocked}
	model.graph.Nodes[unitID] = domain.Node{ID: unitID, Label: "postgresql/0", Type: domain.NodeUnit, Status: domain.StatusActive}
	model.graph.Order = []string{unitID, appID}

	assertLeftClickSelects(t, model, routePoint{x: 3, y: 3}, appID)

	model.selectedID = appID
	assertMouseDoesNotChangeSelection(t, model, leftClickCanvas(model, routePoint{x: 70, y: 15}))
}

func mouseTopologyTestModel() Model {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	apiID := domain.AppID("prod", "api")
	unitID := domain.UnitID("prod", "postgresql/0")
	machineID := domain.MachineID("prod", "0")
	attachedStorageID := domain.StorageID("prod", "data/0")
	standaloneStorageID := domain.StorageID("prod", "logs/0")
	appUnitEdgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
	machineEdgeID := domain.EdgeID(domain.EdgeUnitOnMachine, unitID, machineID)
	storageEdgeID := domain.EdgeID(domain.EdgeStorageAttached, unitID, attachedStorageID)
	relationEdgeID := domain.RelationID("prod", "postgresql:db", "api:database")

	model.width = 94
	model.height = 34
	model.view = ViewTopology
	model.hasGraph = true
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{ID: appID, Label: "postgresql", Type: domain.NodeApplication, Status: domain.StatusActive, Current: domain.Position{X: 4, Y: 2}}
	model.graph.Nodes[apiID] = domain.Node{ID: apiID, Label: "api", Type: domain.NodeApplication, Status: domain.StatusActive, Current: domain.Position{X: 64, Y: 2}}
	model.graph.Nodes[unitID] = domain.Node{ID: unitID, Label: "postgresql/0", Type: domain.NodeUnit, Status: domain.StatusActive, Current: domain.Position{X: 6, Y: 5}}
	model.graph.Nodes[machineID] = domain.Node{ID: machineID, Label: "machine 0", Type: domain.NodeMachine, Status: domain.StatusActive, Current: domain.Position{X: 44, Y: 14}}
	model.graph.Nodes[attachedStorageID] = domain.Node{ID: attachedStorageID, Label: "data/0", Type: domain.NodeStorage, Status: domain.StatusActive, Current: domain.Position{X: 8, Y: 6}}
	model.graph.Nodes[standaloneStorageID] = domain.Node{ID: standaloneStorageID, Label: "logs/0", Type: domain.NodeStorage, Status: domain.StatusActive, Current: domain.Position{X: 4, Y: 24}}
	model.graph.Edges[appUnitEdgeID] = domain.Edge{ID: appUnitEdgeID, SourceID: appID, TargetID: unitID, Type: domain.EdgeAppHasUnit}
	model.graph.Edges[machineEdgeID] = domain.Edge{ID: machineEdgeID, SourceID: unitID, TargetID: machineID, Type: domain.EdgeUnitOnMachine}
	model.graph.Edges[storageEdgeID] = domain.Edge{ID: storageEdgeID, SourceID: unitID, TargetID: attachedStorageID, Type: domain.EdgeStorageAttached}
	model.graph.Edges[relationEdgeID] = domain.Edge{ID: relationEdgeID, SourceID: appID, TargetID: apiID, Type: domain.EdgeRelation, Label: "db"}
	model.graph.Order = []string{appID, unitID, attachedStorageID, apiID, machineID, standaloneStorageID}
	model.graph.EdgeOrder = []string{appUnitEdgeID, machineEdgeID, storageEdgeID, relationEdgeID}
	return model
}

func assertLeftClickSelects(t *testing.T, model Model, point routePoint, want string) {
	t.Helper()
	updated := updateWithMouse(t, model, leftClickCanvas(model, point))
	if updated.selectedID != want {
		t.Fatalf("selectedID = %q, want %q for click at %+v", updated.selectedID, want, point)
	}
}

func assertMouseDoesNotChangeSelection(t *testing.T, model Model, msg tea.MouseMsg) {
	t.Helper()
	want := model.selectedID
	updated := updateWithMouse(t, model, msg)
	if updated.selectedID != want {
		t.Fatalf("selectedID changed from %q to %q for mouse msg %+v", want, updated.selectedID, msg)
	}
}

func updateWithMouse(t *testing.T, model Model, msg tea.MouseMsg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T, want tui.Model", updated)
	}
	return result
}

func leftClickCanvas(model Model, point routePoint) tea.MouseMsg {
	return tea.MouseMsg{
		X:      point.x,
		Y:      canvasToTerminalY(model, point.y),
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
}

func canvasToTerminalY(model Model, y int) int {
	return y + lipgloss.Height(model.renderHeader())
}

func mustNodeBox(t *testing.T, model Model, id string, width, height int) renderBox {
	t.Helper()
	box, ok := model.nodeRenderBoxIn(id, width, height)
	if !ok {
		t.Fatalf("missing node box for %s", id)
	}
	return box
}

func mustApplicationRow(t *testing.T, model Model, appID, rowID string, box renderBox) int {
	t.Helper()
	for _, row := range model.applicationSelectableRows(appID, box) {
		if row.id == rowID {
			return row.y
		}
	}
	t.Fatalf("missing application row %s in %s", rowID, appID)
	return 0
}

func mustMachineRow(t *testing.T, model Model, machineID, rowID string, box renderBox) int {
	t.Helper()
	for _, row := range model.machineSelectableRows(machineID, box) {
		if row.id == rowID {
			return row.y
		}
	}
	t.Fatalf("missing machine row %s in %s", rowID, machineID)
	return 0
}

func nearbyBlankRoutePoint(model Model, route relationRoute, width, height int) (routePoint, bool) {
	for _, point := range routePathPoints(route) {
		for _, candidate := range []routePoint{
			{x: point.x + 1, y: point.y + 1},
			{x: point.x + 1, y: point.y - 1},
			{x: point.x - 1, y: point.y + 1},
			{x: point.x - 1, y: point.y - 1},
		} {
			if candidate.x < 0 || candidate.y < 0 || candidate.x >= width || candidate.y >= height {
				continue
			}
			if routeHit(candidate, route) {
				continue
			}
			if _, ok := model.hitTestCanvasPoint(candidate, width, height); ok {
				continue
			}
			return candidate, true
		}
	}
	return routePoint{}, false
}
