package animation

import "github.com/anvial/juju-watch/internal/domain"

type Store struct {
	Nodes map[string]*Node
}

func NewStore() Store {
	return Store{Nodes: map[string]*Node{}}
}

func (s *Store) Sync(graph domain.Graph, changed map[string]bool) {
	if s.Nodes == nil {
		s.Nodes = map[string]*Node{}
	}
	for id, node := range graph.Nodes {
		animated, ok := s.Nodes[id]
		if !ok {
			animated = NewNode(id, node.Current)
			s.Nodes[id] = animated
		}
		if changed[id] {
			animated.MarkChanged()
		}
	}
	for id := range s.Nodes {
		if _, ok := graph.Nodes[id]; !ok {
			delete(s.Nodes, id)
		}
	}
}

func (s *Store) Step(graph *domain.Graph, noAnimation bool) bool {
	active := false
	for id, node := range graph.Nodes {
		animated, ok := s.Nodes[id]
		if !ok {
			animated = NewNode(id, node.Current)
			s.Nodes[id] = animated
		}
		node.Current = animated.Step(node.Target, noAnimation)
		graph.Nodes[id] = node
		if !animated.Settled(node.Target) {
			active = true
		}
	}
	return active
}

func (s Store) Pulse(id string) float64 {
	if node, ok := s.Nodes[id]; ok {
		return node.Pulse
	}
	return 0
}
