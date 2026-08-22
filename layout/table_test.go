// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"math"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestTableBasic(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (1 row), got %d", len(lines))
	}
	if !lines[0].IsTable() {
		t.Error("line should be a table row")
	}
}

func TestTableMultipleRows(t *testing.T) {
	tbl := NewTable()
	for range 3 {
		r := tbl.AddRow()
		r.AddCell("Cell", font.Helvetica, 10)
	}
	lines := tbl.Layout(400)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestTableNumCols(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)
	r.AddCell("C", font.Helvetica, 10)

	if tbl.numCols() != 3 {
		t.Errorf("expected 3 columns, got %d", tbl.numCols())
	}
}

func TestTableEqualColumnWidths(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	widths := tbl.resolveColWidths(400)
	if len(widths) != 2 {
		t.Fatalf("expected 2 widths, got %d", len(widths))
	}
	if widths[0] != 200 || widths[1] != 200 {
		t.Errorf("expected 200/200, got %.1f/%.1f", widths[0], widths[1])
	}
}

func TestTableExplicitColumnWidths(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 300})
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	widths := tbl.resolveColWidths(400)
	if widths[0] != 100 || widths[1] != 300 {
		t.Errorf("expected 100/300, got %.1f/%.1f", widths[0], widths[1])
	}
}

func TestTableColspan(t *testing.T) {
	tbl := NewTable()

	// Header row: one cell spanning 2 columns.
	r1 := tbl.AddRow()
	r1.AddCell("Header", font.Helvetica, 10).SetColspan(2)

	// Data row: two separate cells.
	r2 := tbl.AddRow()
	r2.AddCell("A", font.Helvetica, 10)
	r2.AddCell("B", font.Helvetica, 10)

	if tbl.numCols() != 2 {
		t.Errorf("expected 2 columns, got %d", tbl.numCols())
	}

	lines := tbl.Layout(400)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestTableRowHeight(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	// Single word, 10pt font, 1.2 leading = 12pt + 2*4pt padding = 20pt
	r.AddCell("Hi", font.Helvetica, 10)

	lines := tbl.Layout(400)
	expected := 10.0*1.2 + 2*4.0 // 20.0
	if math.Abs(lines[0].Height-expected) > 0.1 {
		t.Errorf("expected height ~%.1f, got %.3f", expected, lines[0].Height)
	}
}

func TestTableCellPadding(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("Hi", font.Helvetica, 10).SetPadding(10)

	lines := tbl.Layout(400)
	expected := 10.0*1.2 + 2*10.0 // 32.0
	if math.Abs(lines[0].Height-expected) > 0.1 {
		t.Errorf("expected height ~%.1f, got %.3f", expected, lines[0].Height)
	}
}

func TestTableCellAlignment(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	c := r.AddCell("Hello", font.Helvetica, 10).SetAlign(AlignCenter)
	if c.align != AlignCenter {
		t.Error("cell alignment should be center")
	}
}

func TestTableNoBorders(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	c := r.AddCell("Hello", font.Helvetica, 10).SetBorders(NoBorders())
	if c.borders.Top.Width != 0 {
		t.Error("expected no top border")
	}
}

func TestTableHeaderRow(t *testing.T) {
	tbl := NewTable()
	h := tbl.AddHeaderRow()
	h.AddCell("Header", font.HelveticaBold, 10)

	r := tbl.AddRow()
	r.AddCell("Data", font.Helvetica, 10)

	lines := tbl.Layout(400)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// First line should be a header.
	if !lines[0].tableRow.grid[lines[0].tableRow.rowIndex].isHeader {
		t.Error("first line should be a header row")
	}
	if lines[1].tableRow.grid[lines[1].tableRow.rowIndex].isHeader {
		t.Error("second line should not be a header row")
	}
}

func TestTableRendererBasic(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell("Hello", font.Helvetica, 10)
	row.AddCell("World", font.Helvetica, 10)

	r.Add(tbl)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Fonts) == 0 {
		t.Error("expected at least 1 font registered")
	}
}

func TestTableRendererPageBreak(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Usable height = 648pt. Each row ~20pt. 648/20 = ~32 rows per page.
	tbl := NewTable()
	for range 40 {
		row := tbl.AddRow()
		row.AddCell("Row data", font.Helvetica, 10)
	}
	r.Add(tbl)
	pages := r.Render()
	if len(pages) < 2 {
		t.Errorf("expected at least 2 pages, got %d", len(pages))
	}
}

func TestTableRendererHeaderRepetition(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tbl := NewTable()
	h := tbl.AddHeaderRow()
	h.AddCell("Name", font.HelveticaBold, 10)
	h.AddCell("Value", font.HelveticaBold, 10)

	// Add enough data rows to force a page break.
	for range 40 {
		row := tbl.AddRow()
		row.AddCell("Key", font.Helvetica, 10)
		row.AddCell("Data", font.Helvetica, 10)
	}

	r.Add(tbl)
	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}

	// Second page should have content (header repeated + data rows).
	if pages[1].Stream == nil {
		t.Error("second page should have content")
	}
	stream := string(pages[1].Stream.Bytes())
	// The header text should appear on page 2 (repeated).
	if !contains(stream, "Name") {
		t.Error("header 'Name' should be repeated on page 2")
	}
}

// TestTableOverflowPreservesAutoWidthsWithCellHints is a regression test for
// a bug where a table that relied on per-cell width hints for sizing (the
// path used by the HTML converter for `<th style="width:N%">`) would silently
// fall back to equal-distribution column widths on continuation pages after
// a page break. The root cause was that Table.PlanLayout's overflow table
// construction copied only a subset of the source table's fields and
// omitted autoWidths, minWidth, and minWidthUnit. With autoWidths=false and
// no colWidths/colUnitWidths set, resolveColWidths fell through to equal
// distribution, shifting the columns visibly between pages.
//
// We exercise the scenario directly: a 3-column table sized via
// SetAutoColumnWidths() + per-cell SetWidthHint calls, forced to split by a
// narrow area Height. The overflow table's resolved column widths must
// match the first page's exactly.
func TestTableOverflowPreservesAutoWidthsWithCellHints(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()

	// Header row with width hints on every cell. The hints pin the
	// intrinsic widths of the auto-sizing algorithm.
	h := tbl.AddHeaderRow()
	h.AddCell("Item", font.HelveticaBold, 10).SetWidthHint(250)
	h.AddCell("Qty", font.HelveticaBold, 10).SetWidthHint(100)
	h.AddCell("Amount", font.HelveticaBold, 10).SetWidthHint(150)

	// Enough body rows to force a split at the Height we pick below.
	for range 20 {
		row := tbl.AddRow()
		row.AddCell("Widget", font.Helvetica, 10)
		row.AddCell("1", font.Helvetica, 10)
		row.AddCell("$10", font.Helvetica, 10)
	}

	const areaWidth = 500.0
	// Height chosen to fit the header + a handful of body rows, forcing
	// the rest to overflow. Exact value doesn't matter — we just need
	// Status: LayoutPartial.
	plan := tbl.PlanLayout(LayoutArea{Width: areaWidth, Height: 100})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial to exercise the overflow path, got %v", plan.Status)
	}

	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected overflow to be *Table, got %T", plan.Overflow)
	}

	// First-page widths computed from the source table.
	wantWidths := tbl.resolveColWidths(areaWidth)
	gotWidths := overflow.resolveColWidths(areaWidth)

	if len(gotWidths) != len(wantWidths) {
		t.Fatalf("overflow column count: got %d, want %d", len(gotWidths), len(wantWidths))
	}
	for i := range wantWidths {
		if math.Abs(gotWidths[i]-wantWidths[i]) > 0.01 {
			t.Errorf("overflow column %d width: got %.2f, want %.2f — overflow table "+
				"should reproduce the source table's column widths exactly so "+
				"continuation pages don't visibly shift",
				i, gotWidths[i], wantWidths[i])
		}
	}
}

// TestTableOverflowPreservesMinWidth guards the less common path where a
// table's total width is constrained by SetMinWidth. The overflow must
// carry the same constraint, otherwise the continuation table would shrink
// to fit its content (still auto-sized) and drift below the first page's
// total width.
func TestTableOverflowPreservesMinWidth(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	tbl.SetMinWidth(450)

	tbl.AddHeaderRow().
		AddCell("A", font.HelveticaBold, 10)
	for range 20 {
		tbl.AddRow().AddCell("x", font.Helvetica, 10)
	}

	const areaWidth = 500.0
	plan := tbl.PlanLayout(LayoutArea{Width: areaWidth, Height: 80})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}

	want := tbl.resolveColWidths(areaWidth)
	got := overflow.resolveColWidths(areaWidth)
	if len(got) != len(want) || math.Abs(got[0]-want[0]) > 0.01 {
		t.Errorf("overflow minWidth not preserved: got %v, want %v", got, want)
	}
}

// TestTableOverflowPreservesBorderCollapseAndSpacing is a coverage guard
// for the fields that were already copied by the old field-by-field
// construction. The struct-copy fix must not regress them.
func TestTableOverflowPreservesBorderCollapseAndSpacing(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	tbl.SetBorderCollapse(true)
	tbl.SetCellSpacing(3, 5)

	tbl.AddHeaderRow().AddCell("H", font.HelveticaBold, 10)
	for range 20 {
		tbl.AddRow().AddCell("b", font.Helvetica, 10)
	}

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 80})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	if !overflow.BorderCollapse() {
		t.Error("overflow lost borderCollapse flag")
	}
	if overflow.cellSpacingH != 3 || overflow.cellSpacingV != 5 {
		t.Errorf("overflow lost cellSpacing: got (%v, %v), want (3, 5)",
			overflow.cellSpacingH, overflow.cellSpacingV)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTableEmptyTable(t *testing.T) {
	tbl := NewTable()
	lines := tbl.Layout(400)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty table, got %d", len(lines))
	}
}

func TestTableEmptyRow(t *testing.T) {
	tbl := NewTable()
	tbl.AddRow() // empty row, no cells
	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestTableRowspan(t *testing.T) {
	tbl := NewTable()

	r1 := tbl.AddRow()
	r1.AddCell("Span", font.Helvetica, 10).SetRowspan(2)
	r1.AddCell("B1", font.Helvetica, 10)

	r2 := tbl.AddRow()
	// First column is occupied by rowspan, so only one cell.
	r2.AddCell("B2", font.Helvetica, 10)

	lines := tbl.Layout(400)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// The spanning cell must cover both rows, not just its starting row.
	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	span := grid[0].cells[0]
	if span.cell.text != "Span" {
		t.Fatalf("expected first cell to be the span cell, got %q", span.cell.text)
	}
	want := grid[0].height + grid[1].height // sv == 0 by default
	if span.spanHeight != want {
		t.Errorf("span height: got %g, want %g (row0 %g + row1 %g)",
			span.spanHeight, want, grid[0].height, grid[1].height)
	}
	if span.spanHeight <= grid[0].height {
		t.Errorf("span height %g should exceed a single row height %g",
			span.spanHeight, grid[0].height)
	}
	// Single-row cells carry no spanHeight.
	if b1 := grid[0].cells[1]; b1.spanHeight != 0 {
		t.Errorf("non-spanning cell B1: spanHeight got %g, want 0", b1.spanHeight)
	}
}

func TestTableRowspanIncludesVerticalSpacing(t *testing.T) {
	tbl := NewTable()
	tbl.SetCellSpacing(0, 6) // 6pt vertical gap between rows

	r1 := tbl.AddRow()
	r1.AddCell("Span", font.Helvetica, 10).SetRowspan(2)
	r1.AddCell("B1", font.Helvetica, 10)
	r2 := tbl.AddRow()
	r2.AddCell("B2", font.Helvetica, 10)

	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	span := grid[0].cells[0]
	// Span reaches the bottom of row 1, so it must include the gap between rows.
	want := grid[0].height + grid[1].height + 6
	if span.spanHeight != want {
		t.Errorf("span height with spacing: got %g, want %g", span.spanHeight, want)
	}
}

func TestTableRowspanDeficitGrowsLastRow(t *testing.T) {
	// A tall rowspan cell whose content needs more height than its spanned
	// rows naturally provide must grow the last spanned row, and its
	// spanHeight must cover the grown rows.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{60, 200})

	r1 := tbl.AddRow()
	// Narrow column forces the span cell's text to wrap into many lines.
	r1.AddCell("one two three four five six seven eight nine ten", font.Helvetica, 10).SetRowspan(2)
	r1.AddCell("B1", font.Helvetica, 10)
	r2 := tbl.AddRow()
	r2.AddCell("B2", font.Helvetica, 10)

	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	span := grid[0].cells[0]

	// spanHeight equals the (possibly grown) two rows plus their gap (sv == 0).
	wantSpan := grid[0].height + grid[1].height
	if span.spanHeight != wantSpan {
		t.Errorf("span height: got %g, want %g", span.spanHeight, wantSpan)
	}
	// The cell's full content must fit within the span.
	need := tbl.cellContentHeight(&grid[0].cells[0])
	if span.spanHeight+0.01 < need {
		t.Errorf("span height %g does not cover content height %g", span.spanHeight, need)
	}
}

func TestTableDefaultBorder(t *testing.T) {
	b := DefaultBorder()
	if b.Width != 0.5 {
		t.Errorf("expected width 0.5, got %f", b.Width)
	}
	if b.Color != ColorBlack {
		t.Error("default border should be black")
	}
}

func TestTableAllBorders(t *testing.T) {
	b := AllBorders(SolidBorder(1, ColorRed))
	if b.Top.Width != 1 || b.Right.Width != 1 || b.Bottom.Width != 1 || b.Left.Width != 1 {
		t.Error("all borders should have width 1")
	}
}

func TestTableRowspanThreeRows(t *testing.T) {
	// A cell spanning three rows must cover all three (plus the two gaps
	// between them), not just the first.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	tbl.SetCellSpacing(0, 5)

	r1 := tbl.AddRow()
	r1.AddCell("Span3", font.Helvetica, 10).SetRowspan(3)
	r1.AddCell("B1", font.Helvetica, 10)
	r2 := tbl.AddRow()
	r2.AddCell("B2", font.Helvetica, 10)
	r3 := tbl.AddRow()
	r3.AddCell("B3", font.Helvetica, 10)

	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	span := grid[0].cells[0]
	want := grid[0].height + grid[1].height + grid[2].height + 2*5 // two inter-row gaps
	if span.spanHeight != want {
		t.Errorf("3-row span height: got %g, want %g", span.spanHeight, want)
	}
}

func TestTableColspanAndRowspanCombined(t *testing.T) {
	// A cell with both colspan=2 and rowspan=2: width spans two columns,
	// height spans two rows, and the following row's cell lands past the
	// spanned columns.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{50, 50, 50})

	r1 := tbl.AddRow()
	r1.AddCell("Big", font.Helvetica, 10).SetColspan(2).SetRowspan(2)
	r1.AddCell("C1", font.Helvetica, 10)
	r2 := tbl.AddRow()
	// Cols 0-1 occupied by the rowspan; this cell goes to col 2.
	r2.AddCell("C2", font.Helvetica, 10)

	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	big := grid[0].cells[0]
	if big.spanWidth != 100 {
		t.Errorf("colspan width: got %g, want 100", big.spanWidth)
	}
	if want := grid[0].height + grid[1].height; big.spanHeight != want {
		t.Errorf("rowspan height: got %g, want %g", big.spanHeight, want)
	}
	if c2 := grid[1].cells[0]; c2.col != 2 {
		t.Errorf("row 2 cell should start at col 2, got %d", c2.col)
	}
}

func TestTableRowspanFewerCellsDecrement(t *testing.T) {
	// 3 columns. Row 1: cell spanning 3 rows in col 0, plus cells in cols 1-2.
	// Row 2: only 1 cell (goes to col 1, col 0 is occupied by rowspan, col 2 unvisited).
	// Row 3: only 1 cell. If colOccupied for col 2 isn't decremented in row 2,
	// row 3's cell would be misplaced.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})

	r1 := tbl.AddRow()
	r1.AddCell("Span3", font.Helvetica, 10).SetRowspan(3)
	r1.AddCell("B1", font.Helvetica, 10)
	r1.AddCell("C1", font.Helvetica, 10).SetRowspan(2) // occupies col 2 for rows 1-2

	r2 := tbl.AddRow()
	// Col 0 occupied (rowspan from r1), col 2 occupied (rowspan from r1).
	// Only col 1 is free.
	r2.AddCell("B2", font.Helvetica, 10)

	r3 := tbl.AddRow()
	// Col 0 still occupied (rowspan=3 from r1), cols 1-2 should be free now.
	r3.AddCell("B3", font.Helvetica, 10)
	r3.AddCell("C3", font.Helvetica, 10)

	colWidths := []float64{100, 100, 100}
	grid := tbl.buildGrid(colWidths)

	if len(grid) != 3 {
		t.Fatalf("expected 3 grid rows, got %d", len(grid))
	}

	// Row 3 (index 2) should have 2 cells: B3 at col 1, C3 at col 2.
	if len(grid[2].cells) != 2 {
		t.Fatalf("row 3: expected 2 cells, got %d", len(grid[2].cells))
	}
	if grid[2].cells[0].col != 1 {
		t.Errorf("row 3 cell 0: expected col=1, got %d", grid[2].cells[0].col)
	}
	if grid[2].cells[1].col != 2 {
		t.Errorf("row 3 cell 1: expected col=2, got %d", grid[2].cells[1].col)
	}
}

// --- Sprint B: Cell background and vertical alignment ---

func TestCellBackground(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	c := r.AddCell("test", font.Helvetica, 10)
	bg := RGB(0.9, 0.9, 0.9)
	c.SetBackground(bg)

	if c.bgColor == nil {
		t.Fatal("expected bgColor to be set")
	}
	if *c.bgColor != bg {
		t.Errorf("expected %+v, got %+v", bg, *c.bgColor)
	}
}

func TestCellVAlignMiddle(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{200})
	r := tbl.AddRow()
	c := r.AddCell("short", font.Helvetica, 10)
	c.SetVAlign(VAlignMiddle)

	if c.valign != VAlignMiddle {
		t.Errorf("expected VAlignMiddle, got %d", c.valign)
	}

	// Layout should succeed.
	lines := tbl.Layout(200)
	if len(lines) == 0 {
		t.Error("expected at least 1 line")
	}
}

func TestCellVAlignBottom(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	c := r.AddCell("bottom", font.Helvetica, 10)
	c.SetVAlign(VAlignBottom)
	if c.valign != VAlignBottom {
		t.Errorf("expected VAlignBottom, got %d", c.valign)
	}
}

func TestCellBackgroundAndVAlign(t *testing.T) {
	// Both set together should work.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	r := tbl.AddRow()
	c1 := r.AddCell("A", font.Helvetica, 10).SetBackground(RGB(1, 0, 0)).SetVAlign(VAlignMiddle)
	c2 := r.AddCell("B", font.Helvetica, 10).SetBackground(RGB(0, 0, 1)).SetVAlign(VAlignBottom)

	if c1.bgColor == nil || c2.bgColor == nil {
		t.Error("both cells should have backgrounds")
	}

	lines := tbl.Layout(200)
	if len(lines) == 0 {
		t.Error("expected layout lines")
	}
}

// --- Rich table cell (AddCellElement) tests ---

func TestCellElementParagraph(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{400})
	r := tbl.AddRow()
	p := NewParagraph("Hello World", font.Helvetica, 12)
	r.AddCellElement(p)

	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Row height should account for paragraph content plus default padding (4pt * 2).
	expectedMin := 12.0*1.2 + 2*4.0 // at least one line of 12pt text with leading + padding
	if lines[0].Height < expectedMin-0.1 {
		t.Errorf("expected row height >= %.1f, got %.3f", expectedMin, lines[0].Height)
	}
}

func TestCellElementStyledParagraph(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{400})
	r := tbl.AddRow()
	sp := NewStyledParagraph(
		NewRun("Bold ", font.HelveticaBold, 12).WithColor(RGB(1, 0, 0)),
		NewRun("Normal ", font.Helvetica, 12).WithColor(RGB(0, 0, 1)),
		NewRun("Italic", font.HelveticaOblique, 12).WithColor(RGB(0, 1, 0)),
	)
	r.AddCellElement(sp)

	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	expectedMin := 12.0*1.2 + 2*4.0
	if lines[0].Height < expectedMin-0.1 {
		t.Errorf("expected row height >= %.1f, got %.3f", expectedMin, lines[0].Height)
	}
}

func TestCellElementList(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{400})
	r := tbl.AddRow()
	lst := NewList(font.Helvetica, 10)
	lst.AddItem("Item one")
	lst.AddItem("Item two")
	lst.AddItem("Item three")
	r.AddCellElement(lst)

	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// A list with 3 items should be taller than a single text line.
	singleLineHeight := 10.0*1.2 + 2*4.0
	if lines[0].Height <= singleLineHeight {
		t.Errorf("expected row height > %.1f (single line), got %.3f", singleLineHeight, lines[0].Height)
	}
}

func TestCellElementNestedTable(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{400})
	r := tbl.AddRow()

	inner := NewTable()
	inner.SetColumnWidths([]float64{200, 200})
	ir := inner.AddRow()
	ir.AddCell("Inner A", font.Helvetica, 10)
	ir.AddCell("Inner B", font.Helvetica, 10)

	r.AddCellElement(inner)

	// Should not panic.
	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Height <= 0 {
		t.Error("expected positive row height for nested table")
	}
}

func TestCellElementVAlignMiddle(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{200, 200})
	r := tbl.AddRow()

	// Short cell with VAlignMiddle.
	shortP := NewParagraph("Short", font.Helvetica, 10)
	r.AddCellElement(shortP).SetVAlign(VAlignMiddle)

	// Tall cell: use a paragraph with enough text to wrap and force a taller row.
	tallP := NewParagraph("This is a much longer paragraph that should wrap across multiple lines to force a taller row height in the table", font.Helvetica, 10)
	r.AddCellElement(tallP)

	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// The row height should be driven by the tall cell, not the short one.
	singleLineHeight := 10.0*1.2 + 2*4.0
	if lines[0].Height <= singleLineHeight+0.1 {
		t.Errorf("expected row height > %.1f (single line), got %.3f", singleLineHeight, lines[0].Height)
	}
}

func TestCellElementWithBordersAndBg(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{400})
	r := tbl.AddRow()
	p := NewParagraph("Styled cell", font.Helvetica, 12)
	c := r.AddCellElement(p)
	c.SetBorders(AllBorders(SolidBorder(1, ColorBlack)))
	c.SetBackground(RGB(0.95, 0.95, 0.95))

	// Should not panic and should produce a valid layout.
	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Height <= 0 {
		t.Error("expected positive row height")
	}
}

func TestCellElementMixedWithText(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{200, 200})
	r := tbl.AddRow()

	// Plain text cell.
	r.AddCell("Plain text", font.Helvetica, 10)

	// Element cell with a paragraph.
	p := NewParagraph("Element text", font.Helvetica, 10)
	r.AddCellElement(p)

	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Height <= 0 {
		t.Error("expected positive row height")
	}
}

func TestRendererRichTableCell(t *testing.T) {
	rend := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tbl := NewTable()
	row := tbl.AddRow()
	p := NewParagraph("Rich cell content", font.Helvetica, 10)
	row.AddCellElement(p)

	rend.Add(tbl)
	pages := rend.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if pages[0].Stream == nil {
		t.Error("expected non-nil output stream")
	}
	if len(pages[0].Fonts) == 0 {
		t.Error("expected at least 1 font registered")
	}
}

func TestTableZeroColumns(t *testing.T) {
	// A table with no rows should have 0 columns and not panic.
	tbl := NewTable()
	if tbl.numCols() != 0 {
		t.Errorf("expected 0 columns for empty table, got %d", tbl.numCols())
	}
	widths := tbl.resolveColWidths(400)
	if widths != nil {
		t.Errorf("expected nil widths for 0-column table, got %v", widths)
	}
	// PlanLayout should also not panic.
	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull for empty table, got %d", plan.Status)
	}
}

func TestTableZeroWidthColumn(t *testing.T) {
	// Explicit column widths that sum to 0 should not cause division by zero.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{0, 0})
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	// Should not panic.
	lines := tbl.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

// --- border-spacing tests ---

func TestTableCellSpacingColumnWidths(t *testing.T) {
	// With 2 columns and 5pt horizontal spacing, 3 gaps (left, between, right)
	// consume 15pt, leaving 385pt for 2 columns = 192.5pt each.
	tbl := NewTable()
	tbl.SetCellSpacing(5, 0)
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	widths := tbl.resolveColWidths(400)
	if len(widths) != 2 {
		t.Fatalf("expected 2 widths, got %d", len(widths))
	}
	expected := (400.0 - 3*5.0) / 2.0 // 192.5
	if math.Abs(widths[0]-expected) > 0.01 {
		t.Errorf("expected column width %.2f, got %.2f", expected, widths[0])
	}
	if math.Abs(widths[1]-expected) > 0.01 {
		t.Errorf("expected column width %.2f, got %.2f", expected, widths[1])
	}
}

func TestTableCellSpacingVerticalHeight(t *testing.T) {
	// 2 rows with 10pt vertical spacing: 3 gaps (top, between, bottom) = 30pt.
	// Each row is about 20pt (10pt*1.2 + 2*4pt padding).
	// Total height via Layout lines should include spacing.
	tbl := NewTable()
	tbl.SetCellSpacing(0, 10)
	for range 2 {
		r := tbl.AddRow()
		r.AddCell("X", font.Helvetica, 10)
	}

	lines := tbl.Layout(400)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	totalH := 0.0
	for _, l := range lines {
		totalH += l.Height
	}

	// Without spacing, total would be ~40pt. With 3 gaps of 10pt each, ~70pt.
	rowH := 10.0*1.2 + 2*4.0 // 20pt per row
	expectedTotal := 2*rowH + 3*10.0
	if math.Abs(totalH-expectedTotal) > 0.1 {
		t.Errorf("expected total height ~%.1f, got %.1f", expectedTotal, totalH)
	}
}

func TestTableCellSpacingPlanLayout(t *testing.T) {
	// Verify PlanLayout positions rows with spacing gaps.
	tbl := NewTable()
	tbl.SetCellSpacing(0, 10)
	for range 2 {
		r := tbl.AddRow()
		r.AddCell("X", font.Helvetica, 10)
	}

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}

	// The outer Table block wraps all TR blocks.
	if len(plan.Blocks) != 1 || plan.Blocks[0].Tag != "Table" {
		t.Fatal("expected a single Table wrapper block")
	}
	rowBlocks := plan.Blocks[0].Children
	if len(rowBlocks) != 2 {
		t.Fatalf("expected 2 row blocks, got %d", len(rowBlocks))
	}

	// First row should be at Y = 10 (top spacing gap).
	if math.Abs(rowBlocks[0].Y-10) > 0.01 {
		t.Errorf("first row Y: expected 10, got %.2f", rowBlocks[0].Y)
	}

	// Second row should be at Y = 10 + rowH + 10.
	rowH := rowBlocks[0].Height
	expectedY2 := 10 + rowH + 10
	if math.Abs(rowBlocks[1].Y-expectedY2) > 0.01 {
		t.Errorf("second row Y: expected %.2f, got %.2f", expectedY2, rowBlocks[1].Y)
	}

	// Consumed height should include bottom spacing.
	expectedConsumed := 10 + rowH + 10 + rowH + 10
	if math.Abs(plan.Consumed-expectedConsumed) > 0.01 {
		t.Errorf("consumed: expected %.2f, got %.2f", expectedConsumed, plan.Consumed)
	}
}

func TestTablePlanLayoutNoAvailableHeight(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("One", font.Helvetica, 10)
	r.AddCell("Two", font.Helvetica, 10)

	for _, h := range []float64{0, -50} {
		plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: h})
		if plan.Status != LayoutNothing {
			t.Fatalf("height %.1f: expected LayoutNothing, got %d", h, plan.Status)
		}
	}
}

func TestTableCellSpacingIgnoredWithCollapse(t *testing.T) {
	// When border-collapse is enabled, spacing should be ignored.
	tbl := NewTable()
	tbl.SetCellSpacing(10, 10)
	tbl.SetBorderCollapse(true)
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)

	// Column widths should not be reduced by spacing.
	widths := tbl.resolveColWidths(400)
	if len(widths) != 2 {
		t.Fatalf("expected 2 widths, got %d", len(widths))
	}
	if math.Abs(widths[0]-200) > 0.01 {
		t.Errorf("expected column width 200 (collapse ignores spacing), got %.2f", widths[0])
	}

	// Total height via Layout should not include any spacing gaps.
	lines := tbl.Layout(400)
	totalH := 0.0
	for _, l := range lines {
		totalH += l.Height
	}
	rowH := 10.0*1.2 + 2*4.0
	if math.Abs(totalH-rowH) > 0.1 {
		t.Errorf("expected height ~%.1f (no spacing), got %.1f", rowH, totalH)
	}
}

func TestTableCellSpacingSetMethod(t *testing.T) {
	tbl := NewTable()
	ret := tbl.SetCellSpacing(5, 8)
	if ret != tbl {
		t.Error("SetCellSpacing should return the table for chaining")
	}
	if tbl.cellSpacingH != 5 || tbl.cellSpacingV != 8 {
		t.Errorf("expected spacing 5/8, got %.1f/%.1f", tbl.cellSpacingH, tbl.cellSpacingV)
	}
}

func TestTableFooterRowRepeats(t *testing.T) {
	// Footer rows should appear on every page.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{200, 200})

	header := tbl.AddHeaderRow()
	header.AddCell("Header A", font.HelveticaBold, 10)
	header.AddCell("Header B", font.HelveticaBold, 10)

	footer := tbl.AddFooterRow()
	footer.AddCell("Footer A", font.Helvetica, 9)
	footer.AddCell("Footer B", font.Helvetica, 9)

	for i := range 30 {
		row := tbl.AddRow()
		row.AddCell("Data", font.Helvetica, 10)
		row.AddCell("Row "+string(rune('0'+i%10)), font.Helvetica, 10)
	}

	// Very short page to force multi-page.
	r := NewRenderer(612, 200, Margins{Top: 20, Bottom: 20, Left: 20, Right: 20})
	r.Add(tbl)
	pages := r.Render()

	if len(pages) < 2 {
		t.Fatalf("expected ≥2 pages, got %d", len(pages))
	}
	// Both pages should have content (header + footer + some rows).
	for i, p := range pages {
		if len(p.Stream.Bytes()) == 0 {
			t.Errorf("page %d has empty stream", i)
		}
	}
}

func TestTableCellPaddingSidesLayout(t *testing.T) {
	// Asymmetric padding should produce different content positioning
	// than uniform padding.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{300})

	row := tbl.AddRow()
	cell := row.AddCell("Padded", font.Helvetica, 12)
	cell.SetPaddingSides(Padding{Top: 20, Right: 5, Bottom: 5, Left: 40})

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	// Top padding 20 + content ~14 + bottom 5 = ~39pt row height.
	// Uniform padding 4 would give ~22pt.
	if plan.Consumed < 35 {
		t.Errorf("asymmetric padding should produce taller row: consumed=%f", plan.Consumed)
	}
}

func TestTableBorderRadiusRendered(t *testing.T) {
	// Cell with uniform border + radius → stream should contain curve operators.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{200})
	row := tbl.AddRow()
	cell := row.AddCell("Rounded", font.Helvetica, 12)
	cell.SetBorderRadius(10)
	cell.SetBackground(RGB(0.9, 0.9, 0.9))
	cell.SetBorders(AllBorders(SolidBorder(1, ColorBlack)))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(tbl)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	curves := countOps(b, "c")
	if curves < 4 {
		t.Errorf("cell with radius should have ≥4 curve operators, got %d", curves)
	}
}

func TestTableMixedBorderRadiusRendered(t *testing.T) {
	// Cell with only bottom + left borders and bottom-left radius.
	// The rounded corner should still render as curves.
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{200})
	row := tbl.AddRow()
	cell := row.AddCell("Mixed", font.Helvetica, 12)
	cell.SetBorderRadiusPerCorner(0, 0, 0, 10) // only BL
	cell.SetBorders(CellBorders{
		Bottom: SolidBorder(1, ColorBlack),
		Left:   SolidBorder(1, ColorBlack),
	})

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(tbl)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Bottom border should have a BL corner arc (1 curve op).
	// Left border should also have a BL corner arc (1 curve op).
	curves := countOps(b, "c")
	if curves < 2 {
		t.Errorf("mixed-border cell with BL radius should have ≥2 curve operators, got %d", curves)
	}
}

func TestTableCollapseRemovesDuplicateBorders(t *testing.T) {
	// In collapse mode, interior cell borders are removed.
	// Verify by rendering: collapse stream should have fewer stroke ops.
	makeTable := func(collapse bool) []byte {
		tbl := NewTable()
		tbl.SetColumnWidths([]float64{100, 100})
		if collapse {
			tbl.SetBorderCollapse(true)
		}
		row := tbl.AddRow()
		row.AddCell("A", font.Helvetica, 10).SetBorders(AllBorders(SolidBorder(1, ColorBlack)))
		row.AddCell("B", font.Helvetica, 10).SetBorders(AllBorders(SolidBorder(1, ColorBlack)))
		row2 := tbl.AddRow()
		row2.AddCell("C", font.Helvetica, 10).SetBorders(AllBorders(SolidBorder(1, ColorBlack)))
		row2.AddCell("D", font.Helvetica, 10).SetBorders(AllBorders(SolidBorder(1, ColorBlack)))

		r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
		r.Add(tbl)
		pages := r.Render()
		return pages[0].Stream.Bytes()
	}

	sepStrokes := countOps(makeTable(false), "S")
	colStrokes := countOps(makeTable(true), "S")
	// Collapse removes interior right and bottom borders, so fewer strokes.
	if colStrokes >= sepStrokes {
		t.Errorf("collapse should have fewer strokes: separate=%d, collapse=%d", sepStrokes, colStrokes)
	}
}

func TestTableBorderCollapseGetter(t *testing.T) {
	tbl := NewTable()
	if tbl.BorderCollapse() {
		t.Error("default should be separate (false)")
	}
	tbl.SetBorderCollapse(true)
	if !tbl.BorderCollapse() {
		t.Error("expected true after SetBorderCollapse(true)")
	}
}

func TestTableAutoColumnWidthsContentSized(t *testing.T) {
	// Auto-widths: columns sized by content. A wide cell should get more space.
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	row := tbl.AddRow()
	row.AddCell("Short", font.Helvetica, 10)
	row.AddCell("This is a much longer cell with more content", font.Helvetica, 10)

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

// TestTableSplitNoSpanCharacterization pins the span-free split arithmetic
// so the rowspan-group fix below provably leaves it unchanged: no cell in
// this table has rowspan > 1, so every row is its own group and the group
// fit check must reduce to the plain per-row check.
func TestTableSplitNoSpanCharacterization(t *testing.T) {
	tbl := NewTable()
	h := tbl.AddHeaderRow()
	h.AddCell("Header", font.HelveticaBold, 10)
	for range 20 {
		tbl.AddRow().AddCell("Row", font.Helvetica, 10)
	}

	// Height chosen so roughly half the rows fit.
	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 130})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	if len(plan.Blocks) != 1 || plan.Blocks[0].Tag != "Table" {
		t.Fatalf("expected one wrapping Table block, got %+v", plan.Blocks)
	}
	renderedRows := len(plan.Blocks[0].Children)

	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	overflowPlan := overflow.PlanLayout(LayoutArea{Width: 400, Height: 10000})
	if overflowPlan.Status != LayoutFull {
		t.Fatalf("expected overflow to fit fully, got %v", overflowPlan.Status)
	}
	overflowRows := overflowPlan.Blocks[0].Children
	if len(overflowRows) == 0 || overflowRows[0].Tag != "TR" {
		t.Fatalf("expected overflow rows, got %+v", overflowRows)
	}

	// Header is rendered on page one but not counted as a data row; the
	// overflow re-adds its own header. Total data rows across both pages
	// must equal the 20 body rows, and the overflow's first row is a
	// repeated header.
	pageOneDataRows := renderedRows - 1 // minus the header row
	overflowDataRows := len(overflowRows) - 1
	if pageOneDataRows+overflowDataRows != 20 {
		t.Errorf("data rows: page one %d + overflow %d = %d, want 20",
			pageOneDataRows, overflowDataRows, pageOneDataRows+overflowDataRows)
	}
	if len(overflow.rows) == 0 || !overflow.rows[0].isHeader {
		t.Error("expected overflow's first row to be the repeated header")
	}
}

// TestSpanGroupStarts is a table-driven unit test of the span group
// computation: identity for span-free grids, a simple span, chained
// overlapping spans, and a span clipped by the end of the grid.
func TestSpanGroupStarts(t *testing.T) {
	plainCell := func() gridCell { return gridCell{cell: &Cell{rowspan: 1}} }
	spanCell := func(n int) gridCell { return gridCell{cell: &Cell{rowspan: n}} }

	tests := []struct {
		name string
		grid []gridRow
		want []int
	}{
		{
			name: "no spans is identity",
			grid: []gridRow{
				{cells: []gridCell{plainCell()}},
				{cells: []gridCell{plainCell()}},
				{cells: []gridCell{plainCell()}},
			},
			want: []int{0, 1, 2},
		},
		{
			name: "simple 3-row span",
			grid: []gridRow{
				{cells: []gridCell{spanCell(3)}},
				{cells: []gridCell{plainCell()}},
				{cells: []gridCell{plainCell()}},
				{cells: []gridCell{plainCell()}},
			},
			want: []int{0, 0, 0, 3},
		},
		{
			name: "chained overlapping spans",
			grid: []gridRow{
				{cells: []gridCell{plainCell()}},
				{cells: []gridCell{spanCell(2)}},
				{cells: []gridCell{spanCell(2)}},
				{cells: []gridCell{plainCell()}},
			},
			// Row 1 spans rows 1-2; row 2's span extends the group to row 3.
			want: []int{0, 1, 1, 1},
		},
		{
			name: "span clipped by grid end",
			grid: []gridRow{
				{cells: []gridCell{plainCell()}},
				{cells: []gridCell{spanCell(5)}},
			},
			want: []int{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spanGroupStarts(tt.grid)
			if len(got) != len(tt.want) {
				t.Fatalf("length: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("row %d: got group start %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// buildRowspanSplitTable builds a 2-column, 12-row table: 6 plain rows,
// then a 3-row group (row 7 has a rowspan(3) cell plus a plain cell; rows
// 8-9 each have one cell that lands under the span), then 3 more plain
// rows. Every row is 20pt tall (verified by the tests below), so callers
// can pick a Height in multiples of 20 to force a split at a known row.
func buildRowspanSplitTable() *Table {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	for i := 1; i <= 6; i++ {
		tbl.AddRow().AddCell(fmt.Sprintf("Row%d", i), font.Helvetica, 10)
	}
	r7 := tbl.AddRow()
	r7.AddCell("Span", font.Helvetica, 10).SetRowspan(3)
	r7.AddCell("Row7", font.Helvetica, 10)
	tbl.AddRow().AddCell("Row8", font.Helvetica, 10)
	tbl.AddRow().AddCell("Row9", font.Helvetica, 10)
	for i := 10; i <= 12; i++ {
		tbl.AddRow().AddCell(fmt.Sprintf("Row%d", i), font.Helvetica, 10)
	}
	return tbl
}

// TestTableRowspanSplitPushesGroupToNextPage is the core regression for
// issue #362: a split that would otherwise land inside a rowspan group
// must move up to the group's first row instead.
func TestTableRowspanSplitPushesGroupToNextPage(t *testing.T) {
	tbl := buildRowspanSplitTable()

	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	if len(grid) != 12 {
		t.Fatalf("expected 12 grid rows, got %d", len(grid))
	}
	if grid[0].height != 20 {
		t.Fatalf("row height assumption broken: got %g, want 20 (adjust Height below)", grid[0].height)
	}

	// 170 = 6 rows (120) + the group leader's own row (20) + 30 spare —
	// enough for the group leader alone but not the full 3-row group.
	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 170})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected one wrapping Table block, got %d", len(plan.Blocks))
	}
	if got := len(plan.Blocks[0].Children); got != 6 {
		t.Errorf("page one TR count: got %d, want 6 (split pushed up to the group leader)", got)
	}

	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	ogrid := overflow.buildGrid(overflow.resolveColWidths(400))
	if len(ogrid) == 0 || len(ogrid[0].cells) == 0 {
		t.Fatal("overflow grid has no rows/cells")
	}
	span := ogrid[0].cells[0]
	if span.cell.text != "Span" {
		t.Fatalf("expected overflow's first cell to be the span cell, got %q", span.cell.text)
	}
	if span.spanHeight <= 0 {
		t.Error("expected span cell to carry a positive spanHeight in the overflow table")
	}

	// The row after the leader must keep its cell in column 1 (the
	// occupied column skipped), not shift left into column 0.
	if len(ogrid) < 2 || len(ogrid[1].cells) == 0 {
		t.Fatal("overflow grid missing the row under the span")
	}
	if got := ogrid[1].cells[0].col; got != 1 {
		t.Errorf("row under span: cell col got %d, want 1 (no column shift)", got)
	}
}

// TestTableRowspanSplitNoContentLoss checks that every body row from the
// source table appears exactly once across page one and the overflow.
func TestTableRowspanSplitNoContentLoss(t *testing.T) {
	tbl := buildRowspanSplitTable()

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 170})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	pageOneRows := len(plan.Blocks[0].Children)

	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	if got := len(overflow.rows); got != 12-pageOneRows {
		t.Errorf("overflow row count: got %d, want %d (12 total - %d on page one)",
			got, 12-pageOneRows, pageOneRows)
	}
	if len(overflow.rows) == 0 || len(overflow.rows[0].cells) == 0 || overflow.rows[0].cells[0].text != "Span" {
		t.Error("expected the span-defining row to be preserved whole in the overflow table")
	}
}

// TestTableRowspanChainedGroupsSplitTogether checks that overlapping spans
// form one transitive group: a span starting inside another span's
// coverage extends the group, and a split inside the extended group must
// move the whole thing to the next page.
func TestTableRowspanChainedGroupsSplitTogether(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})
	// 3 cells per plain row so the table has 3 real columns (column count
	// is derived from the widest row, not from SetColumnWidths).
	addPlainRow3 := func(prefix string) {
		r := tbl.AddRow()
		r.AddCell(prefix+"A", font.Helvetica, 10)
		r.AddCell(prefix+"B", font.Helvetica, 10)
		r.AddCell(prefix+"C", font.Helvetica, 10)
	}
	for i := 1; i <= 4; i++ {
		addPlainRow3(fmt.Sprintf("Row%d", i))
	}
	// Row A covers A,B in col 0; row B covers B,C in col 1. The group is
	// the transitive closure A-C. Every row carries a plain cell so its
	// natural height is 20, matching the plain rows around it.
	rowA := tbl.AddRow()
	rowA.AddCell("A", font.Helvetica, 10).SetRowspan(2)
	rowA.AddCell("A-col1", font.Helvetica, 10)
	rowA.AddCell("A-col2", font.Helvetica, 10)
	rowB := tbl.AddRow()
	rowB.AddCell("B-col1", font.Helvetica, 10).SetRowspan(2)
	rowB.AddCell("B-col2", font.Helvetica, 10)
	rowC := tbl.AddRow()
	rowC.AddCell("C-col0", font.Helvetica, 10)
	rowC.AddCell("C-col2", font.Helvetica, 10)
	for i := 1; i <= 3; i++ {
		addPlainRow3(fmt.Sprintf("Tail%d", i))
	}

	grid := tbl.buildGrid(tbl.resolveColWidths(400))
	if grid[0].height != 20 || grid[4].height != 20 || grid[5].height != 20 || grid[6].height != 20 {
		t.Fatalf("row height assumption broken: got %v, want 20 for every row", []float64{grid[0].height, grid[4].height, grid[5].height, grid[6].height})
	}

	// 4 plain rows (80) + row A (20) fits; row B does not (100+20=120>110),
	// so the split must move up to A, the group's leader.
	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 110})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	if got := len(plan.Blocks[0].Children); got != 4 {
		t.Errorf("page one TR count: got %d, want 4 (whole A-C group pushed to overflow)", got)
	}
	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	if got := len(overflow.rows); got != 6 {
		t.Errorf("overflow row count: got %d, want 6 (A, B, C, and 3 tail rows)", got)
	}
}

// TestTableRowspanTallerThanPageFallsBack pins the fallback for a span group
// taller than a full page: rendering must terminate and still place the group
// rather than dropping it or looping forever.
//
// A multi-row span group that cannot fit even a fresh page falls back to
// per-row splitting inside the group (continuing the spanning cell as a styled
// placeholder), so it makes progress while each placed row still respects the
// page bottom. That is distinct from a span-free first body row that doesn't
// fit, which defers the whole table so its header isn't stranded in the bottom
// margin — see TestTableHeaderNotStrandedWithoutFirstRow.
func TestTableRowspanTallerThanPageFallsBack(t *testing.T) {
	build := func() *Table {
		tbl := NewTable()
		tbl.SetColumnWidths([]float64{100, 100})
		r := tbl.AddRow()
		r.AddCell("Span", font.Helvetica, 10).SetRowspan(120) // taller than a full page
		r.AddCell("Row1", font.Helvetica, 10)
		for i := 2; i <= 120; i++ {
			tbl.AddRow().AddCell(fmt.Sprintf("Row%d", i), font.Helvetica, 10)
		}
		return tbl
	}

	// End-to-end: rendering must terminate (no infinite relocation loop) and
	// emit at least one page carrying the group. Reaching this assertion at
	// all is the anti-loop guarantee.
	r := NewRenderer(300, 200, Margins{Top: 20, Right: 20, Bottom: 20, Left: 20})
	r.Add(build())
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("oversized rowspan group produced no pages — content dropped or pagination looped")
	}

	// Direct contract: given a slot smaller than the oversized first row group
	// the table must still make progress by splitting inside the group, and the
	// rows it does place must stay within the slot it was given.
	small := build().PlanLayout(LayoutArea{Width: 400, Height: 50})
	if small.Status == LayoutNothing {
		t.Errorf("oversized group in a small slot: expected progress, got LayoutNothing")
	}
	if len(small.Blocks) == 0 || len(small.Blocks[0].Children) == 0 {
		t.Error("expected at least one row placed despite the oversized group")
	}
	if small.Consumed > 50 {
		t.Errorf("placed rows overran the slot: consumed %.2f, slot height 50", small.Consumed)
	}
	big := build().PlanLayout(LayoutArea{Width: 400, Height: 1e6})
	if big.Status == LayoutNothing || len(big.Blocks) == 0 || len(big.Blocks[0].Children) == 0 {
		t.Error("given ample height the oversized group must be placed, not deferred")
	}
}

func TestTableCellSpacingWithColspan(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})
	tbl.SetCellSpacing(5, 5)

	row1 := tbl.AddRow()
	c := row1.AddCell("Spanning two columns", font.Helvetica, 10)
	c.SetColspan(2)
	row1.AddCell("Normal", font.Helvetica, 10)

	row2 := tbl.AddRow()
	row2.AddCell("A", font.Helvetica, 10)
	row2.AddCell("B", font.Helvetica, 10)
	row2.AddCell("C", font.Helvetica, 10)

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}
	// Spacing adds gaps — consumed height should include spacing.
	if plan.Consumed < 30 {
		t.Errorf("expected consumed > 30 with cell spacing, got %f", plan.Consumed)
	}
}

// TestResolveBordersColspanSegments verifies that a colspan cell's bottom
// edge is resolved per grid segment against each row-below neighbor, not
// against only the first one (issue: segment-granular border-collapse
// conflict resolution).
func TestResolveBordersColspanSegments(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})
	tbl.SetBorderCollapse(true)

	r0 := tbl.AddRow()
	r0.AddCell("Total", font.Helvetica, 10).
		SetColspan(3).
		SetBorders(CellBorders{Bottom: SolidBorder(1, ColorBlack)})

	r1 := tbl.AddRow()
	r1.AddCell("A", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 0}}) // none
	r1.AddCell("B", font.Helvetica, 10).
		SetBorders(CellBorders{Top: SolidBorder(2, ColorBlack)})
	r1.AddCell("C", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 0}}) // none

	colWidths := tbl.resolveColWidths(400)
	grid := tbl.buildGrid(colWidths)
	resolveBorders(grid, len(colWidths))

	// The winner of each segment is written to the FAR (below) cell's
	// segTop — the near (spanning) cell's own segBottom is zeroed, per the
	// documented "later element wins, near clears" convention.
	if len(grid[1].cells) != 3 {
		t.Fatalf("expected 3 row-1 cells, got %d", len(grid[1].cells))
	}
	want := []float64{1, 2, 1}
	for i, w := range want {
		got := grid[1].cells[i].segTop[0]
		if got.Width != w {
			t.Errorf("segment %d: got width %v, want %v", i, got.Width, w)
		}
	}
}

// TestResolveBordersRowspanSegments is the symmetric case: a rowspan
// cell's right edge resolved per row segment against each column-right
// neighbor.
func TestResolveBordersRowspanSegments(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	tbl.SetBorderCollapse(true)

	r0 := tbl.AddRow()
	r0.AddCell("Span", font.Helvetica, 10).
		SetRowspan(3).
		SetBorders(CellBorders{Right: SolidBorder(1, ColorBlack)})
	r0.AddCell("A", font.Helvetica, 10).
		SetBorders(CellBorders{Left: Border{Width: 0}}) // none

	r1 := tbl.AddRow()
	r1.AddCell("B", font.Helvetica, 10).
		SetBorders(CellBorders{Left: SolidBorder(2, ColorBlack)})

	r2 := tbl.AddRow()
	r2.AddCell("C", font.Helvetica, 10).
		SetBorders(CellBorders{Left: Border{Width: 0}}) // none

	colWidths := tbl.resolveColWidths(400)
	grid := tbl.buildGrid(colWidths)
	resolveBorders(grid, len(colWidths))

	// The winner of each segment is written to the FAR (right) cell's
	// segLeft — the near (spanning) cell's own segRight is zeroed.
	want := []float64{1, 2, 1}
	for i, w := range want {
		// Row 0 has two cells (spanner, A); rows 1-2 have only the
		// right-column cell (B, C) since column 0 is occupied by the
		// rowspan and buildGrid does not emit filler cells for it.
		cellIdx := 0
		if i == 0 {
			cellIdx = 1
		}
		got := grid[i].cells[cellIdx].segLeft[0]
		if got.Width != w {
			t.Errorf("segment %d: got width %v, want %v", i, got.Width, w)
		}
	}
}

// TestResolveBordersHiddenSegment verifies that a BorderHidden declaration
// on one neighbor suppresses only its own segment, leaving sibling
// segments along the same spanned edge unaffected.
func TestResolveBordersHiddenSegment(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})
	tbl.SetBorderCollapse(true)

	r0 := tbl.AddRow()
	r0.AddCell("Total", font.Helvetica, 10).
		SetColspan(3).
		SetBorders(CellBorders{Bottom: SolidBorder(1, ColorBlack)})

	r1 := tbl.AddRow()
	r1.AddCell("A", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 3, Style: BorderHidden}})
	r1.AddCell("B", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 0}})
	r1.AddCell("C", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 0}})

	colWidths := tbl.resolveColWidths(400)
	grid := tbl.buildGrid(colWidths)
	resolveBorders(grid, len(colWidths))

	if got := grid[1].cells[0].segTop[0]; got != (Border{}) {
		t.Errorf("segment 0 should be suppressed by BorderHidden, got %+v", got)
	}
	if got := grid[1].cells[1].segTop[0]; got.Width != 1 {
		t.Errorf("segment 1 unaffected: got width %v, want 1", got.Width)
	}
	if got := grid[1].cells[2].segTop[0]; got.Width != 1 {
		t.Errorf("segment 2 unaffected: got width %v, want 1", got.Width)
	}
}

// TestTableColspanMixedBordersDraw is an end-to-end pin: PlanLayout for a
// colspan "Total" row over columns with mixed borders must not drop any
// segment when rendered.
func TestTableColspanMixedBordersDraw(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})
	tbl.SetBorderCollapse(true)

	r0 := tbl.AddRow()
	r0.AddCell("Total", font.Helvetica, 10).
		SetColspan(3).
		SetBorders(CellBorders{Bottom: SolidBorder(1, ColorBlack)})

	r1 := tbl.AddRow()
	r1.AddCell("A", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 0}})
	r1.AddCell("B", font.Helvetica, 10).
		SetBorders(CellBorders{Top: SolidBorder(2, ColorBlack)})
	r1.AddCell("C", font.Helvetica, 10).
		SetBorders(CellBorders{Top: Border{Width: 0}})

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected output")
	}

	colWidths := tbl.resolveColWidths(400)
	grid := tbl.buildGrid(colWidths)
	resolveBorders(grid, len(colWidths))
	want := []float64{1, 2, 1}
	for i, w := range want {
		got := grid[1].cells[i].segTop[0]
		if got.Width != w {
			t.Errorf("segment %d dropped or wrong: got width %v, want %v", i, got.Width, w)
		}
	}
}

// TestTableOversizeRowspanGroup covers a rowspan group whose leader is the
// first body row and whose total height is far more than any single page
// can hold. PlanLayout must fall back to per-row splitting inside the
// group instead of emitting the whole group (or nothing useful) on one
// page: nothing may be placed past the bottom margin, and the overflow
// chain must terminate.
func TestTableOversizeRowspanGroup(t *testing.T) {
	const pageH = 100.0 // area height per page

	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	r := tbl.AddRow()
	r.AddCell("Span", font.Helvetica, 10).SetRowspan(10)
	r.AddCell("Row1", font.Helvetica, 10)
	for i := 2; i <= 10; i++ {
		// Column 0 is occupied by the rowspan; only add the column-1
		// cell, matching real rowspan usage (buildRowspanSplitTable does
		// the same).
		tbl.AddRow().AddCell(fmt.Sprintf("Row%d", i), font.Helvetica, 10)
	}

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: pageH})
	if plan.Status != LayoutPartial || plan.Overflow == nil {
		t.Fatalf("expected LayoutPartial with Overflow (group ~2x page height must split), got status=%v overflow=%v", plan.Status, plan.Overflow)
	}

	// Every emitted row block (including nested Table wrapper's children)
	// must stay within the page bounds.
	var walk func(blocks []PlacedBlock)
	walk = func(blocks []PlacedBlock) {
		for _, b := range blocks {
			if b.Y+b.Height > pageH+0.01 {
				t.Errorf("block %q at Y=%v H=%v exceeds page height %v", b.Tag, b.Y, b.Height, pageH)
			}
			walk(b.Children)
		}
	}
	walk(plan.Blocks)

	// Chase the overflow chain until it terminates (LayoutFull/Nothing),
	// bounded so non-termination fails the test instead of hanging.
	cur := plan.Overflow
	for i := 0; i < 10; i++ {
		ot, ok := cur.(*Table)
		if !ok {
			t.Fatalf("overflow is not *Table: %T", cur)
		}
		p := ot.PlanLayout(LayoutArea{Width: 400, Height: pageH})
		walk(p.Blocks)
		if p.Status != LayoutPartial || p.Overflow == nil {
			return // terminated
		}
		cur = p.Overflow
	}
	t.Fatal("overflow chain did not terminate within 10 pages")
}

// TestTableOversizeRowspanGroupContinuesSpan verifies that when the split
// lands strictly inside an oversize group, the overflow table's first row
// re-carries a continuation of the spanning cell (reduced rowspan) rather
// than losing the column entirely — the row after it must keep its cell in
// column 1, not shift left into column 0 (mirrors
// TestTableRowspanSplitPushesGroupToNextPage's column-shift check for the
// #362 keep-together path).
func TestTableOversizeRowspanGroupContinuesSpan(t *testing.T) {
	const pageH = 100.0

	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	r := tbl.AddRow()
	r.AddCell("Span", font.Helvetica, 10).SetRowspan(10)
	r.AddCell("Row1", font.Helvetica, 10)
	for i := 2; i <= 10; i++ {
		tbl.AddRow().AddCell(fmt.Sprintf("Row%d", i), font.Helvetica, 10)
	}

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: pageH})
	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	og := overflow.buildGrid(overflow.resolveColWidths(400))
	if len(og) == 0 || len(og[0].cells) == 0 {
		t.Fatal("overflow grid has no rows/cells")
	}
	first := og[0].cells[0]
	if first.col != 0 {
		t.Errorf("expected the continuation span cell at column 0, got %d", first.col)
	}
	if first.rowSpanCount <= 0 {
		t.Errorf("expected a positive continuation rowspan, got %d", first.rowSpanCount)
	}
	if len(og) < 2 || len(og[1].cells) == 0 {
		t.Fatal("overflow grid missing the row under the continued span")
	}
	if got := og[1].cells[0].col; got != 1 {
		t.Errorf("row under continued span: cell col got %d, want 1 (no column shift)", got)
	}
}

// TestTableOversizeGroupWithHeaderFooter covers the oversize fallback in
// the presence of header/footer rows: the header must still repeat, the
// footer must still land at the end, and the fallback must still fire.
func TestTableOversizeGroupWithHeaderFooter(t *testing.T) {
	const pageH = 100.0

	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100})
	hdr := tbl.AddRow()
	hdr.isHeader = true
	hdr.AddCell("H1", font.Helvetica, 10)
	hdr.AddCell("H2", font.Helvetica, 10)

	r := tbl.AddRow()
	r.AddCell("Span", font.Helvetica, 10).SetRowspan(10)
	r.AddCell("Row1", font.Helvetica, 10)
	for i := 2; i <= 10; i++ {
		tbl.AddRow().AddCell(fmt.Sprintf("Row%d", i), font.Helvetica, 10)
	}

	ftr := tbl.AddRow()
	ftr.isFooter = true
	ftr.AddCell("F1", font.Helvetica, 10)
	ftr.AddCell("F2", font.Helvetica, 10)

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: pageH})
	if plan.Status != LayoutPartial || plan.Overflow == nil {
		t.Fatalf("expected LayoutPartial with Overflow, got status=%v overflow=%v", plan.Status, plan.Overflow)
	}
	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected *Table overflow, got %T", plan.Overflow)
	}
	if len(overflow.rows) == 0 || !overflow.rows[0].isHeader {
		t.Error("expected the overflow table to repeat the header row")
	}
	if last := overflow.rows[len(overflow.rows)-1]; !last.isFooter {
		t.Error("expected the overflow table's last row to be the footer")
	}
}
