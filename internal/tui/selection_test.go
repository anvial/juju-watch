package tui

import (
	"testing"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestVisibleIDsIncludesTopologyNodes(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	model.graph = domain.BuildGraph(domain.State{
		Model: "prod",
		Applications: map[string]domain.Application{
			"postgresql": {Name: "postgresql", Status: domain.StatusActive, Units: []string{"postgresql/0"}},
		},
		Units: map[string]domain.Unit{
			"postgresql/0": {Name: "postgresql/0", AppName: "postgresql", WorkloadStatus: domain.StatusActive},
		},
		Machines: map[string]domain.Machine{},
		Storage:  map[string]domain.Storage{},
	})
	model.hasGraph = true
	model.view = ViewTopology
	ids := model.visibleIDs()
	if len(ids) != 2 {
		t.Fatalf("visible IDs = %v", ids)
	}
}
