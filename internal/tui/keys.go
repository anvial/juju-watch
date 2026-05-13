package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit     key.Binding
	Refresh  key.Binding
	Pause    key.Binding
	NextView key.Binding
	Search   key.Binding
	SSH      key.Binding
	Focus    key.Binding
	Help     key.Binding
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PanLeft  key.Binding
	PanDown  key.Binding
	PanUp    key.Binding
	PanRight key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Pause:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause")),
		NextView: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "view")),
		Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		SSH:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "ssh")),
		Focus:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "focus")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "select")),
		Down:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "select")),
		Left:     key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "select")),
		Right:    key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "select")),
		PanLeft:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "pan left")),
		PanDown:  key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "pan down")),
		PanUp:    key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "pan up")),
		PanRight: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "pan right")),
		PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		Home:     key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		End:      key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.Pause, k.NextView, k.Search, k.SSH, k.Focus, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Refresh, k.Pause, k.NextView, k.Search},
		{k.SSH, k.Focus, k.Up, k.Down, k.Left, k.Right},
		{k.PanLeft, k.PanDown, k.PanUp, k.PanRight},
		{k.PageUp, k.PageDown, k.Home, k.End},
		{k.Help, k.Quit},
	}
}
