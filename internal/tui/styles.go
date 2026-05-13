package tui

import (
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	Header            lipgloss.Style
	Footer            lipgloss.Style
	Panel             lipgloss.Style
	Title             lipgloss.Style
	Dim               lipgloss.Style
	Selected          lipgloss.Style
	Changed           lipgloss.Style
	Active            lipgloss.Style
	Waiting           lipgloss.Style
	Blocked           lipgloss.Style
	Error             lipgloss.Style
	Maintenance       lipgloss.Style
	Unknown           lipgloss.Style
	Edge              lipgloss.Style
	Relation          lipgloss.Style
	RelationSelected  lipgloss.Style
	Placement         lipgloss.Style
	PlacementSelected lipgloss.Style
}

func NewStyles() Styles {
	text := lipgloss.AdaptiveColor{Light: "0", Dark: "15"}
	muted := lipgloss.AdaptiveColor{Light: "240", Dark: "250"}
	panel := lipgloss.AdaptiveColor{Light: "244", Dark: "245"}

	return Styles{
		Header:            lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("0")).Padding(0, 1),
		Footer:            lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		Panel:             lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(panel).Padding(0, 1),
		Title:             lipgloss.NewStyle().Bold(true).Foreground(text),
		Dim:               lipgloss.NewStyle().Foreground(muted),
		Selected:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")),
		Changed:           lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")),
		Active:            lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		Waiting:           lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		Blocked:           lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		Error:             lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		Maintenance:       lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		Unknown:           lipgloss.NewStyle().Foreground(muted),
		Edge:              lipgloss.NewStyle().Foreground(panel),
		Relation:          lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true),
		RelationSelected:  lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true).Underline(true),
		Placement:         lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "31", Dark: "117"}),
		PlacementSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true),
	}
}

func (s Styles) Status(status domain.Status) lipgloss.Style {
	switch status {
	case domain.StatusActive:
		return s.Active
	case domain.StatusWaiting:
		return s.Waiting
	case domain.StatusBlocked:
		return s.Blocked
	case domain.StatusError:
		return s.Error
	case domain.StatusMaintenance:
		return s.Maintenance
	default:
		return s.Unknown
	}
}

func StatusSymbol(status domain.Status) string {
	switch status {
	case domain.StatusActive:
		return "●"
	case domain.StatusWaiting:
		return "◐"
	case domain.StatusBlocked:
		return "▲"
	case domain.StatusError:
		return "✖"
	case domain.StatusMaintenance:
		return "◒"
	default:
		return "?"
	}
}
