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

func TestPlacementPaletteVariesAdjacentGroupEdges(t *testing.T) {
	model := placementTestModel(2)
	edges := model.placementEdges()

	first := model.placementStyle(edges[0])
	second := model.placementStyle(edges[1])

	if styleForeground(first) == styleForeground(second) {
		t.Fatalf("adjacent placement edges should use different palette colors, got %q", styleForeground(first))
	}
}

func TestPlacementAppAndMachineSelectionPreservesPaletteColors(t *testing.T) {
	model := placementTestModel(2)
	edges := model.placementEdges()

	for _, selectedID := range []string{edges[0].appID, edges[0].machineID} {
		model.selectedID = selectedID
		first := model.placementStyle(edges[0])
		second := model.placementStyle(edges[1])

		if !first.GetBold() || !second.GetBold() {
			t.Fatalf("selected app/machine placement edges should be bold for %s", selectedID)
		}
		if styleForeground(first) == styleForeground(second) {
			t.Fatalf("selected app/machine placement edges should preserve palette variation for %s", selectedID)
		}
		if styleForeground(first) == styleForeground(model.styles.PlacementSelected) || styleForeground(second) == styleForeground(model.styles.PlacementSelected) {
			t.Fatalf("selected app/machine placement edges should not collapse to unit selected color for %s", selectedID)
		}
	}
}

func TestSelectedUnitPlacementStyleUsesSelectedPlacementStyle(t *testing.T) {
	model := placementTestModel(2)
	edges := model.placementEdges()
	model.selectedID = edges[0].unitID

	style := model.placementStyle(edges[0])
	if styleForeground(style) != styleForeground(model.styles.PlacementSelected) || style.GetBold() != model.styles.PlacementSelected.GetBold() {
		t.Fatalf("selected unit placement style = foreground %q bold %t, want selected foreground %q bold %t",
			styleForeground(style),
			style.GetBold(),
			styleForeground(model.styles.PlacementSelected),
			model.styles.PlacementSelected.GetBold(),
		)
	}
	if styleForeground(model.placementStyle(edges[1])) == styleForeground(model.styles.PlacementSelected) {
		t.Fatal("unselected placement edge should keep palette color when another unit is selected")
	}
}

func styleForeground(style lipgloss.Style) string {
	return fmt.Sprint(style.GetForeground())
}
