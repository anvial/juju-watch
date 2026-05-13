package layout

import (
	"context"
	"os/exec"

	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/juju"
)

type Graphviz struct{}

func (Graphviz) Layout(_ context.Context, graph domain.Graph, _ *domain.Graph, _ Options) (domain.Graph, error) {
	if _, err := exec.LookPath("dot"); err != nil {
		return graph, juju.CommandError{Kind: juju.ErrGraphviz, Err: err}
	}
	return graph, nil
}
