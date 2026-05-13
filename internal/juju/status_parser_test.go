package juju

import (
	"os"
	"testing"

	"github.com/anvial/juju-watch/internal/domain"
)

func TestParseStatus(t *testing.T) {
	data, err := os.ReadFile("../../testdata/juju/status/healthy.json")
	if err != nil {
		t.Fatal(err)
	}
	state, err := ParseStatus("prod", data)
	if err != nil {
		t.Fatal(err)
	}
	if state.Model != "prod" {
		t.Fatalf("model = %q", state.Model)
	}
	if len(state.Applications) != 2 {
		t.Fatalf("applications = %d", len(state.Applications))
	}
	if state.Applications["postgresql"].Status != domain.StatusActive {
		t.Fatalf("postgresql status = %q", state.Applications["postgresql"].Status)
	}
	if state.Units["postgresql/0"].MachineID != "0" {
		t.Fatalf("unit machine = %q", state.Units["postgresql/0"].MachineID)
	}
	if len(state.Relations) != 1 {
		t.Fatalf("relations = %d", len(state.Relations))
	}
	if state.Storage["pgdata/0"].Unit != "postgresql/0" {
		t.Fatalf("storage unit = %q", state.Storage["pgdata/0"].Unit)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	if _, err := ParseStatus("prod", []byte("{")); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestParseStatusWithJuju4PreambleAndNestedStorage(t *testing.T) {
	data := []byte(`provided relations, storage options are always enabled in non tabular formats
{
  "model": {"name": "test-model", "model-status": {"current": "available"}},
  "machines": {
    "0": {
      "ip-addresses": ["10.0.0.10"],
      "machine-status": {"current": "running"}
    },
    "1": {
      "ip-addresses": ["10.0.0.11"],
      "machine-status": {"current": "running"}
    }
  },
  "applications": {
    "postgresql": {
      "charm": "postgresql",
      "application-status": {"current": "active"},
      "relations": {
        "database": [
          {"related-application": "postgresql-test-app", "interface": "postgresql_client", "scope": "global"}
        ],
        "database-peers": [
          {"related-application": "postgresql", "interface": "postgresql_peers", "scope": "global"}
        ]
      },
      "units": {
        "postgresql/0": {
          "machine": "0",
          "public-address": "10.0.0.10",
          "open-ports": ["5432/tcp"],
          "workload-status": {"current": "active", "message": "Primary"},
          "juju-status": {"current": "idle"}
        }
      }
    },
    "postgresql-test-app": {
      "charm": "postgresql-test-app",
      "application-status": {"current": "active"},
      "relations": {
        "database": [
          {"related-application": "postgresql", "interface": "postgresql_client", "scope": "global"}
        ]
      },
      "units": {
        "postgresql-test-app/0": {
          "machine": "1",
          "public-address": "10.0.0.11",
          "workload-status": {"current": "active"},
          "juju-status": {"current": "idle"}
        }
      }
    }
  },
  "storage": {
    "storage": {
      "data/0": {
        "kind": "filesystem",
        "status": {"current": "attached"},
        "attachments": {
          "units": {
            "postgresql/0": {"machine": "0", "location": "/var/lib/postgresql"}
          }
        }
      }
    }
  }
}`)
	state, err := ParseStatus("test-model", data)
	if err != nil {
		t.Fatal(err)
	}
	if state.Model != "test-model" {
		t.Fatalf("model = %q", state.Model)
	}
	if len(state.Relations) != 1 {
		t.Fatalf("relations = %d", len(state.Relations))
	}
	if state.Relations[0].AppA == state.Relations[0].AppB {
		t.Fatalf("self relation should be skipped: %+v", state.Relations[0])
	}
	if state.Units["postgresql/0"].Ports[0] != "5432/tcp" {
		t.Fatalf("ports = %v", state.Units["postgresql/0"].Ports)
	}
	if state.Storage["data/0"].Unit != "postgresql/0" {
		t.Fatalf("storage = %+v", state.Storage["data/0"])
	}
}
