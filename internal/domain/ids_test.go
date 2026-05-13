package domain

import "testing"

func TestStableIDs(t *testing.T) {
	if got := ModelID("prod"); got != "model:prod" {
		t.Fatalf("ModelID = %q", got)
	}
	if got := AppID("prod", "postgresql"); got != "app:prod:postgresql" {
		t.Fatalf("AppID = %q", got)
	}
	if got := UnitID("prod", "postgresql/0"); got != "unit:prod:postgresql/0" {
		t.Fatalf("UnitID = %q", got)
	}
	if got := MachineID("prod", "0"); got != "machine:prod:0" {
		t.Fatalf("MachineID = %q", got)
	}
	if got := StorageID("prod", "pgdata/0"); got != "storage:prod:pgdata/0" {
		t.Fatalf("StorageID = %q", got)
	}
	a := RelationID("prod", "postgresql:db", "api-server:database")
	b := RelationID("prod", "api-server:database", "postgresql:db")
	if a != b {
		t.Fatalf("relation IDs should be order independent: %q != %q", a, b)
	}
}
