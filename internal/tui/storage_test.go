package tui

import (
	"strings"
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestRenderTopologyUsesStorageMountedDeviceCard(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	storageID := domain.StorageID("prod", "data/3")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[storageID] = domain.Node{
		ID:     storageID,
		Label:  "data/3",
		Type:   domain.NodeStorage,
		Status: domain.StatusActive,
		Metadata: map[string]string{
			"kind":     "filesystem",
			"location": "/var/lib/postgresql",
		},
		Current: domain.Position{X: 1, Y: 1},
		Target:  domain.Position{X: 1, Y: 1},
	}
	model.graph.Order = []string{storageID}
	model.hasGraph = true
	model.view = ViewTopology

	rendered := model.renderTopology(50, 8)
	if !strings.Contains(rendered, storageIcon+" data/3") {
		t.Fatalf("storage icon/title missing: %q", rendered)
	}
	if !strings.Contains(rendered, "/var/lib/postgresql") {
		t.Fatalf("mount path missing: %q", rendered)
	}
	if !strings.Contains(rendered, "filesystem ● active") {
		t.Fatalf("storage footer missing: %q", rendered)
	}
	if strings.Contains(rendered, "storage          ") {
		t.Fatalf("storage should not render through generic compact node: %q", rendered)
	}
}

func TestRenderTopologyNestsAttachedStorageInApplication(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	storageID := domain.StorageID("prod", "data/3")
	appUnitEdgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
	attachmentEdgeID := domain.EdgeID(domain.EdgeStorageAttached, unitID, storageID)
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 4, Y: 1},
	}
	model.graph.Nodes[unitID] = domain.Node{
		ID:      unitID,
		Label:   "postgresql/0",
		Type:    domain.NodeUnit,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 6, Y: 4},
	}
	model.graph.Nodes[storageID] = domain.Node{
		ID:      storageID,
		Label:   "data/3",
		Type:    domain.NodeStorage,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 8, Y: 5},
		Metadata: map[string]string{
			"kind": "filesystem",
		},
	}
	model.graph.Edges[appUnitEdgeID] = domain.Edge{
		ID:       appUnitEdgeID,
		SourceID: appID,
		TargetID: unitID,
		Type:     domain.EdgeAppHasUnit,
	}
	model.graph.Edges[attachmentEdgeID] = domain.Edge{
		ID:       attachmentEdgeID,
		SourceID: unitID,
		TargetID: storageID,
		Type:     domain.EdgeStorageAttached,
		Label:    "storage",
	}
	model.graph.Order = []string{appID, unitID, storageID}
	model.graph.EdgeOrder = []string{appUnitEdgeID, attachmentEdgeID}
	model.hasGraph = true
	model.view = ViewTopology

	rendered := model.renderTopology(80, 12)
	if !strings.Contains(rendered, "╰─ "+storageIcon+" data/3") {
		t.Fatalf("nested storage row missing: %q", rendered)
	}
	if strings.Contains(rendered, "filesystem ● active") {
		t.Fatalf("attached storage should not render as standalone storage card: %q", rendered)
	}
	for _, id := range model.topologyNodeIDs() {
		if id == storageID {
			t.Fatalf("attached storage should not be a standalone topology render node")
		}
	}
}

func TestTopologySelectionOrdersAttachedStorageAfterOwningUnit(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	storageID := domain.StorageID("prod", "data/3")
	appUnitEdgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
	attachmentEdgeID := domain.EdgeID(domain.EdgeStorageAttached, unitID, storageID)
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{ID: appID, Label: "postgresql", Type: domain.NodeApplication, Status: domain.StatusActive}
	model.graph.Nodes[unitID] = domain.Node{ID: unitID, Label: "postgresql/0", Type: domain.NodeUnit, Status: domain.StatusActive}
	model.graph.Nodes[storageID] = domain.Node{ID: storageID, Label: "data/3", Type: domain.NodeStorage, Status: domain.StatusActive}
	model.graph.Edges[appUnitEdgeID] = domain.Edge{ID: appUnitEdgeID, SourceID: appID, TargetID: unitID, Type: domain.EdgeAppHasUnit}
	model.graph.Edges[attachmentEdgeID] = domain.Edge{ID: attachmentEdgeID, SourceID: unitID, TargetID: storageID, Type: domain.EdgeStorageAttached}
	model.graph.Order = []string{appID, unitID, storageID}
	model.hasGraph = true
	model.view = ViewTopology

	ids := model.visibleIDs()
	want := []string{appID, unitID, storageID}
	if len(ids) < len(want) {
		t.Fatalf("visible IDs = %v, want prefix %v", ids, want)
	}
	for index, id := range want {
		if ids[index] != id {
			t.Fatalf("visible IDs = %v, want prefix %v", ids, want)
		}
	}
}
