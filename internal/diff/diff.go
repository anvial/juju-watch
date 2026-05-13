package diff

import (
	"reflect"
	"sort"

	"github.com/anvial/juju-watch/internal/domain"
)

type ChangeKind string

const (
	Added     ChangeKind = "added"
	Removed   ChangeKind = "removed"
	Updated   ChangeKind = "updated"
	Unchanged ChangeKind = "unchanged"
)

type NodeChange struct {
	ID     string
	Kind   ChangeKind
	Before domain.Node
	After  domain.Node
}

type EdgeChange struct {
	ID     string
	Kind   ChangeKind
	Before domain.Edge
	After  domain.Edge
}

type Result struct {
	Nodes           map[string]NodeChange
	Edges           map[string]EdgeChange
	TopologyChanged bool
}

func Graphs(before, after domain.Graph) Result {
	result := Result{
		Nodes:           map[string]NodeChange{},
		Edges:           map[string]EdgeChange{},
		TopologyChanged: before.TopologyHash != after.TopologyHash,
	}

	for _, id := range unionNodeIDs(before, after) {
		oldNode, hadOld := before.Nodes[id]
		newNode, hasNew := after.Nodes[id]
		switch {
		case !hadOld && hasNew:
			result.Nodes[id] = NodeChange{ID: id, Kind: Added, After: newNode}
		case hadOld && !hasNew:
			result.Nodes[id] = NodeChange{ID: id, Kind: Removed, Before: oldNode}
		case !nodeEqual(oldNode, newNode):
			result.Nodes[id] = NodeChange{ID: id, Kind: Updated, Before: oldNode, After: newNode}
		default:
			result.Nodes[id] = NodeChange{ID: id, Kind: Unchanged, Before: oldNode, After: newNode}
		}
	}

	for _, id := range unionEdgeIDs(before, after) {
		oldEdge, hadOld := before.Edges[id]
		newEdge, hasNew := after.Edges[id]
		switch {
		case !hadOld && hasNew:
			result.Edges[id] = EdgeChange{ID: id, Kind: Added, After: newEdge}
		case hadOld && !hasNew:
			result.Edges[id] = EdgeChange{ID: id, Kind: Removed, Before: oldEdge}
		case !edgeEqual(oldEdge, newEdge):
			result.Edges[id] = EdgeChange{ID: id, Kind: Updated, Before: oldEdge, After: newEdge}
		default:
			result.Edges[id] = EdgeChange{ID: id, Kind: Unchanged, Before: oldEdge, After: newEdge}
		}
	}

	return result
}

func (r Result) ChangedNodeIDs() []string {
	ids := []string{}
	for id, change := range r.Nodes {
		if change.Kind == Added || change.Kind == Removed || change.Kind == Updated {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (r Result) ChangedEdgeIDs() []string {
	ids := []string{}
	for id, change := range r.Edges {
		if change.Kind == Added || change.Kind == Removed || change.Kind == Updated {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func EventsFromResult(result Result, at func() domain.Event) []domain.Event {
	base := at()
	events := []domain.Event{}
	for _, id := range result.ChangedNodeIDs() {
		change := result.Nodes[id]
		event := base
		event.ObjectID = id
		event.Kind = string(change.Kind)
		switch change.Kind {
		case Added:
			event.Label = change.After.Label
			event.To = string(change.After.Status)
			event.Message = change.After.StatusMessage
		case Removed:
			event.Label = change.Before.Label
			event.From = string(change.Before.Status)
			event.Message = "removed"
		case Updated:
			event.Label = change.After.Label
			event.From = string(change.Before.Status)
			event.To = string(change.After.Status)
			if change.Before.Status == change.After.Status && change.Before.StatusMessage != change.After.StatusMessage {
				event.From = change.Before.StatusMessage
				event.To = change.After.StatusMessage
			}
			event.Message = change.After.StatusMessage
		}
		events = append(events, event)
	}
	return events
}

func unionNodeIDs(before, after domain.Graph) []string {
	seen := map[string]struct{}{}
	for id := range before.Nodes {
		seen[id] = struct{}{}
	}
	for id := range after.Nodes {
		seen[id] = struct{}{}
	}
	return sortedSeen(seen)
}

func unionEdgeIDs(before, after domain.Graph) []string {
	seen := map[string]struct{}{}
	for id := range before.Edges {
		seen[id] = struct{}{}
	}
	for id := range after.Edges {
		seen[id] = struct{}{}
	}
	return sortedSeen(seen)
}

func sortedSeen(seen map[string]struct{}) []string {
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func nodeEqual(a, b domain.Node) bool {
	return a.ID == b.ID &&
		a.Label == b.Label &&
		a.Type == b.Type &&
		a.Status == b.Status &&
		a.StatusMessage == b.StatusMessage &&
		reflect.DeepEqual(a.Metadata, b.Metadata)
}

func edgeEqual(a, b domain.Edge) bool {
	return a.ID == b.ID &&
		a.SourceID == b.SourceID &&
		a.TargetID == b.TargetID &&
		a.Type == b.Type &&
		a.Label == b.Label &&
		a.Status == b.Status &&
		reflect.DeepEqual(a.Metadata, b.Metadata)
}
