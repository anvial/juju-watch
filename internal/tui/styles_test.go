package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRelationStyleUsesMagenta(t *testing.T) {
	styles := NewStyles()
	color, ok := styles.Relation.GetForeground().(lipgloss.Color)
	if !ok {
		t.Fatalf("relation foreground type = %T", styles.Relation.GetForeground())
	}
	if color != lipgloss.Color("5") {
		t.Fatalf("relation foreground = %q, want ANSI magenta", color)
	}
	if !styles.Relation.GetBold() {
		t.Fatal("relation style should be bold")
	}
}

func TestReadableLowEmphasisStylesAvoidBlackAndFaint(t *testing.T) {
	styles := NewStyles()
	cases := map[string]lipgloss.Style{
		"dim":         styles.Dim,
		"footer":      styles.Footer,
		"panel":       styles.Panel,
		"unknown":     styles.Unknown,
		"edge":        styles.Edge,
		"placement":   styles.Placement,
		"waiting":     styles.Waiting,
		"maintenance": styles.Maintenance,
	}
	for name, style := range cases {
		if style.GetFaint() {
			t.Fatalf("%s style should not use faint text", name)
		}
		foreground := fmt.Sprint(style.GetForeground())
		if foreground == "0" || foreground == "8" {
			t.Fatalf("%s foreground = %q, want non-black readable color", name, foreground)
		}
	}
}
