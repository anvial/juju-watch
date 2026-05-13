package layout

import (
	"context"
	"errors"

	"github.com/anvial/juju-watch/internal/domain"
)

type Force struct{}

func (Force) Layout(_ context.Context, graph domain.Graph, _ *domain.Graph, _ Options) (domain.Graph, error) {
	return graph, errors.New("force layout is not implemented in MVP")
}
