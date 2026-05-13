package tui

import (
	"time"

	"github.com/anvial/juju-watch/internal/animation"
	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/diff"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/juju"
	"github.com/anvial/juju-watch/internal/layout"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type ViewMode string

const (
	ViewTopology ViewMode = "topology"
	ViewMachines ViewMode = "machines"
	ViewProblems ViewMode = "problems"
	ViewEvents   ViewMode = "events"
)

type PollResultMsg struct {
	State domain.State
	Err   error
	At    time.Time
}

type pollTickMsg time.Time
type frameMsg time.Time

type Model struct {
	cfg    cli.Config
	poller *juju.Poller
	layout layout.Engine

	keys   KeyMap
	styles Styles
	help   help.Model
	search textinput.Model
	spin   spinner.Model

	width  int
	height int
	view   ViewMode

	graph      domain.Graph
	hasGraph   bool
	changes    diff.Result
	animations animation.Store
	changedIDs map[string]bool
	events     []domain.Event
	selectedID string
	panX       int
	panY       int
	paused     bool
	polling    bool
	searching  bool
	showHelp   bool
	lastPollAt time.Time
	lastErr    error
	layoutErr  error
}

func New(cfg cli.Config, poller *juju.Poller, engine layout.Engine) Model {
	search := textinput.New()
	search.Placeholder = "search app, unit, machine, relation, status"
	search.Prompt = "/ "
	search.CharLimit = 80

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return Model{
		cfg:        cfg,
		poller:     poller,
		layout:     engine,
		keys:       DefaultKeyMap(),
		styles:     NewStyles(),
		help:       help.New(),
		search:     search,
		spin:       spin,
		view:       ViewMode(cfg.View),
		animations: animation.NewStore(),
		changedIDs: map[string]bool{},
		events:     []domain.Event{},
		polling:    true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.pollCmd(), tickCmd(m.cfg.Interval), frameCmd(), m.spin.Tick)
}
