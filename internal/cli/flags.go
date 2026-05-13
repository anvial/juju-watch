package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

type Config struct {
	Model       string
	Interval    time.Duration
	View        string
	Layout      string
	Focus       string
	Relations   bool
	Storage     bool
	AllModels   bool
	NoAnimation bool
	Debug       bool
	LogFile     string
}

func DefaultConfig() Config {
	return Config{
		Interval:  5 * time.Second,
		View:      "topology",
		Layout:    "schema",
		Relations: true,
		Storage:   true,
	}
}

func Parse(args []string) (Config, error) {
	cfg := DefaultConfig()
	fs := flag.NewFlagSet("juju-watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var shortModel string
	fs.StringVar(&shortModel, "m", "", "Juju model")
	fs.StringVar(&cfg.Model, "model", "", "Juju model")
	fs.DurationVar(&cfg.Interval, "interval", cfg.Interval, "poll interval")
	fs.StringVar(&cfg.View, "view", cfg.View, "initial view: topology, machines, problems, events")
	fs.StringVar(&cfg.Layout, "layout", cfg.Layout, "layout mode: schema, graphviz, force")
	fs.StringVar(&cfg.Focus, "focus", "", "initial app or unit focus")
	fs.BoolVar(&cfg.Relations, "relations", cfg.Relations, "include relations in juju status")
	fs.BoolVar(&cfg.Storage, "storage", cfg.Storage, "include storage in juju status")
	fs.BoolVar(&cfg.AllModels, "all-models", false, "show all models")
	fs.BoolVar(&cfg.NoAnimation, "no-animation", false, "disable animations")
	fs.BoolVar(&cfg.Debug, "debug", false, "enable debug logging")
	fs.StringVar(&cfg.LogFile, "log-file", "", "debug log file")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if cfg.Model == "" {
		cfg.Model = shortModel
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Interval <= 0 {
		return errors.New("interval must be greater than zero")
	}
	if !c.AllModels && c.Model == "" {
		return errors.New("model is required; pass -m <model>")
	}
	switch c.View {
	case "topology", "machines", "problems", "events":
	default:
		return fmt.Errorf("invalid view %q", c.View)
	}
	switch c.Layout {
	case "schema", "graphviz", "force":
	default:
		return fmt.Errorf("invalid layout %q", c.Layout)
	}
	return nil
}
