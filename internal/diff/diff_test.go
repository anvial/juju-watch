package diff

import (
	"os"
	"testing"

	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/juju"
)

func TestGraphsDetectsAddedNode(t *testing.T) {
	before := graphFromFixture(t, "../../testdata/juju/status/healthy.json")
	after := graphFromFixture(t, "../../testdata/juju/status/changed.json")
	result := Graphs(before, after)
	if !result.TopologyChanged {
		t.Fatal("expected topology change")
	}
	id := domain.AppID("prod", "redis")
	if result.Nodes[id].Kind != Added {
		t.Fatalf("redis change = %q", result.Nodes[id].Kind)
	}
}

func graphFromFixture(t *testing.T, path string) domain.Graph {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state, err := juju.ParseStatus("prod", data)
	if err != nil {
		t.Fatal(err)
	}
	return domain.BuildGraph(state)
}
