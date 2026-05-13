package tui

import (
	"strings"
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestRelationRouteAvoidsTopologyBoxes(t *testing.T) {
	model := relationTestModel()
	storageID := domain.StorageID("prod", "data/0")
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "data/0",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 32, Y: 1},
	}
	model.graph.Order = append(model.graph.Order, storageID)

	edge := model.graph.Edges[domain.RelationID("prod", "postgresql:db", "api:database")]
	route, ok := model.relationRoute(edge, 100, 24)
	if !ok {
		t.Fatal("expected relation route")
	}
	boxes := model.topologyRenderBoxes()
	for _, segment := range route.segments() {
		for _, box := range boxes {
			if segmentIntersectsBox(segment, box) {
				t.Fatalf("relation segment intersects box: segment=%+v box=%+v route=%+v", segment, box, route)
			}
		}
	}
}

func TestRenderTopologyRelationUsesRecognizableLineAndLabel(t *testing.T) {
	model := relationTestModel()
	rendered := model.renderTopology(100, 20)
	if !strings.Contains(rendered, "db") {
		t.Fatalf("relation label missing: %q", rendered)
	}
	if !strings.Contains(rendered, "━") {
		t.Fatalf("relation line glyph missing: %q", rendered)
	}
}

func TestRelationRouteUsesStraightSideLineForAlignedApps(t *testing.T) {
	model := relationTestModel()
	targetID := domain.AppID("prod", "api")
	target := model.graph.Nodes[targetID]
	target.Current = domain.Position{X: 30, Y: 1}
	model.graph.Nodes[targetID] = target

	edgeID := domain.RelationID("prod", "postgresql:db", "api:database")
	edge := model.graph.Edges[edgeID]
	edge.Label = "database-provider-to-api"
	model.graph.Edges[edgeID] = edge

	route, ok := model.relationRoute(edge, 120, 20)
	if !ok {
		t.Fatal("expected relation route")
	}
	if len(route.points) != 2 || route.points[0].y != route.points[1].y {
		t.Fatalf("route should be a straight horizontal side line: %+v", route)
	}
	if route.label == "" {
		t.Fatalf("straight route should still draw a fitted label fragment: %+v", route)
	}
}

func relationTestModel() Model {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	sourceID := domain.AppID("prod", "postgresql")
	targetID := domain.AppID("prod", "api")
	edgeID := domain.RelationID("prod", "postgresql:db", "api:database")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[sourceID] = domain.Node{
		ID:      sourceID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 1, Y: 1},
	}
	model.graph.Nodes[targetID] = domain.Node{
		ID:      targetID,
		Label:   "api",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 60, Y: 1},
	}
	model.graph.Edges[edgeID] = domain.Edge{
		ID:       edgeID,
		SourceID: sourceID,
		TargetID: targetID,
		Type:     domain.EdgeRelation,
		Label:    "db",
	}
	model.graph.Order = []string{sourceID, targetID}
	model.graph.EdgeOrder = []string{edgeID}
	model.hasGraph = true
	model.view = ViewTopology
	return model
}
