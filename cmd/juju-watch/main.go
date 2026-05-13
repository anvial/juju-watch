package main

import (
	"fmt"
	"os"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/juju"
	"github.com/anvial/juju-watch/internal/layout"
	"github.com/anvial/juju-watch/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.AllModels {
		fmt.Fprintln(os.Stderr, "--all-models is not implemented in the MVP")
		os.Exit(1)
	}
	if cfg.Debug {
		logFile := cfg.LogFile
		if logFile == "" {
			logFile = "juju-watch.log"
		}
		file, err := tea.LogToFile(logFile, "juju-watch")
		if err != nil {
			fmt.Fprintln(os.Stderr, "debug log:", err)
		} else {
			defer file.Close()
		}
	}

	poller := juju.NewPoller(juju.NewExecRunner(), juju.PollConfig{
		Model:     cfg.Model,
		Interval:  cfg.Interval,
		Timeout:   cfg.Interval,
		Relations: cfg.Relations,
		Storage:   cfg.Storage,
	})
	model := tui.New(cfg, poller, layout.New(cfg.Layout))
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
