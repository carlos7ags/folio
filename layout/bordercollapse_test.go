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
	a := grid[0].cells[0].cell
	if a.borders.Right.Width != 0 {
		t.Error("cell A should have no right border after collapse")
	}
	if a.borders.Bottom.Width != 0 {
		t.Error("cell A should have no bottom border after collapse")
	}

	// Row 0, Cell 1 (B): should have right border (last col), no bottom.
	b := grid[0].cells[1].cell
	if b.borders.Right.Width == 0 {
		t.Error("cell B should keep right border (last column)")
	}
	if b.borders.Bottom.Width != 0 {
		t.Error("cell B should have no bottom border after collapse")
	}

	// Row 1, Cell 0 (C): should have no right border, keeps bottom (last row).
	c := grid[1].cells[0].cell
	if c.borders.Right.Width != 0 {
		t.Error("cell C should have no right border after collapse")
	}
	if c.borders.Bottom.Width == 0 {
		t.Error("cell C should keep bottom border (last row)")
	}

	// Row 1, Cell 1 (D): keeps both right and bottom (last col + last row).
	d := grid[1].cells[1].cell
	if d.borders.Right.Width == 0 {
		t.Error("cell D should keep right border (last column)")
	}
	if d.borders.Bottom.Width == 0 {
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
