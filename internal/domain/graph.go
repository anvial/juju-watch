package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

type NodeType string

const (
	NodeModel       NodeType = "model"
	NodeApplication NodeType = "application"
	NodeUnit        NodeType = "unit"
	NodeMachine     NodeType = "machine"
	NodeStorage     NodeType = "storage"
	NodeOffer       NodeType = "offer"
	NodeConsumer    NodeType = "consumer"
	NodeSpace       NodeType = "space"
)

type EdgeType string

const (
	EdgeRelation        EdgeType = "relation"
	EdgeAppHasUnit      EdgeType = "app-has-unit"
	EdgeUnitOnMachine   EdgeType = "unit-on-machine"
	EdgeStorageAttached EdgeType = "storage-attached"
	EdgeOfferConsumer   EdgeType = "offer-consumer"
	EdgeNetworkSpace    EdgeType = "network-space"
)

type Position struct {
	X float64
	Y float64
}

type Node struct {
	ID            string
	Label         string
	Type          NodeType
	Status        Status
	StatusMessage string
	Metadata      map[string]string
	Current       Position
	Target        Position
}

type Edge struct {
	ID       string
	SourceID string
	TargetID string
	Type     EdgeType
	Label    string
	Status   Status
	Metadata map[string]string
}

type Graph struct {
	Model        string
	Nodes        map[string]Node
	Edges        map[string]Edge
	Order        []string
	EdgeOrder    []string
	TopologyHash string
}

func NewGraph(model string) Graph {
	return Graph{
		Model:     model,
		Nodes:     map[string]Node{},
		Edges:     map[string]Edge{},
		Order:     []string{},
		EdgeOrder: []string{},
	}
}

func BuildGraph(state State) Graph {
	graph := NewGraph(state.Model)
	modelID := ModelID(state.Model)
	graph.addNode(Node{
		ID:            modelID,
		Label:         state.Model,
		Type:          NodeModel,
		Status:        state.Status,
		StatusMessage: state.StatusMsg,
		Metadata: map[string]string{
			"controller": state.Controller,
			"cloud":      state.Cloud,
			"region":     state.Region,
			"version":    state.Version,
		},
	})

	for _, name := range SortedKeys(state.Applications) {
		app := state.Applications[name]
		status := app.Status
		for _, unitName := range app.Units {
			unit := state.Units[unitName]
			status = WorstStatus(status, unit.WorkloadStatus)
		}
		appID := AppID(state.Model, app.Name)
		graph.addNode(Node{
			ID:            appID,
			Label:         app.Name,
			Type:          NodeApplication,
			Status:        status,
			StatusMessage: app.StatusMessage,
			Metadata: map[string]string{
				"charm":         app.Charm,
				"charm_channel": app.CharmChannel,
				"charm_version": app.CharmVersion,
				"units":         strconv.Itoa(len(app.Units)),
			},
		})
		for _, unitName := range app.Units {
			unit := state.Units[unitName]
			unitID := UnitID(state.Model, unit.Name)
			graph.addNode(Node{
				ID:            unitID,
				Label:         unit.Name,
				Type:          NodeUnit,
				Status:        unit.WorkloadStatus,
				StatusMessage: unit.WorkloadMessage,
				Metadata: map[string]string{
					"application":    unit.AppName,
					"machine":        unit.MachineID,
					"public_address": unit.PublicAddress,
					"agent_status":   string(unit.AgentStatus),
					"agent_message":  unit.AgentMessage,
					"leader":         strconv.FormatBool(unit.Leader),
					"ports":          strings.Join(unit.Ports, ", "),
				},
			})
			graph.addEdge(Edge{
				ID:       EdgeID(EdgeAppHasUnit, appID, unitID),
				SourceID: appID,
				TargetID: unitID,
				Type:     EdgeAppHasUnit,
				Label:    "unit",
				Status:   unit.WorkloadStatus,
			})
		}
	}

	for _, machineID := range SortedKeys(state.Machines) {
		machine := state.Machines[machineID]
		id := MachineID(state.Model, machine.ID)
		graph.addNode(Node{
			ID:            id,
			Label:         "machine " + machine.ID,
			Type:          NodeMachine,
			Status:        machine.Status,
			StatusMessage: machine.StatusMessage,
			Metadata: map[string]string{
				"machine_id":  machine.ID,
				"instance_id": machine.InstanceID,
				"dns_name":    machine.DNSName,
				"ip_address":  machine.IPAddress,
				"units":       strconv.Itoa(len(machine.Units)),
			},
		})
		for _, unitName := range machine.Units {
			unitID := UnitID(state.Model, unitName)
			if _, ok := graph.Nodes[unitID]; !ok {
				continue
			}
			graph.addEdge(Edge{
				ID:       EdgeID(EdgeUnitOnMachine, unitID, id),
				SourceID: unitID,
				TargetID: id,
				Type:     EdgeUnitOnMachine,
				Label:    "on",
				Status:   machine.Status,
			})
		}
	}

	for _, storageID := range SortedKeys(state.Storage) {
		storage := state.Storage[storageID]
		id := StorageID(state.Model, storage.ID)
		graph.addNode(Node{
			ID:            id,
			Label:         storage.ID,
			Type:          NodeStorage,
			Status:        storage.Status,
			StatusMessage: storage.StatusMessage,
			Metadata: map[string]string{
				"kind":     storage.Kind,
				"unit":     storage.Unit,
				"machine":  storage.MachineID,
				"location": storage.Location,
			},
		})
		if storage.Unit != "" {
			unitID := UnitID(state.Model, storage.Unit)
			if _, ok := graph.Nodes[unitID]; ok {
				graph.addEdge(Edge{
					ID:       EdgeID(EdgeStorageAttached, unitID, id),
					SourceID: unitID,
					TargetID: id,
					Type:     EdgeStorageAttached,
					Label:    "storage",
					Status:   storage.Status,
				})
			}
		}
	}

	for _, rel := range state.Relations {
		sourceApp, targetApp := rel.DirectedApps()
		sourceID := AppID(state.Model, sourceApp)
		targetID := AppID(state.Model, targetApp)
		if _, ok := graph.Nodes[sourceID]; !ok {
			continue
		}
		if _, ok := graph.Nodes[targetID]; !ok {
			continue
		}
		graph.addEdge(Edge{
			ID:       RelationID(state.Model, rel.EndpointA, rel.EndpointB),
			SourceID: sourceID,
			TargetID: targetID,
			Type:     EdgeRelation,
			Label:    rel.Label(),
			Status:   StatusActive,
			Metadata: map[string]string{
				"endpoint_a":      rel.EndpointA,
				"endpoint_b":      rel.EndpointB,
				"role_a":          rel.RoleA,
				"role_b":          rel.RoleB,
				"interface":       rel.Interface,
				"source_endpoint": rel.EndpointForApp(sourceApp),
				"target_endpoint": rel.EndpointForApp(targetApp),
				"source_role":     rel.RoleForApp(sourceApp),
				"target_role":     rel.RoleForApp(targetApp),
			},
		})
	}

	graph.TopologyHash = graphTopologyHash(graph)
	return graph
}

func (g *Graph) addNode(node Node) {
	node.Metadata = cloneMap(node.Metadata)
	g.Nodes[node.ID] = node
	g.Order = append(g.Order, node.ID)
}

func (g *Graph) addEdge(edge Edge) {
	edge.Metadata = cloneMap(edge.Metadata)
	g.Edges[edge.ID] = edge
	g.EdgeOrder = append(g.EdgeOrder, edge.ID)
}

func (g Graph) Neighbors(id string) []string {
	neighbors := []string{}
	for _, edge := range g.Edges {
		if edge.SourceID == id {
			neighbors = append(neighbors, edge.TargetID)
		}
		if edge.TargetID == id {
			neighbors = append(neighbors, edge.SourceID)
		}
	}
	sort.Strings(neighbors)
	return neighbors
}

func (g Graph) NodeLabel(id string) string {
	if node, ok := g.Nodes[id]; ok {
		return node.Label
	}
	if edge, ok := g.Edges[id]; ok {
		return edge.Label
	}
	return id
}

func cloneMap(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		if value != "" {
			out[key] = value
		}
	}
	return out
}

func graphTopologyHash(graph Graph) string {
	parts := make([]string, 0, len(graph.Nodes)+len(graph.Edges))
	for _, node := range graph.Nodes {
		parts = append(parts, "n:"+node.ID+":"+string(node.Type))
	}
	for _, edge := range graph.Edges {
		parts = append(parts, "e:"+edge.ID+":"+edge.SourceID+":"+edge.TargetID+":"+string(edge.Type))
	}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}
