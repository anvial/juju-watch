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
	if !strings.Contains(rendered, "runs on") {
		t.Fatalf("placement label missing: %q", rendered)
	}
	if !strings.Contains(rendered, "╌") || !strings.Contains(rendered, "╎") {
		t.Fatalf("placement dashed edge missing: %q", rendered)
	}
}

func TestPlacementEdgesCollapseAppMachineUnits(t *testing.T) {
	model := placementTestModel(2)

	edges := model.placementEdges()
	if len(edges) != 1 {
		t.Fatalf("placement edges = %d, want one collapsed app-machine edge", len(edges))
	}
	if edges[0].label != "runs on 2" {
		t.Fatalf("placement label = %q, want unit count", edges[0].label)
	}
}

func TestPlacementRouteAvoidsTopologyBoxes(t *testing.T) {
	model := placementTestModel(1)
	edge := model.placementEdges()[0]
	route, ok := model.placementRoute(edge, 90, 24)
	if !ok {
		t.Fatal("expected placement route")
	}
	if route.arrow != '◀' {
		t.Fatalf("placement should point from application to machine: %+v", route)
	}
	boxes := model.topologyRenderBoxesIn(90, 24)
	for _, segment := range route.segments() {
		for _, box := range boxes {
			if segmentIntersectsBox(segment, box) {
				t.Fatalf("placement segment intersects box: segment=%+v box=%+v route=%+v", segment, box, route)
			}
		}
	}
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
