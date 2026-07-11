// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestBorderCollapseRemovesDuplicates(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)

	r1 := tbl.AddRow()
	r1.AddCell("A", font.Helvetica, 10)
	r1.AddCell("B", font.Helvetica, 10)

	r2 := tbl.AddRow()
	r2.AddCell("C", font.Helvetica, 10)
	r2.AddCell("D", font.Helvetica, 10)

	// Layout the table.
	lines := tbl.Layout(400)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(lines))
	}

	// After collapse: first row cells should have no bottom border,
	// and first column cells should have no right border.
	ref0 := lines[0].tableRow
	if ref0 == nil {
		t.Fatal("expected tableRow ref")
	}

	grid := ref0.grid

	// Row 0, Cell 0 (A): should have no right border, no bottom border.
	a := grid[0].cells[0]
	if a.resolved.Right.Width != 0 {
		t.Error("cell A should have no right border after collapse")
	}
	if a.resolved.Bottom.Width != 0 {
		t.Error("cell A should have no bottom border after collapse")
	}
	// The original declared border must survive untouched — resolution
	// must not mutate cell.borders (see resolveBorders).
	if a.cell.borders.Right.Width == 0 {
		t.Error("cell A's original declared border should be unmutated after collapse")
	}

	// Row 0, Cell 1 (B): should have right border (last col), no bottom.
	b := grid[0].cells[1]
	if b.resolved.Right.Width == 0 {
		t.Error("cell B should keep right border (last column)")
	}
	if b.resolved.Bottom.Width != 0 {
		t.Error("cell B should have no bottom border after collapse")
	}

	// Row 1, Cell 0 (C): should have no right border, keeps bottom (last row).
	c := grid[1].cells[0]
	if c.resolved.Right.Width != 0 {
		t.Error("cell C should have no right border after collapse")
	}
	if c.resolved.Bottom.Width == 0 {
		t.Error("cell C should keep bottom border (last row)")
	}

	// Row 1, Cell 1 (D): keeps both right and bottom (last col + last row).
	d := grid[1].cells[1]
	if d.resolved.Right.Width == 0 {
		t.Error("cell D should keep right border (last column)")
	}
	if d.resolved.Bottom.Width == 0 {
		t.Error("cell D should keep bottom border (last row)")
	}
}

func TestBorderCollapseDisabledByDefault(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	lines := tbl.Layout(400)
	ref := lines[0].tableRow
	grid := ref.grid

	// Without collapse, both cells keep all borders.
	a := grid[0].cells[0].cell
	if a.borders.Right.Width == 0 {
		t.Error("without collapse, cell A should keep right border")
	}
}

func TestBorderCollapseRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tbl := NewTable().SetBorderCollapse(true)
	for range 3 {
		row := tbl.AddRow()
		row.AddCell("Col 1", font.Helvetica, 10)
		row.AddCell("Col 2", font.Helvetica, 10)
		row.AddCell("Col 3", font.Helvetica, 10)
	}
	r.Add(tbl)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

// TestResolveEdgeTable directly exercises resolveEdge's CSS2.1 §17.6.2
// priority order: hidden, then width, then style, then source order.
func TestResolveEdgeTable(t *testing.T) {
	solid := func(w float64) Border { return SolidBorder(w, ColorBlack) }
	dashed := func(w float64) Border { return DashedBorder(w, ColorBlack) }
	dotted := func(w float64) Border { return DottedBorder(w, ColorBlack) }
	double := func(w float64) Border { return DoubleBorder(w, ColorBlack) }
	hidden := Border{Style: BorderHidden}
	hiddenWide := Border{Style: BorderHidden, Width: 10} // API-constructed nonzero-width hidden: the rule must hold regardless of width

	nearColor := Border{Width: 1, Style: BorderSolid, Color: Color{R: 1}}
	farColor := Border{Width: 1, Style: BorderSolid, Color: Color{B: 1}}

	tests := []struct {
		name      string
		near, far Border
		want      Border
	}{
		{"wider far wins over narrower near, same style", solid(0.25), solid(3), solid(3)},
		{"wider near wins over narrower far, same style", solid(3), solid(0.25), solid(3)},
		{"equal width: double beats solid", double(1), solid(1), double(1)},
		{"equal width: solid beats dashed", solid(1), dashed(1), solid(1)},
		{"equal width: dashed beats dotted", dashed(1), dotted(1), dashed(1)},
		{"equal width: double beats dashed regardless of side", double(1), dashed(1), double(1)},
		{"hidden near suppresses even a much wider far", hidden, solid(10), Border{}},
		{"hidden far suppresses even a much wider near", solid(10), hidden, Border{}},
		{"hidden with explicit nonzero width still suppresses", hiddenWide, solid(10), Border{}},
		{"both hidden", hidden, hidden, Border{}},
		{"undeclared near loses to any real far border", Border{}, solid(1), solid(1)},
		{"undeclared far loses to any real near border", solid(1), Border{}, solid(1)},
		{"zero-value near not mistaken for a declared solid border", Border{}, Border{}, Border{}},
		{"exact tie (same width, same style): far wins", nearColor, farColor, farColor},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveEdge(tc.near, tc.far)
			if got != tc.want {
				t.Errorf("resolveEdge(%+v, %+v) = %+v, want %+v", tc.near, tc.far, got, tc.want)
			}
		})
	}
}

// TestBorderConflictWiderWins mirrors the reduction's thead/tbody numbers
// on the column axis: a narrower right border loses to a wider left
// border declared on the neighboring cell.
func TestBorderConflictWiderWins(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)
	row := tbl.AddRow()
	left := row.AddCell("L", font.Helvetica, 10)
	left.SetBorders(CellBorders{Right: SolidBorder(0.25, ColorBlack)})
	right := row.AddCell("R", font.Helvetica, 10)
	right.SetBorders(CellBorders{Left: SolidBorder(3, ColorBlack)})

	lines := tbl.Layout(400)
	grid := lines[0].tableRow.grid
	l := grid[0].cells[0]
	r := grid[0].cells[1]

	if l.resolved.Right != (Border{}) {
		t.Errorf("left cell's own resolved.Right should be zero, got %+v", l.resolved.Right)
	}
	if r.resolved.Left != (SolidBorder(3, ColorBlack)) {
		t.Errorf("expected the wider 3pt border to win, got %+v", r.resolved.Left)
	}
}

// TestBorderConflictRowAxis reproduces issue #378's exact reduction: a
// thead cell declares border-bottom: .25pt solid, the tbody cell below it
// declares an explicit border-top: 0pt (folio has no distinct
// representation for a declared-but-zero border, so this is Border{}).
// Per CSS2.1 §17.6.2 the wider, nonzero border must win.
func TestBorderConflictRowAxis(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)
	top := tbl.AddRow()
	topCell := top.AddCell("Top", font.Helvetica, 10)
	topCell.SetBorders(CellBorders{Bottom: SolidBorder(0.25, ColorBlack)})

	bottom := tbl.AddRow()
	bottomCell := bottom.AddCell("Bottom", font.Helvetica, 10)
	bottomCell.SetBorders(CellBorders{})

	lines := tbl.Layout(400)
	grid := lines[0].tableRow.grid
	topGC := grid[0].cells[0]
	bottomGC := grid[1].cells[0]

	if topGC.resolved.Bottom != (Border{}) {
		t.Errorf("top row's own resolved.Bottom should be zero (the winner is recorded once, on the row below), got %+v", topGC.resolved.Bottom)
	}
	if bottomGC.resolved.Top != (SolidBorder(0.25, ColorBlack)) {
		t.Errorf("expected the header's .25pt border to win over the body's explicit 0pt, got %+v", bottomGC.resolved.Top)
	}
}

// TestBorderConflictHiddenWins proves hidden unconditionally suppresses an
// edge, even against a much wider border on the other side — not merely
// "loses" the way a narrow border would.
func TestBorderConflictHiddenWins(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)
	row := tbl.AddRow()
	left := row.AddCell("L", font.Helvetica, 10)
	left.SetBorders(CellBorders{Right: Border{Style: BorderHidden, Width: 5}})
	right := row.AddCell("R", font.Helvetica, 10)
	right.SetBorders(CellBorders{Left: SolidBorder(10, ColorBlack)})

	lines := tbl.Layout(400)
	grid := lines[0].tableRow.grid
	l := grid[0].cells[0]
	r := grid[0].cells[1]

	if l.resolved.Right != (Border{}) {
		t.Errorf("expected nothing drawn on the left cell's side, got %+v", l.resolved.Right)
	}
	if r.resolved.Left != (Border{}) {
		t.Errorf("hidden must suppress the edge even against a much wider border, got %+v", r.resolved.Left)
	}
}

// TestBorderConflictRowspanBottomEdge is a regression for the rowspan
// bottom-edge bug: a rowspan="2" cell starting at row 0 visually ends at
// row 2, not row 1 — its border-bottom must be resolved against row 2's
// border-top, not row 1's (row 1 is entirely covered by the span).
func TestBorderConflictRowspanBottomEdge(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)

	r0 := tbl.AddRow()
	spanCell := r0.AddCell("Span", font.Helvetica, 10)
	spanCell.SetRowspan(2)
	spanCell.SetBorders(CellBorders{Bottom: SolidBorder(2, ColorBlack)})

	tbl.AddRow() // row 1: fully covered by the rowspan above; irrelevant to the edge under test

	r2 := tbl.AddRow()
	r2Cell := r2.AddCell("R2", font.Helvetica, 10)
	r2Cell.SetBorders(CellBorders{Top: SolidBorder(1, ColorBlack)})

	lines := tbl.Layout(400)
	grid := lines[0].tableRow.grid
	span := grid[0].cells[0]
	row2Cell := grid[2].cells[0]

	if span.resolved.Bottom != (Border{}) {
		t.Errorf("span cell's own resolved.Bottom should be zero, got %+v", span.resolved.Bottom)
	}
	if row2Cell.resolved.Top != (SolidBorder(2, ColorBlack)) {
		t.Errorf("expected the rowspanning cell's 2pt border (its true bottom edge is row 2, not row 1) to win, got %+v", row2Cell.resolved.Top)
	}
}

// TestBorderConflictOverflowTableNonDestructive is the pagination hazard
// pin: a header row is reused (same *Row, same *Cell pointers) across the
// overflow continuation table produced by a page split. Resolving borders
// for page one must not corrupt the header's declared border, or page
// two's independent resolution (against a different neighbor row) would
// compare a corrupted value instead of the header's true declaration.
func TestBorderConflictOverflowTableNonDestructive(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)
	h := tbl.AddHeaderRow()
	hc := h.AddCell("Header", font.HelveticaBold, 10)
	hc.SetBorders(CellBorders{Bottom: SolidBorder(1, ColorBlack)})

	for i := range 20 {
		top := SolidBorder(2, ColorBlack)
		if i == 0 {
			top = SolidBorder(5, ColorBlack) // wider than the header on page one
		}
		row := tbl.AddRow()
		row.AddCell("Row", font.Helvetica, 10).SetBorders(CellBorders{Top: top})
	}

	area := LayoutArea{Width: 400, Height: 130} // roughly half of the 20 rows fit
	plan := tbl.PlanLayout(area)
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial (a real page split), got %v", plan.Status)
	}
	if len(plan.Blocks) != 1 || len(plan.Blocks[0].Children) < 2 {
		t.Fatalf("expected page one to render the header plus at least one body row, got %+v", plan.Blocks)
	}

	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	if len(overflow.rows) < 2 || !overflow.rows[0].isHeader {
		t.Fatalf("expected overflow to start with the repeated header, got %d rows", len(overflow.rows))
	}
	if overflow.rows[1] == tbl.rows[1] {
		t.Fatal("expected overflow's first body row to differ from page one's first body row (split landed after row 0)")
	}

	// Reconstruct the exact grid PlanLayout resolved for page one.
	colWidths := tbl.resolveColWidths(area.Width)
	grid1 := tbl.buildGrid(colWidths)
	resolveBorders(grid1, len(colWidths))
	headerCell1 := grid1[0].cells[0]
	row0Cell := grid1[1].cells[0]

	if headerCell1.resolved.Bottom != (Border{}) {
		t.Error("header's own resolved.Bottom should be zero on page one")
	}
	if row0Cell.resolved.Top != (SolidBorder(5, ColorBlack)) {
		t.Errorf("expected row 0's resolved.Top to be the 5pt winner, got %+v", row0Cell.resolved.Top)
	}
	if hc.borders.Bottom != (SolidBorder(1, ColorBlack)) {
		t.Error("header's declared border must survive page one's resolution unmutated")
	}

	// Reconstruct the grid the overflow table's own PlanLayout resolves,
	// independently of page one — this is the assertion that would have
	// failed under a naive in-place-mutation implementation.
	overflowColWidths := overflow.resolveColWidths(area.Width)
	grid2 := overflow.buildGrid(overflowColWidths)
	resolveBorders(grid2, len(overflowColWidths))
	headerCell2 := grid2[0].cells[0]
	overflowRow0Cell := grid2[1].cells[0]

	if headerCell2.resolved.Bottom != (Border{}) {
		t.Error("repeated header's own resolved.Bottom should also be zero on page two")
	}
	if overflowRow0Cell.resolved.Top != (SolidBorder(2, ColorBlack)) {
		t.Errorf("expected page two's row to independently win with its 2pt border (vs the header's narrower 1pt), got %+v", overflowRow0Cell.resolved.Top)
	}
}

// TestBorderConflictPerimeterUnaffected: a cell with no neighbor on a side
// (the table's own outer edge) is never suppressed or compared against
// anything — its resolved value passes through verbatim.
func TestBorderConflictPerimeterUnaffected(t *testing.T) {
	tbl := NewTable().SetBorderCollapse(true)
	row := tbl.AddRow()
	cell := row.AddCell("Only", font.Helvetica, 10)
	cell.SetBorders(CellBorders{
		Top:    SolidBorder(1, ColorBlack),
		Right:  SolidBorder(2, ColorBlack),
		Bottom: SolidBorder(3, ColorBlack),
		Left:   SolidBorder(4, ColorBlack),
	})

	lines := tbl.Layout(400)
	grid := lines[0].tableRow.grid
	gc := grid[0].cells[0]

	want := cell.borders
	if gc.resolved != want {
		t.Errorf("a cell with no neighbors should pass its declared border through unresolved, got %+v want %+v", gc.resolved, want)
	}
}

// TestBuildOwnershipHandlesRowspanColspan verifies neighbor lookups
// account for a cell's full occupied span, not just its starting position.
func TestBuildOwnershipHandlesRowspanColspan(t *testing.T) {
	tbl := NewTable().SetColumnWidths([]float64{100, 100, 100})

	r0 := tbl.AddRow()
	spanCell := r0.AddCell("A", font.Helvetica, 10)
	spanCell.SetColspan(2)
	spanCell.SetRowspan(2)
	bCell := r0.AddCell("B", font.Helvetica, 10)

	r1 := tbl.AddRow()
	r1.AddCell("C", font.Helvetica, 10)

	r2 := tbl.AddRow()
	dCell := r2.AddCell("D", font.Helvetica, 10)
	r2.AddCell("E", font.Helvetica, 10)
	fCell := r2.AddCell("F", font.Helvetica, 10)

	colWidths := []float64{100, 100, 100}
	grid := tbl.buildGrid(colWidths)
	owner := buildOwnership(grid, len(colWidths))

	if owner[0][0] == nil || owner[0][0] != owner[0][1] || owner[0][1] != owner[1][0] || owner[1][0] != owner[1][1] {
		t.Fatalf("expected the spanning cell to own all 4 positions of its span, got %+v %+v %+v %+v",
			owner[0][0], owner[0][1], owner[1][0], owner[1][1])
	}
	if owner[0][0].cell != spanCell {
		t.Error("expected owner[0][0] to be the spanning cell A")
	}

	distinct := []*gridCell{owner[0][2], owner[2][0], owner[2][2]}
	seen := map[*gridCell]bool{}
	for _, gc := range distinct {
		if gc == nil {
			t.Fatal("expected a non-nil owner at every non-spanning position")
		}
		if seen[gc] {
			t.Error("expected owner[0][2], owner[2][0], owner[2][2] to be three distinct cells")
		}
		seen[gc] = true
	}
	if owner[0][2].cell != bCell {
		t.Error("expected owner[0][2] to be cell B")
	}
	if owner[2][0].cell != dCell {
		t.Error("expected owner[2][0] to be cell D")
	}
	if owner[2][2].cell != fCell {
		t.Error("expected owner[2][2] to be cell F")
	}
}
