package tui

import (
	"strings"
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestRenderTopologyShowsMachineAndPlacementEdge(t *testing.T) {
	model := placementTestModel(1)

	rendered := model.renderTopology(90, 24)
	if !strings.Contains(rendered, machineIcon+" machine 0") {
		t.Fatalf("machine icon/title missing: %q", rendered)
	}
	if !strings.Contains(rendered, "╭─"+machineIcon+" machine 0") {
		t.Fatalf("machine title should be embedded in the top border: %q", rendered)
	}
	if strings.Contains(rendered, machineIcon+" machine 0 "+StatusSymbol(domain.StatusActive)) {
		t.Fatalf("machine title should not be duplicated as a body row: %q", rendered)
	}
	if !strings.Contains(rendered, "active "+StatusSymbol(domain.StatusActive)) {
		t.Fatalf("machine status line missing status symbol: %q", rendered)
	}
	if strings.Contains(rendered, "runs on") {
		t.Fatalf("collapsed placement label should not be rendered: %q", rendered)
	}
	if strings.Contains(rendered, "╌") || strings.Contains(rendered, "╎") {
		t.Fatalf("old dashed placement glyphs should not be rendered: %q", rendered)
	}
	if !strings.ContainsAny(rendered, "╭╮╰╯") {
		t.Fatalf("rounded placement connector glyph missing: %q", rendered)
	}
}

func TestRenderTopologySelectedMachineUsesDoubleTitleBorder(t *testing.T) {
	model := placementTestModel(1)
	model.selectedID = domain.MachineID("prod", "0")

	rendered := model.renderTopology(90, 24)
	if !strings.Contains(rendered, "╔═"+machineIcon+" machine 0") {
		t.Fatalf("selected machine title border missing: %q", rendered)
	}
}

func TestPlacementEdgesUseUnitMachineEdges(t *testing.T) {
	model := placementTestModel(2)

	edges := model.placementEdges()
	if len(edges) != 2 {
		t.Fatalf("placement edges = %d, want one per unit", len(edges))
	}
	appID := domain.AppID("prod", "postgresql")
	machineID := domain.MachineID("prod", "0")
	for index, edge := range edges {
		unitID := domain.UnitID("prod", "postgresql/"+string(rune('0'+index)))
		if edge.appID != appID || edge.machineID != machineID || edge.unitID != unitID {
			t.Fatalf("placement edge[%d] = %+v, want app=%s unit=%s machine=%s", index, edge, appID, unitID, machineID)
		}
		if edge.label != model.graph.Nodes[unitID].Label {
			t.Fatalf("placement edge[%d] label = %q, want unit label %q", index, edge.label, model.graph.Nodes[unitID].Label)
		}
		if edge.groupIndex != index || edge.groupSize != len(edges) {
			t.Fatalf("placement edge[%d] group = %d/%d, want %d/%d", index, edge.groupIndex, edge.groupSize, index, len(edges))
		}
	}
}

func TestPlacementRouteAnchorsToUnitRows(t *testing.T) {
	model := placementTestModel(2)

	for _, edge := range model.placementEdges() {
		route, ok := model.placementRoute(edge, 90, 24)
		if !ok {
			t.Fatalf("expected placement route for %s", edge.unitID)
		}
		appBox, ok := model.nodeRenderBoxIn(edge.appID, 90, 24)
		if !ok {
			t.Fatalf("missing app box for %s", edge.appID)
		}
		machineBox, ok := model.nodeRenderBoxIn(edge.machineID, 90, 24)
		if !ok {
			t.Fatalf("missing machine box for %s", edge.machineID)
		}
		appRow, ok := model.applicationUnitRow(edge.appID, edge.unitID, appBox)
		if !ok {
			t.Fatalf("missing app unit row for %s", edge.unitID)
		}
		machineRow, ok := model.machineUnitRow(edge.machineID, edge.unitID, machineBox)
		if !ok {
			t.Fatalf("missing machine unit row for %s", edge.unitID)
		}
		if route.points[0].y != appRow {
			t.Fatalf("source anchor row = %d, want app unit row %d for route %+v", route.points[0].y, appRow, route)
		}
		if route.arrowAt.y != machineRow {
			t.Fatalf("target anchor row = %d, want machine unit row %d for route %+v", route.arrowAt.y, machineRow, route)
		}
		if route.points[0].x != appBox.x-1 || route.arrowAt.x != machineBox.x+machineBox.width || route.arrow != '◀' {
			t.Fatalf("placement should point from application row into machine row: route=%+v appBox=%+v machineBox=%+v", route, appBox, machineBox)
		}
	}
}

func TestPlacementRoutesUseDistinctPreferredGroupLanes(t *testing.T) {
	model := placementTestModel(3)
	edges := model.placementEdges()
	lanes := map[int]string{}

	for _, edge := range edges {
		route, ok := model.placementRoute(edge, 90, 24)
		if !ok {
			t.Fatalf("expected placement route for %s", edge.unitID)
		}
		laneX, ok := placementMiddleLaneX(route)
		if !ok {
			t.Fatalf("expected middle vertical placement lane for %s: %+v", edge.unitID, route)
		}
		if other, exists := lanes[laneX]; exists {
			t.Fatalf("placement lane %d is shared by %s and %s", laneX, other, edge.unitID)
		}
		lanes[laneX] = edge.unitID
	}

	if len(lanes) != len(edges) {
		t.Fatalf("distinct placement lanes = %d, want %d", len(lanes), len(edges))
	}
}

func TestPlacementGroupOffsetsUseTwoCellSpacing(t *testing.T) {
	cases := []struct {
		index int
		size  int
		want  int
	}{
		{index: 0, size: 2, want: -1},
		{index: 1, size: 2, want: 1},
		{index: 0, size: 3, want: -2},
		{index: 1, size: 3, want: 0},
		{index: 2, size: 3, want: 2},
	}
	for _, tc := range cases {
		if got := placementGroupOffset(tc.index, tc.size); got != tc.want {
			t.Fatalf("placement group offset index=%d size=%d = %d, want %d", tc.index, tc.size, got, tc.want)
		}
	}
}

func TestPlacementRoutesInSameGroupAvoidLongSharedRuns(t *testing.T) {
	model := placementTestModel(4)
	edges := model.placementEdges()
	routes := make([]relationRoute, 0, len(edges))

	for _, edge := range edges {
		route, ok := model.placementRoute(edge, 90, 24)
		if !ok {
			t.Fatalf("expected placement route for %s", edge.unitID)
		}
		routes = append(routes, route)
	}

	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			if run := maxSharedContiguousRouteRun(routes[i], routes[j]); run > 1 {
				t.Fatalf("placement routes %d and %d share a route run of %d cells: routeA=%+v routeB=%+v", i, j, run, routes[i], routes[j])
			}
		}
	}
}

func TestPlacementFallbackLaneCandidatesAreStaggeredByGroup(t *testing.T) {
	sourceBox := renderBox{x: 44, y: 2, width: appBoxWidth, height: 8}
	targetBox := renderBox{x: 4, y: 8, width: appBoxWidth + 4, height: 8}

	first := placementLaneYCandidates(5, 11, sourceBox, targetBox, 0, 2, 24)
	second := placementLaneYCandidates(6, 12, sourceBox, targetBox, 1, 2, 24)

	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("expected fallback lane candidates, got first=%v second=%v", first, second)
	}
	if first[0] == second[0] {
		t.Fatalf("fallback lane candidates should be staggered, got first=%d second=%d", first[0], second[0])
	}
}

func TestPlacementRouteAvoidsTopologyBoxes(t *testing.T) {
	model := placementTestModel(2)
	storageID := domain.StorageID("prod", "data/0")
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "data/0",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 34, Y: 9},
	}
	model.graph.Order = append(model.graph.Order, storageID)

	for _, edge := range model.placementEdges() {
		route, ok := model.placementRoute(edge, 90, 24)
		if !ok {
			t.Fatalf("expected placement route for %s", edge.unitID)
		}
		boxes := model.topologyRenderBoxesIn(90, 24)
		for _, segment := range route.segments() {
			for _, box := range boxes {
				if segmentIntersectsBox(segment, box) {
					t.Fatalf("placement segment intersects box: edge=%+v segment=%+v box=%+v route=%+v", edge, segment, box, route)
				}
			}
		}
	}
}

func TestPlacementRouteUsesRoundedConnectorCorners(t *testing.T) {
	model := placementTestModel(1)
	edge := model.placementEdges()[0]
	route, ok := model.placementRoute(edge, 90, 24)
	if !ok {
		t.Fatal("expected placement route")
	}
	found := false
	for _, point := range route.joints() {
		if strings.ContainsRune("╭╮╰╯", placementJoint(route, point)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("placement route should have a rounded corner: %+v", route)
	}
}

func TestPlacementRouteSkipsEdgesWithHiddenUnitRows(t *testing.T) {
	model := placementTestModel(2)

	for _, edge := range model.placementEdges() {
		if _, ok := model.placementRoute(edge, 90, 11); ok {
			t.Fatalf("placement route should be skipped when unit row is outside the canvas: %+v", edge)
		}
	}
}

func TestPlacementSelectionScopesToUnitMachineAndApplication(t *testing.T) {
	model := placementTestModel(2)
	edges := model.placementEdges()

	model.selectedID = edges[0].unitID
	if got := selectedPlacementCount(model, edges); got != 1 {
		t.Fatalf("selected unit should highlight one placement edge, got %d", got)
	}

	model.selectedID = edges[0].machineID
	if got := selectedPlacementCount(model, edges); got != len(edges) {
		t.Fatalf("selected machine should highlight all target placement edges, got %d", got)
	}

	model.selectedID = edges[0].appID
	if got := selectedPlacementCount(model, edges); got != len(edges) {
		t.Fatalf("selected application should highlight all app placement edges, got %d", got)
	}
}

func selectedPlacementCount(model Model, edges []placementEdge) int {
	count := 0
	for _, edge := range edges {
		if model.placementSelected(edge) {
			count++
		}
	}
	return count
}

func placementMiddleLaneX(route relationRoute) (int, bool) {
	if len(route.points) == 0 {
		return 0, false
	}
	startX := route.points[0].x
	targetX := route.arrowAt.x
	for _, segment := range route.segments() {
		if segment.from.x == segment.to.x && segment.from.x != startX && segment.from.x != targetX {
			return segment.from.x, true
		}
	}
	return 0, false
}

func maxSharedContiguousRouteRun(first, second relationRoute) int {
	shared := map[routePoint]bool{}
	for _, point := range routePathPoints(second) {
		shared[point] = true
	}
	longest := 0
	current := 0
	for _, point := range routePathPoints(first) {
		if shared[point] {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

func placementTestModel(unitCount int) Model {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	machineID := domain.MachineID("prod", "0")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 44, Y: 2},
	}
	model.graph.Nodes[machineID] = domain.Node{
		ID:     machineID,
		Label:  "machine 0",
		Type:   domain.NodeMachine,
		Status: domain.StatusActive,
		Metadata: map[string]string{
			"ip_address": "10.20.0.15",
		},
		Current: domain.Position{X: 4, Y: 8},
	}
	model.graph.Order = []string{appID}
	for index := 0; index < unitCount; index++ {
		unitID := domain.UnitID("prod", "postgresql/"+string(rune('0'+index)))
		model.graph.Nodes[unitID] = domain.Node{
			ID:      unitID,
			Label:   "postgresql/" + string(rune('0'+index)),
			Type:    domain.NodeUnit,
			Status:  domain.StatusActive,
			Current: domain.Position{X: 46, Y: float64(5 + index)},
		}
		appEdgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
		machineEdgeID := domain.EdgeID(domain.EdgeUnitOnMachine, unitID, machineID)
		model.graph.Edges[appEdgeID] = domain.Edge{
			ID:       appEdgeID,
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
		model.graph.Order = append(model.graph.Order, unitID)
		model.graph.EdgeOrder = append(model.graph.EdgeOrder, appEdgeID, machineEdgeID)
	}
	model.graph.Order = append(model.graph.Order, machineID)
	model.hasGraph = true
	model.view = ViewTopology
	return model
}
