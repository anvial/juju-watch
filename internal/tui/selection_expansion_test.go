package tui

import (
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestSelectedApplicationStaysCompactWhenContentFits(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 4, Y: 2},
	}
	model.graph.Order = []string{appID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = appID

	box, ok := model.nodeRenderBoxIn(appID, 100, 20)
	if !ok {
		t.Fatal("missing application box")
	}
	if box.width != appBoxWidth {
		t.Fatalf("width = %d, want compact width %d", box.width, appBoxWidth)
	}
}

func TestSelectedApplicationExpandsWhenContentDoesNotFit(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql-main-database-cluster",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 4, Y: 2},
	}
	model.graph.Order = []string{appID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = appID

	box, ok := model.nodeRenderBoxIn(appID, 100, 20)
	if !ok {
		t.Fatal("missing application box")
	}
	if box.width != selectedAppBoxWidth {
		t.Fatalf("width = %d, want expanded width %d", box.width, selectedAppBoxWidth)
	}
}

func TestSelectedUnitExpandsParentApplicationWhenSafe(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 4, Y: 2},
	}
	model.graph.Nodes[unitID] = domain.Node{
		ID:      unitID,
		Label:   "postgresql-main-database-cluster/0",
		Type:    domain.NodeUnit,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 6, Y: 5},
	}
	model.graph.Edges[domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)] = domain.Edge{
		ID:       domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID),
		SourceID: appID,
		TargetID: unitID,
		Type:     domain.EdgeAppHasUnit,
	}
	model.graph.Order = []string{appID, unitID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = unitID

	box, ok := model.nodeRenderBoxIn(appID, 100, 20)
	if !ok {
		t.Fatal("missing application box")
	}
	if box.width != selectedAppBoxWidth {
		t.Fatalf("width = %d, want parent app expanded width %d", box.width, selectedAppBoxWidth)
	}
}

func TestSelectedApplicationExpansionFallsBackWhenBlocked(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	selectedID := domain.AppID("prod", "postgresql")
	blockerID := domain.AppID("prod", "api")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[selectedID] = domain.Node{
		ID:      selectedID,
		Label:   "postgresql-main-database-cluster",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 1, Y: 1},
	}
	model.graph.Nodes[blockerID] = domain.Node{
		ID:      blockerID,
		Label:   "api",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 30, Y: 1},
	}
	model.graph.Order = []string{selectedID, blockerID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = selectedID

	box, ok := model.nodeRenderBoxIn(selectedID, 100, 20)
	if !ok {
		t.Fatal("missing application box")
	}
	if box.width != appBoxWidth {
		t.Fatalf("width = %d, want compact fallback width %d", box.width, appBoxWidth)
	}
}

func TestSelectedStorageExpandsWhenContentDoesNotFit(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	storageID := domain.StorageID("prod", "archive/2")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "archive-with-very-long-storage-name/2",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 4, Y: 2},
	}
	model.graph.Order = []string{storageID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = storageID

	box, ok := model.nodeRenderBoxIn(storageID, 100, 20)
	if !ok {
		t.Fatal("missing storage box")
	}
	if box.width != selectedStorageBoxWidth || box.height != selectedStorageBoxHeight {
		t.Fatalf("box = %+v, want selected storage size %dx%d", box, selectedStorageBoxWidth, selectedStorageBoxHeight)
	}
}

func TestSelectedAttachedStorageExpandsParentApplicationWhenContentDoesNotFit(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	storageID := domain.StorageID("prod", "archive/2")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 4, Y: 2},
	}
	model.graph.Nodes[unitID] = domain.Node{
		ID:      unitID,
		Label:   "postgresql/0",
		Type:    domain.NodeUnit,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 6, Y: 5},
	}
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "archive-with-very-long-storage-name/2",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 8, Y: 6},
	}
	appUnitEdgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
	storageEdgeID := domain.EdgeID(domain.EdgeStorageAttached, unitID, storageID)
	model.graph.Edges[appUnitEdgeID] = domain.Edge{
		ID:       appUnitEdgeID,
		SourceID: appID,
		TargetID: unitID,
		Type:     domain.EdgeAppHasUnit,
	}
	model.graph.Edges[storageEdgeID] = domain.Edge{
		ID:       storageEdgeID,
		SourceID: unitID,
		TargetID: storageID,
		Type:     domain.EdgeStorageAttached,
	}
	model.graph.Order = []string{appID, unitID, storageID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = storageID

	box, ok := model.nodeRenderBoxIn(appID, 100, 20)
	if !ok {
		t.Fatal("missing application box")
	}
	if box.width != selectedAppBoxWidth {
		t.Fatalf("width = %d, want parent app expanded width %d", box.width, selectedAppBoxWidth)
	}
}

func TestRelationRouteAvoidsExpandedSelectedApplication(t *testing.T) {
	model := relationTestModel()
	sourceID := domain.AppID("prod", "postgresql")
	source := model.graph.Nodes[sourceID]
	source.Label = "postgresql-main-database-cluster"
	model.graph.Nodes[sourceID] = source
	edge := model.graph.Edges[domain.RelationID("prod", "postgresql:db", "api:database")]
	model.selectedID = sourceID

	route, ok := model.relationRoute(edge, 120, 24)
	if !ok {
		t.Fatal("expected relation route")
	}
	boxes := model.topologyRenderBoxesIn(120, 24)
	for _, segment := range route.segments() {
		for _, box := range boxes {
			if segmentIntersectsBox(segment, box) {
				t.Fatalf("relation segment intersects expanded box: segment=%+v box=%+v route=%+v", segment, box, route)
			}
		}
	}
}

func TestSelectedRelationUsesLongerLineLabel(t *testing.T) {
	model := relationTestModel()
	edgeID := domain.RelationID("prod", "postgresql:db", "api:database")
	edge := model.graph.Edges[edgeID]
	edge.Label = "database-provider-to-api"
	model.graph.Edges[edgeID] = edge
	model.selectedID = edgeID

	selectedRoute, ok := model.relationRoute(edge, 120, 20)
	if !ok {
		t.Fatal("expected selected relation route")
	}
	if selectedRoute.label != edge.Label {
		t.Fatalf("selected label = %q, want full %q", selectedRoute.label, edge.Label)
	}

	model.selectedID = ""
	compactRoute, ok := model.relationRoute(edge, 120, 20)
	if !ok {
		t.Fatal("expected compact relation route")
	}
	if len([]rune(compactRoute.label)) > relationLabelWidth {
		t.Fatalf("compact label = %q, want at most %d runes", compactRoute.label, relationLabelWidth)
	}
}

func TestSelectedRelationKeepsCompactLimitWhenLabelFits(t *testing.T) {
	model := relationTestModel()
	edgeID := domain.RelationID("prod", "postgresql:db", "api:database")
	edge := model.graph.Edges[edgeID]
	edge.Label = "db"
	model.graph.Edges[edgeID] = edge
	model.selectedID = edgeID

	if limit := model.relationLabelLimit(edge); limit != relationLabelWidth {
		t.Fatalf("limit = %d, want compact relation label width %d", limit, relationLabelWidth)
	}
}
