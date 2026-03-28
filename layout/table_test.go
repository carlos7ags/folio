// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
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
