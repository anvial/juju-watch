package domain

import (
	"sort"
	"strings"
	"time"
)

type Status string

const (
	StatusActive      Status = "active"
	StatusWaiting     Status = "waiting"
	StatusBlocked     Status = "blocked"
	StatusError       Status = "error"
	StatusMaintenance Status = "maintenance"
	StatusUnknown     Status = "unknown"
	StatusRunning     Status = "running"
	StatusStarted     Status = "started"
	StatusIdle        Status = "idle"
)

func NormalizeStatus(value string) Status {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "available", "running", "started", "idle", "executing", "allocating", "attached", "ready":
		return StatusActive
	case "waiting", "pending":
		return StatusWaiting
	case "blocked":
		return StatusBlocked
	case "error", "failed":
		return StatusError
	case "maintenance":
		return StatusMaintenance
	case "":
		return StatusUnknown
	default:
		return Status(strings.ToLower(strings.TrimSpace(value)))
	}
}

func (s Status) Interesting() bool {
	switch s {
	case StatusError, StatusBlocked, StatusWaiting, StatusUnknown, StatusMaintenance:
		return true
	default:
		return false
	}
}

func (s Status) Severity() int {
	switch s {
	case StatusError:
		return 5
	case StatusBlocked:
		return 4
	case StatusWaiting:
		return 3
	case StatusMaintenance:
		return 2
	case StatusUnknown:
		return 1
	default:
		return 0
	}
}

func WorstStatus(statuses ...Status) Status {
	worst := StatusActive
	for _, status := range statuses {
		if status.Severity() > worst.Severity() {
			worst = status
		}
	}
	return worst
}

type State struct {
	Model        string
	Controller   string
	Cloud        string
	Region       string
	Version      string
	Status       Status
	StatusMsg    string
	Applications map[string]Application
	Units        map[string]Unit
	Machines     map[string]Machine
	Relations    []Relation
	Storage      map[string]Storage
	ObservedAt   time.Time
}

type Application struct {
	Name          string
	Charm         string
	CharmChannel  string
	CharmVersion  string
	Status        Status
	StatusMessage string
	Units         []string
	Relations     []string
	Metadata      map[string]string
}

type Unit struct {
	Name            string
	AppName         string
	MachineID       string
	PublicAddress   string
	WorkloadStatus  Status
	WorkloadMessage string
	AgentStatus     Status
	AgentMessage    string
	Ports           []string
	Storage         []string
	Leader          bool
	Metadata        map[string]string
}

type Machine struct {
	ID            string
	InstanceID    string
	DNSName       string
	IPAddress     string
	Status        Status
	StatusMessage string
	Units         []string
	Metadata      map[string]string
}

type Relation struct {
	EndpointA string
	EndpointB string
	AppA      string
	AppB      string
	Interface string
	RoleA     string
	RoleB     string
}

func (r Relation) Label() string {
	sourceApp, targetApp := r.DirectedApps()
	sourceEndpoint := r.endpointLabelForApp(sourceApp)
	targetEndpoint := r.endpointLabelForApp(targetApp)
	switch {
	case sourceEndpoint != "" && targetEndpoint != "" && sourceEndpoint != targetEndpoint:
		return sourceEndpoint + " → " + targetEndpoint
	case sourceEndpoint != "":
		return sourceEndpoint
	case targetEndpoint != "":
		return targetEndpoint
	case r.Interface != "":
		return r.Interface
	default:
		return "relation"
	}
}

func (r Relation) DirectedApps() (string, string) {
	roleA := normalizeRelationRole(r.RoleA)
	roleB := normalizeRelationRole(r.RoleB)
	switch {
	case isProviderRole(roleA) && isConsumerRole(roleB):
		return r.AppA, r.AppB
	case isProviderRole(roleB) && isConsumerRole(roleA):
		return r.AppB, r.AppA
	case isProviderRole(roleA) && !isProviderRole(roleB):
		return r.AppA, r.AppB
	case isProviderRole(roleB) && !isProviderRole(roleA):
		return r.AppB, r.AppA
	case isConsumerRole(roleA) && !isConsumerRole(roleB):
		return r.AppB, r.AppA
	case isConsumerRole(roleB) && !isConsumerRole(roleA):
		return r.AppA, r.AppB
	default:
		return r.AppA, r.AppB
	}
}

func (r Relation) EndpointForApp(app string) string {
	switch app {
	case r.AppA:
		return r.EndpointA
	case r.AppB:
		return r.EndpointB
	default:
		return ""
	}
}

func (r Relation) RoleForApp(app string) string {
	switch app {
	case r.AppA:
		return r.RoleA
	case r.AppB:
		return r.RoleB
	default:
		return ""
	}
}

func (r Relation) endpointLabelForApp(app string) string {
	endpoint := r.EndpointForApp(app)
	if endpoint == "" {
		return ""
	}
	label := strings.TrimPrefix(endpoint, app+":")
	if label == "" || label == app {
		return ""
	}
	return label
}

func normalizeRelationRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func isProviderRole(role string) bool {
	switch role {
	case "provider", "provides":
		return true
	default:
		return false
	}
}

func isConsumerRole(role string) bool {
	switch role {
	case "consumer", "requirer", "requires":
		return true
	default:
		return false
	}
}

type Storage struct {
	ID            string
	Kind          string
	Status        Status
	StatusMessage string
	Unit          string
	MachineID     string
	Location      string
	Metadata      map[string]string
}

type Event struct {
	At       time.Time
	ObjectID string
	Label    string
	Kind     string
	From     string
	To       string
	Message  string
}

func NewState(model string) State {
	return State{
		Model:        model,
		Status:       StatusUnknown,
		Applications: map[string]Application{},
		Units:        map[string]Unit{},
		Machines:     map[string]Machine{},
		Storage:      map[string]Storage{},
		ObservedAt:   time.Now(),
	}
}

func SortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
