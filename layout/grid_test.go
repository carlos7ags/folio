// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestGridBasicTwoColumns(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 1},
	})
	g.AddChild(NewParagraph("Cell 1", font.Helvetica, 12))
	g.AddChild(NewParagraph("Cell 2", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout to produce output")
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks from grid layout")
	}
}

func TestGridFrAndPx(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackPx, Value: 100},
		{Type: GridTrackFr, Value: 1},
	})
	g.AddChild(NewParagraph("Fixed", font.Helvetica, 12))
	g.AddChild(NewParagraph("Flex", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
}

func TestGridGap(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 1},
	})
	g.SetGap(10, 20) // row gap 10, col gap 20
	g.AddChild(NewParagraph("A", font.Helvetica, 12))
	g.AddChild(NewParagraph("B", font.Helvetica, 12))
	g.AddChild(NewParagraph("C", font.Helvetica, 12))
	g.AddChild(NewParagraph("D", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	// 4 children in 2x2 grid with gaps should produce content.
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
}

func TestGridExplicitPlacement(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 1},
	})
	g.AddChild(NewParagraph("Spanning", font.Helvetica, 12))
	g.SetPlacement(0, GridPlacement{ColStart: 1, ColEnd: 3, RowStart: 1, RowEnd: 2})

	plan := g.PlanLayout(LayoutArea{Width: 600, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output for placed grid item")
	}
}

func TestGridTemplateAreas(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 2},
	})
	g.SetTemplateAreas([][]string{
		{"header", "header"},
		{"sidebar", "content"},
	})
	g.AddChild(NewParagraph("Header", font.Helvetica, 12))
	g.SetPlacement(0, GridPlacement{ColStart: 1, ColEnd: 3, RowStart: 1, RowEnd: 2})
	g.AddChild(NewParagraph("Sidebar", font.Helvetica, 12))
	g.AddChild(NewParagraph("Content", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 600, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
}

func TestGridAutoRows(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
	})
	g.SetAutoRows([]GridTrack{
		{Type: GridTrackPx, Value: 50},
	})
	// Add 5 children — more than explicit rows, so auto rows kick in.
	for range 5 {
		g.AddChild(NewParagraph("Row", font.Helvetica, 12))
	}

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output with auto rows")
	}
	// 5 rows * 50pt each = 250pt minimum.
	if plan.Consumed < 200 {
		t.Errorf("expected consumed >= 200 for 5 auto rows, got %f", plan.Consumed)
	}
}

func TestGridPadding(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	g.SetPaddingAll(Padding{Top: 10, Right: 10, Bottom: 10, Left: 10})
	g.AddChild(NewParagraph("Padded", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	// Consumed should include padding.
	if plan.Consumed < 20 {
		t.Errorf("expected consumed > 20 (includes padding), got %f", plan.Consumed)
	}
}

func TestGridEmpty(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 1},
	})

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	// Empty grid should not panic. Any status is acceptable.
	_ = plan
}

func TestGridSingleChild(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackFr, Value: 1},
		{Type: GridTrackFr, Value: 1},
	})
	g.AddChild(NewParagraph("Only one", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output for single child in grid")
	}
}

func TestGridBackground(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	g.SetBackground(RGB(0.9, 0.9, 0.9))
	g.AddChild(NewParagraph("With BG", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}
}

func TestGridPercentColumns(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackPercent, Value: 30},
		{Type: GridTrackPercent, Value: 70},
	})
	g.AddChild(NewParagraph("30%", font.Helvetica, 12))
	g.AddChild(NewParagraph("70%", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}
}

func TestGridJustifyContentCenter(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackPx, Value: 100},
		{Type: GridTrackPx, Value: 100},
	})
	g.SetJustifyContent(JustifyCenter)
	g.AddChild(NewParagraph("A", font.Helvetica, 12))
	g.AddChild(NewParagraph("B", font.Helvetica, 12))

	// Render through full pipeline to exercise justify-content.
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(g)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
	b := pages[0].Stream.Bytes()
	if len(b) == 0 {
		t.Error("expected content on page")
	}
}

func TestGridAlignItemsCenter(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	g.SetTemplateRows([]GridTrack{{Type: GridTrackPx, Value: 100}})
	g.SetAlignItems(CrossAlignCenter)
	g.AddChild(NewParagraph("Short", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}
	if plan.Consumed < 95 {
		t.Errorf("explicit 100pt row should consume ~100pt, got %f", plan.Consumed)
	}
}

func TestGridPageOverflow(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	for range 30 {
		g.AddChild(NewParagraph("Grid row content.", font.Helvetica, 12))
	}

	r := NewRenderer(612, 200, Margins{Top: 20, Bottom: 20, Left: 20, Right: 20})
	r.Add(g)
	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected ≥2 pages for overflowing grid, got %d", len(pages))
	}
}

func TestGridMinMaxWidth(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{
		{Type: GridTrackPx, Value: 100},
		{Type: GridTrackPx, Value: 150},
	})
	g.AddChild(NewParagraph("A", font.Helvetica, 12))
	g.AddChild(NewParagraph("B", font.Helvetica, 12))

	minW := g.MinWidth()
	maxW := g.MaxWidth()
	if minW < 200 {
		t.Errorf("expected MinWidth ≥ 200, got %f", minW)
	}
	if maxW < minW {
		t.Errorf("MaxWidth (%f) should be ≥ MinWidth (%f)", maxW, minW)
	}
}

func TestGridTemplateRowsExplicit(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	g.SetTemplateRows([]GridTrack{
		{Type: GridTrackPx, Value: 50},
		{Type: GridTrackPx, Value: 80},
	})
	g.AddChild(NewParagraph("Row1", font.Helvetica, 12))
	g.AddChild(NewParagraph("Row2", font.Helvetica, 12))

	plan := g.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}
	if plan.Consumed < 125 || plan.Consumed > 135 {
		t.Errorf("expected ~130pt for explicit rows 50+80, got %f", plan.Consumed)
	}
}

func TestGridRowGapAddsHeight(t *testing.T) {
	gNoGap := NewGrid()
	gNoGap.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	gNoGap.AddChild(NewParagraph("A", font.Helvetica, 12))
	gNoGap.AddChild(NewParagraph("B", font.Helvetica, 12))
	planNoGap := gNoGap.PlanLayout(LayoutArea{Width: 400, Height: 500})

	gWithGap := NewGrid()
	gWithGap.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}})
	gWithGap.SetRowGap(20)
	gWithGap.AddChild(NewParagraph("A", font.Helvetica, 12))
	gWithGap.AddChild(NewParagraph("B", font.Helvetica, 12))
	planWithGap := gWithGap.PlanLayout(LayoutArea{Width: 400, Height: 500})

	diff := planWithGap.Consumed - planNoGap.Consumed
	if diff < 15 || diff > 25 {
		t.Errorf("expected ~20pt difference from row gap, got %f", diff)
	}
}

func TestGridLayoutAPI(t *testing.T) {
	g := NewGrid()
	g.SetTemplateColumns([]GridTrack{{Type: GridTrackFr, Value: 1}, {Type: GridTrackFr, Value: 1}})
	g.AddChild(NewParagraph("A", font.Helvetica, 12))
	g.AddChild(NewParagraph("B", font.Helvetica, 12))

	lines := g.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected lines from Grid.Layout")
	}
	totalH := 0.0
	for _, l := range lines {
		totalH += l.Height
	}
	if totalH <= 0 {
		t.Error("expected positive total height")
	}
}
