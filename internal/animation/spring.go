package animation

import (
	"math"

	"github.com/anvial/juju-watch/internal/domain"
	"github.com/charmbracelet/harmonica"
)

type Node struct {
	ID string
	X  float64
	Y  float64
	VX float64
	VY float64

	Pulse float64

	spring harmonica.Spring
}

func NewNode(id string, pos domain.Position) *Node {
	return &Node{
		ID:     id,
		X:      pos.X,
		Y:      pos.Y,
		Pulse:  0,
		spring: harmonica.NewSpring(harmonica.FPS(30), 8.0, 0.72),
	}
}

func (n *Node) MarkChanged() {
	n.Pulse = 1
}

func (n *Node) Step(target domain.Position, noAnimation bool) domain.Position {
	if noAnimation {
		n.X = target.X
		n.Y = target.Y
		n.VX = 0
		n.VY = 0
		n.Pulse = 0
		return target
	}
	n.X, n.VX = n.spring.Update(n.X, n.VX, target.X)
	n.Y, n.VY = n.spring.Update(n.Y, n.VY, target.Y)
	n.Pulse = math.Max(0, n.Pulse-0.04)
	return domain.Position{X: n.X, Y: n.Y}
}

func (n *Node) Settled(target domain.Position) bool {
	return math.Abs(n.X-target.X) < 0.05 && math.Abs(n.Y-target.Y) < 0.05 && math.Abs(n.VX) < 0.05 && math.Abs(n.VY) < 0.05 && n.Pulse == 0
}
