package layout

import (
	"context"

	"github.com/anvial/juju-watch/internal/domain"
)

type Engine interface {
	Layout(ctx context.Context, graph domain.Graph, previous *domain.Graph, opts Options) (domain.Graph, error)
}

type Options struct {
	Width  int
	Height int
}

type Mode string

const (
	ModeSchema   Mode = "schema"
	ModeGraphviz Mode = "graphviz"
	ModeForce    Mode = "force"
)

func New(mode string) Engine {
	switch Mode(mode) {
	case ModeGraphviz:
		return Graphviz{}
	case ModeForce:
		return Force{}
	default:
		return Schema{}
	}
}
