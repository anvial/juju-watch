package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/anvial/juju-watch/internal/cli"
	"github.com/anvial/juju-watch/internal/domain"
	"github.com/anvial/juju-watch/internal/layout"
)

func TestSetSelectedIDResetsSelectionFrameOnlyWhenChanged(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	model.selectedID = "application:prod:postgresql"
	model.selectionFrame = 7

	model.setSelectedID("application:prod:postgresql")
	if model.selectionFrame != 7 {
		t.Fatalf("same selected ID reset frame to %d", model.selectionFrame)
	}

	model.setSelectedID("machine:prod:0")
	if model.selectionFrame != 0 {
		t.Fatalf("changed selected ID kept frame %d, want reset", model.selectionFrame)
	}
}

func TestFrameMsgAdvancesSelectionAnimationOnlyWhenEnabled(t *testing.T) {
	model := New(cli.DefaultConfig(), nil, layout.Schema{})
	model.selectedID = "application:prod:postgresql"

	updated, _ := model.Update(frameMsg(time.Now()))
	model = updated.(Model)
	if model.selectionFrame != 1 {
		t.Fatalf("selection frame = %d, want advanced frame", model.selectionFrame)
	}

	cfg := cli.DefaultConfig()
	cfg.NoAnimation = true
	model = New(cfg, nil, layout.Schema{})
	model.selectedID = "application:prod:postgresql"
	model.selectionFrame = 4

	updated, _ = model.Update(frameMsg(time.Now()))
	model = updated.(Model)
	if model.selectionFrame != 4 {
		t.Fatalf("no-animation selection frame = %d, want unchanged", model.selectionFrame)
	}

	cfg = cli.DefaultConfig()
	model = New(cfg, nil, layout.Schema{})
	model.selectionFrame = 4

	updated, _ = model.Update(frameMsg(time.Now()))
	model = updated.(Model)
	if model.selectionFrame != 4 {
		t.Fatalf("empty selection frame = %d, want unchanged", model.selectionFrame)
	}
}

func TestSelectedBoxSweepStaysOnBorderWithoutChangingGeometry(t *testing.T) {
	box := renderBox{x: 2, y: 3, width: 12, height: 5}
	before := box

	cells := boxSweepCells(box, DoubleBorderRunes, 4)
	if box != before {
		t.Fatalf("box geometry changed: before=%+v after=%+v", before, box)
	}
	if len(cells) == 0 {
		t.Fatal("expected sweep cells")
	}
	for _, cell := range cells {
		if !pointOnBoxBorder(cell.point, box) {
			t.Fatalf("sweep cell left box border: cell=%+v box=%+v", cell, box)
		}
	}
}

func TestSelectedRelationSweepFollowsRouteAndKeepsGlyphs(t *testing.T) {
	model := relationTestModel()
	edgeID := domain.RelationID("prod", "postgresql:db", "api:database")
	model.selectedID = edgeID
	edge := model.graph.Edges[edgeID]

	route, ok := model.relationRoute(edge, 100, 20)
	if !ok {
		t.Fatal("expected relation route")
	}
	path := map[routePoint]rune{}
	for _, cell := range routePathCells(route, relationSweepGlyphs()) {
		path[cell.point] = cell.ch
	}
	for _, cell := range routeSweepCells(route, relationSweepGlyphs(), 3) {
		want, ok := path[cell.point]
		if !ok {
			t.Fatalf("sweep cell not on relation route: %+v route=%+v", cell, route)
		}
		if cell.ch != want {
			t.Fatalf("sweep glyph = %q, want existing route glyph %q", cell.ch, want)
		}
		if !strings.ContainsRune("━┃╋▶◀▲▼", cell.ch) {
			t.Fatalf("sweep glyph = %q, want relation glyph", cell.ch)
		}
	}
}

func TestSelectedUnitPlacementSweepOnlyUsesUnitEdge(t *testing.T) {
	model := placementTestModel(2)
	edges := model.placementEdges()

	model.selectedID = edges[0].unitID
	for index, edge := range edges {
		want := index == 0
		if got := model.placementSweepActive(edge); got != want {
			t.Fatalf("unit selected placement sweep[%d] = %t, want %t", index, got, want)
		}
	}

	model.selectedID = edges[0].machineID
	for index, edge := range edges {
		if model.placementSweepActive(edge) {
			t.Fatalf("machine selected placement sweep[%d] should be static: %+v", index, edge)
		}
	}

	model.selectedID = edges[0].appID
	for index, edge := range edges {
		if model.placementSweepActive(edge) {
			t.Fatalf("application selected placement sweep[%d] should be static: %+v", index, edge)
		}
	}
}

func TestNoAnimationKeepsSelectedRenderingStableAcrossFrames(t *testing.T) {
	model := placementTestModel(2)
	model.cfg.NoAnimation = true
	model.selectedID = model.placementEdges()[0].unitID

	model.selectionFrame = 0
	first := model.renderTopology(90, 24)
	model.selectionFrame = 17
	second := model.renderTopology(90, 24)
	if first != second {
		t.Fatal("no-animation selected rendering changed across frames")
	}
}

func pointOnBoxBorder(point routePoint, box renderBox) bool {
	onVertical := (point.x == box.x || point.x == box.x+box.width-1) && point.y >= box.y && point.y < box.y+box.height
	onHorizontal := (point.y == box.y || point.y == box.y+box.height-1) && point.x >= box.x && point.x < box.x+box.width
	return onVertical || onHorizontal
}
