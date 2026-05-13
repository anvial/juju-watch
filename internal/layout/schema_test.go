package layout

import (
	"context"
	"os"
	"testing"

	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/juju"
)

func TestSchemaPreservesTargetsOnStatusOnlyChange(t *testing.T) {
	before := layoutFixture(t, "../../testdata/juju/status/healthy.json", nil)
	after := layoutFixture(t, "../../testdata/juju/status/problem.json", &before)

	id := domain.AppID("prod", "api-server")
	if before.Nodes[id].Target != after.Nodes[id].Target {
		t.Fatalf("target moved on status-only change: before=%v after=%v", before.Nodes[id].Target, after.Nodes[id].Target)
	}
}

func TestSchemaPreservesApplicationColumnWhenRelationScoreChanges(t *testing.T) {
	beforeState := stateWithApplications(map[string]domain.Status{
		"api":        domain.StatusActive,
		"postgresql": domain.StatusActive,
	})
	beforeState.Relations = []domain.Relation{{
		AppA:      "postgresql",
		EndpointA: "postgresql:db",
		RoleA:     "provider",
		AppB:      "api",
		EndpointB: "api:database",
		RoleB:     "requirer",
	}}
	before := layoutState(t, beforeState, nil)

	afterState := stateWithApplications(map[string]domain.Status{
		"api":        domain.StatusActive,
		"postgresql": domain.StatusActive,
	})
	afterState.Relations = []domain.Relation{{
		AppA:      "postgresql",
		EndpointA: "postgresql:db",
		RoleA:     "requirer",
		AppB:      "api",
		EndpointB: "api:database",
		RoleB:     "provider",
	}}
	after := layoutState(t, afterState, &before)

	apiID := domain.AppID("prod", "api")
	postgresID := domain.AppID("prod", "postgresql")
	if after.Nodes[postgresID].Target.X != before.Nodes[postgresID].Target.X {
		t.Fatalf("postgresql column moved after relation score changed: before=%v after=%v", before.Nodes[postgresID].Target, after.Nodes[postgresID].Target)
	}
	if after.Nodes[apiID].Target.X != before.Nodes[apiID].Target.X {
		t.Fatalf("api column moved after relation score changed: before=%v after=%v", before.Nodes[apiID].Target, after.Nodes[apiID].Target)
	}

	edge := after.Edges[domain.RelationID("prod", "postgresql:db", "api:database")]
	if edge.SourceID != apiID || edge.TargetID != postgresID {
		t.Fatalf("relation direction = %s -> %s, want provider api -> postgresql", edge.SourceID, edge.TargetID)
	}
}

func TestSchemaPlacesNewRelatedApplicationOppositePinnedNeighbor(t *testing.T) {
	beforeState := stateWithApplications(map[string]domain.Status{
		"api": domain.StatusActive,
	})
	before := layoutState(t, beforeState, nil)

	afterState := stateWithApplications(map[string]domain.Status{
		"api":   domain.StatusActive,
		"redis": domain.StatusActive,
	})
	afterState.Relations = []domain.Relation{{
		AppA:      "redis",
		EndpointA: "redis:cache",
		RoleA:     "provider",
		AppB:      "api",
		EndpointB: "api:cache",
		RoleB:     "requirer",
	}}
	after := layoutState(t, afterState, &before)

	apiID := domain.AppID("prod", "api")
	redisID := domain.AppID("prod", "redis")
	if after.Nodes[apiID].Target.X != before.Nodes[apiID].Target.X {
		t.Fatalf("pinned app moved when new relation appeared: before=%v after=%v", before.Nodes[apiID].Target, after.Nodes[apiID].Target)
	}
	if after.Nodes[redisID].Target.X <= after.Nodes[apiID].Target.X {
		t.Fatalf("new related app should be placed opposite pinned neighbor: api=%v redis=%v", after.Nodes[apiID].Target, after.Nodes[redisID].Target)
	}
}

func TestSchemaPreservesExistingApplicationOrderWhenStatusChanges(t *testing.T) {
	beforeState := stateWithApplications(map[string]domain.Status{
		"alpha": domain.StatusActive,
		"beta":  domain.StatusActive,
		"gamma": domain.StatusActive,
	})
	before := layoutState(t, beforeState, nil)

	alphaID := domain.AppID("prod", "alpha")
	gammaID := domain.AppID("prod", "gamma")
	if before.Nodes[alphaID].Target.Y >= before.Nodes[gammaID].Target.Y {
		t.Fatalf("test setup expected alpha above gamma before status change: alpha=%v gamma=%v", before.Nodes[alphaID].Target, before.Nodes[gammaID].Target)
	}

	afterState := stateWithApplications(map[string]domain.Status{
		"alpha": domain.StatusActive,
		"beta":  domain.StatusActive,
		"gamma": domain.StatusError,
	})
	after := layoutState(t, afterState, &before)
	if after.Nodes[alphaID].Target.Y >= after.Nodes[gammaID].Target.Y {
		t.Fatalf("status change reordered existing apps: alpha=%v gamma=%v", after.Nodes[alphaID].Target, after.Nodes[gammaID].Target)
	}
}

func TestSchemaLayoutDoesNotOverlapGraphBoxes(t *testing.T) {
	graph := layoutFixture(t, "../../testdata/juju/status/changed.json", nil)
	assertNoBoxOverlap(t, graph)
}

func TestSchemaPlacesTopologyInColumns(t *testing.T) {
	graph := layoutFixture(t, "../../testdata/juju/status/healthy.json", nil)
	provider := graph.Nodes[domain.AppID("prod", "postgresql")]
	consumer := graph.Nodes[domain.AppID("prod", "api-server")]
	machine := graph.Nodes[domain.MachineID("prod", "0")]
	storage := graph.Nodes[domain.StorageID("prod", "pgdata/0")]
	if !(machine.Target.X < provider.Target.X && provider.Target.X < consumer.Target.X) {
		t.Fatalf("topology columns should be machine < provider app < consumer app: machine=%v provider=%v consumer=%v", machine.Target, provider.Target, consumer.Target)
	}
	if storage.Target.X <= provider.Target.X || storage.Target.X >= provider.Target.X+appBoxWidth {
		t.Fatalf("attached storage should be nested inside provider app column: provider=%v storage=%v", provider.Target, storage.Target)
	}
}

func TestSchemaUsesTwoApplicationColumnsForRelations(t *testing.T) {
	graph := layoutFixture(t, "../../testdata/juju/status/healthy.json", nil)
	provider := graph.Nodes[domain.AppID("prod", "postgresql")]
	consumer := graph.Nodes[domain.AppID("prod", "api-server")]
	if provider.Target.X >= consumer.Target.X {
		t.Fatalf("provider should be left of consumer: provider=%v consumer=%v", provider.Target, consumer.Target)
	}
}

func TestSchemaLayoutAccountsForTallApplications(t *testing.T) {
	state := domain.NewState("prod")
	state.Applications["api"] = domain.Application{
		Name:   "api",
		Status: domain.StatusActive,
		Units:  []string{"api/0", "api/1", "api/2", "api/3"},
	}
	state.Applications["worker"] = domain.Application{
		Name:   "worker",
		Status: domain.StatusActive,
		Units:  []string{"worker/0"},
	}
	for _, unit := range state.Applications["api"].Units {
		state.Units[unit] = domain.Unit{Name: unit, AppName: "api", WorkloadStatus: domain.StatusActive}
	}
	state.Units["worker/0"] = domain.Unit{Name: "worker/0", AppName: "worker", WorkloadStatus: domain.StatusActive}
	graph := domain.BuildGraph(state)
	graph, err := Schema{}.Layout(context.Background(), graph, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertNoBoxOverlap(t, graph)
}

func TestSchemaIncreasesApplicationGapForRelationLabel(t *testing.T) {
	sourceID := domain.AppID("prod", "postgresql")
	targetID := domain.AppID("prod", "api")
	edgeID := domain.RelationID("prod", "postgresql:database-provider-to-api", "api:database")
	graph := domain.NewGraph("prod")
	graph.Nodes[sourceID] = domain.Node{
		ID:     sourceID,
		Label:  "postgresql",
		Type:   domain.NodeApplication,
		Status: domain.StatusActive,
	}
	graph.Nodes[targetID] = domain.Node{
		ID:     targetID,
		Label:  "api",
		Type:   domain.NodeApplication,
		Status: domain.StatusActive,
	}
	graph.Edges[edgeID] = domain.Edge{
		ID:       edgeID,
		SourceID: sourceID,
		TargetID: targetID,
		Type:     domain.EdgeRelation,
		Label:    "database-provider-to-api",
		Status:   domain.StatusActive,
	}
	graph.Order = []string{sourceID, targetID}
	graph.EdgeOrder = []string{edgeID}

	graph, err := Schema{}.Layout(context.Background(), graph, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	source := graph.Nodes[sourceID]
	target := graph.Nodes[targetID]
	gap := int(absFloat(target.Target.X-source.Target.X)) - appBoxWidth
	if gap < appRelationLabelWidth+2 {
		t.Fatalf("application gap = %d, want at least %d for relation label", gap, appRelationLabelWidth+2)
	}
}

func TestSchemaSpreadsColumnsAcrossViewport(t *testing.T) {
	narrow := layoutFixtureWithWidth(t, "../../testdata/juju/status/healthy.json", nil, 140)
	wide := layoutFixtureWithWidth(t, "../../testdata/juju/status/healthy.json", nil, 220)

	appID := domain.AppID("prod", "api-server")
	if wide.Nodes[appID].Target.X <= narrow.Nodes[appID].Target.X {
		t.Fatalf("right application column should move right with wider viewport: narrow=%v wide=%v", narrow.Nodes[appID].Target, wide.Nodes[appID].Target)
	}
}

func TestSchemaNestsAttachedStorageInsideApplicationBox(t *testing.T) {
	graph := layoutFixture(t, "../../testdata/juju/status/healthy.json", nil)
	appID := domain.AppID("prod", "postgresql")
	unitID := domain.UnitID("prod", "postgresql/0")
	storageID := domain.StorageID("prod", "pgdata/0")
	app := graph.Nodes[appID]
	unit := graph.Nodes[unitID]
	storage := graph.Nodes[storageID]
	appHeight := appHeight(len(unitsByApplication(graph)[appID]), 1)

	if storage.Target.Y <= unit.Target.Y {
		t.Fatalf("storage should be below attached unit: unit=%v storage=%v", unit.Target, storage.Target)
	}
	if storage.Target.Y >= app.Target.Y+float64(appHeight-1) {
		t.Fatalf("storage should be inside app box: app=%v height=%d storage=%v", app.Target, appHeight, storage.Target)
	}
	for _, box := range graphBoxes(graph) {
		if box.ID == storageID {
			t.Fatalf("attached storage should not create standalone graph box: %+v", box)
		}
	}
}

func TestSchemaLayoutAccountsForTallMachineRows(t *testing.T) {
	state := domain.NewState("prod")
	for i := 0; i < 4; i++ {
		machineID := string(rune('0' + i))
		state.Machines[machineID] = domain.Machine{ID: machineID, Status: domain.StatusActive}
		appName := "app" + machineID
		unitCount := 1
		if i == 0 {
			unitCount = 6
		}
		app := domain.Application{Name: appName, Status: domain.StatusActive}
		for unitIndex := 0; unitIndex < unitCount; unitIndex++ {
			unitName := appName + "/" + string(rune('0'+unitIndex))
			app.Units = append(app.Units, unitName)
			state.Units[unitName] = domain.Unit{
				Name:           unitName,
				AppName:        appName,
				MachineID:      machineID,
				WorkloadStatus: domain.StatusActive,
			}
			machine := state.Machines[machineID]
			machine.Units = append(machine.Units, unitName)
			state.Machines[machineID] = machine
		}
		state.Applications[appName] = app
	}
	graph := domain.BuildGraph(state)
	graph, err := Schema{}.Layout(context.Background(), graph, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertNoBoxOverlap(t, graph)
}

func layoutFixture(t *testing.T, path string, previous *domain.Graph) domain.Graph {
	return layoutFixtureWithWidth(t, path, previous, 160)
}

func layoutState(t *testing.T, state domain.State, previous *domain.Graph) domain.Graph {
	t.Helper()
	graph := domain.BuildGraph(state)
	graph, err := Schema{}.Layout(context.Background(), graph, previous, Options{Width: 160})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func stateWithApplications(statuses map[string]domain.Status) domain.State {
	state := domain.NewState("prod")
	for name, status := range statuses {
		state.Applications[name] = domain.Application{
			Name:   name,
			Status: status,
		}
	}
	return state
}

func layoutFixtureWithWidth(t *testing.T, path string, previous *domain.Graph, width int) domain.Graph {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state, err := juju.ParseStatus("prod", data)
	if err != nil {
		t.Fatal(err)
	}
	graph := domain.BuildGraph(state)
	graph, err = Schema{}.Layout(context.Background(), graph, previous, Options{Width: width})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func assertNoBoxOverlap(t *testing.T, graph domain.Graph) {
	t.Helper()
	boxes := graphBoxes(graph)
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			if boxesOverlap(boxes[i], boxes[j]) {
				t.Fatalf("layout boxes overlap: %+v and %+v", boxes[i], boxes[j])
			}
		}
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
