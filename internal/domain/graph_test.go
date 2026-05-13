package domain

import "testing"

func TestBuildGraph(t *testing.T) {
	state := NewState("prod")
	state.Applications["postgresql"] = Application{Name: "postgresql", Status: StatusActive, Units: []string{"postgresql/0"}}
	state.Units["postgresql/0"] = Unit{Name: "postgresql/0", AppName: "postgresql", MachineID: "0", WorkloadStatus: StatusActive}
	state.Machines["0"] = Machine{ID: "0", Status: StatusActive, Units: []string{"postgresql/0"}}
	state.Storage["pgdata/0"] = Storage{ID: "pgdata/0", Status: StatusActive, Unit: "postgresql/0"}

	graph := BuildGraph(state)
	if _, ok := graph.Nodes[AppID("prod", "postgresql")]; !ok {
		t.Fatal("missing application node")
	}
	if _, ok := graph.Nodes[UnitID("prod", "postgresql/0")]; !ok {
		t.Fatal("missing unit node")
	}
	machine, ok := graph.Nodes[MachineID("prod", "0")]
	if !ok {
		t.Fatal("missing machine node")
	}
	if machine.Metadata["machine_id"] != "0" {
		t.Fatalf("machine_id metadata = %q, want 0", machine.Metadata["machine_id"])
	}
	if _, ok := graph.Nodes[StorageID("prod", "pgdata/0")]; !ok {
		t.Fatal("missing storage node")
	}
	if graph.TopologyHash == "" {
		t.Fatal("missing topology hash")
	}
}

func TestBuildGraphDirectsRelationFromProviderToConsumer(t *testing.T) {
	state := NewState("prod")
	state.Applications["api"] = Application{Name: "api", Status: StatusActive}
	state.Applications["postgresql"] = Application{Name: "postgresql", Status: StatusActive}
	state.Relations = []Relation{{
		AppA:      "api",
		EndpointA: "api:database",
		RoleA:     "requirer",
		AppB:      "postgresql",
		EndpointB: "postgresql:db",
		RoleB:     "provider",
		Interface: "postgresql_client",
	}}

	graph := BuildGraph(state)
	edge := graph.Edges[RelationID("prod", "api:database", "postgresql:db")]
	if edge.SourceID != AppID("prod", "postgresql") {
		t.Fatalf("source = %q, want provider", edge.SourceID)
	}
	if edge.TargetID != AppID("prod", "api") {
		t.Fatalf("target = %q, want consumer", edge.TargetID)
	}
	if edge.Label != "db → database" {
		t.Fatalf("label = %q, want directed endpoint label", edge.Label)
	}
	if edge.Metadata["source_role"] != "provider" || edge.Metadata["target_role"] != "requirer" {
		t.Fatalf("metadata roles = source:%q target:%q", edge.Metadata["source_role"], edge.Metadata["target_role"])
	}
}
