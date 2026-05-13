package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestCanvasRenderPreservesAllocatedWidth(t *testing.T) {
	canvas := NewCanvas(10, 2)
	canvas.Text(1, 0, "x", lipgloss.NewStyle())

	lines := strings.Split(canvas.Render(), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if len(line) != 10 {
			t.Fatalf("line %d width = %d, want 10: %q", i, len(line), line)
		}
	}
}

func TestCanvasBoxWithCustomBorder(t *testing.T) {
	canvas := NewCanvas(6, 3)
	canvas.BoxWithBorder(0, 0, 6, 3, lipgloss.NewStyle(), HeavyBorderRunes)

	rendered := canvas.Render()
	if !strings.Contains(rendered, "┏━━━━┓") {
		t.Fatalf("missing heavy top border: %q", rendered)
	}
	if !strings.Contains(rendered, "┗━━━━┛") {
		t.Fatalf("missing heavy bottom border: %q", rendered)
	}
}
