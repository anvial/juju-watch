package tui

import (
	"strings"
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestRenderTopologyUsesApplicationTitleRibbon(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	edgeID := domain.EdgeID(domain.EdgeAppHasUnit, appID, unitID)
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 1, Y: 1},
		Target:  domain.Position{X: 1, Y: 1},
	}
	model.graph.Nodes[unitID] = domain.Node{
		ID:      unitID,
		Label:   "postgresql/0",
		Type:    domain.NodeUnit,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 3, Y: 4},
		Target:  domain.Position{X: 3, Y: 4},
	}
	model.graph.Edges[edgeID] = domain.Edge{
		ID:       edgeID,
		SourceID: appID,
		TargetID: unitID,
		Type:     domain.EdgeAppHasUnit,
	}
	model.graph.Order = []string{appID, unitID}
	model.graph.EdgeOrder = []string{edgeID}
	model.hasGraph = true
	model.view = ViewTopology

	rendered := model.renderTopology(50, 8)
	if !strings.Contains(rendered, applicationIcon+" postgresql") {
		t.Fatalf("application icon/title missing: %q", rendered)
	}
	if !strings.Contains(rendered, "active ●") {
		t.Fatalf("application status missing: %q", rendered)
	}
	if !strings.Contains(rendered, "units: 1") {
		t.Fatalf("unit count missing: %q", rendered)
	}
	if !strings.Contains(rendered, "● postgresql/0") {
		t.Fatalf("unit row missing: %q", rendered)
	}
}

func TestRenderTopologySelectedApplicationUsesDoubleTitleBorder(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	appID := domain.AppID("prod", "postgresql")
	model.graph = domain.NewGraph("prod")
	model.graph.Nodes[appID] = domain.Node{
		ID:      appID,
		Label:   "postgresql",
		Type:    domain.NodeApplication,
		Status:  domain.StatusActive,
		Current: domain.Position{X: 1, Y: 1},
		Target:  domain.Position{X: 1, Y: 1},
	}
	model.graph.Order = []string{appID}
	model.hasGraph = true
	model.view = ViewTopology
	model.selectedID = appID

	rendered := model.renderTopology(50, 8)
	if !strings.Contains(rendered, "╔═"+applicationIcon+" postgresql") {
		t.Fatalf("selected application title border missing: %q", rendered)
	}
}
