package juju

import (
	"context"
	"time"

	"github.com/anvial/juju-watch/internal/domain"
)

type PollConfig struct {
	Model     string
	Interval  time.Duration
	Timeout   time.Duration
	Relations bool
	Storage   bool
}

type Poller struct {
	Runner Runner
	Config PollConfig
}

func NewPoller(runner Runner, cfg PollConfig) *Poller {
	if cfg.Timeout <= 0 {
		cfg.Timeout = cfg.Interval
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Poller{Runner: runner, Config: cfg}
}

func (p *Poller) Poll(ctx context.Context) (domain.State, error) {
	timeout := p.Config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := StatusArgs(StatusRequest{
		Model:     p.Config.Model,
		Relations: p.Config.Relations,
		Storage:   p.Config.Storage,
	})
	output, err := p.Runner.Run(ctx, "juju", args...)
	if err != nil {
		return domain.State{}, err
	}
	return ParseStatus(p.Config.Model, output)
}
