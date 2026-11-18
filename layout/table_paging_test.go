// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// pagingBorders returns cell borders that vary with i, so border-collapse
// picks different winners on neighbouring edges.
func pagingBorders(i int) CellBorders {
	colors := []Color{RGB(1, 0, 0), RGB(0, 0, 1), RGB(0, 0.5, 0)}
	return CellBorders{
		Top:    SolidBorder(float64(i%3)+0.5, colors[i%3]),
		Bottom: SolidBorder(float64((i+1)%3)+0.5, colors[(i+1)%3]),
		Left:   DashedBorder(float64(i%2)+0.5, colors[(i+2)%3]),
		Right:  SolidBorder(0.5, colors[i%3]),
	}
}

// pagingTestTables returns multi-page tables covering rowspan groups, a
// body rowspan into the footer, border-collapse, border-spacing and
// tables without header or footer rows.
func pagingTestTables() map[string]*Table {
	text := func(i int) string { return strings.Repeat("word ", 1+i%7) + fmt.Sprint(i) }
	build := func(rows int, collapse, header, footer bool, row func(t *Table, i int)) *Table {
		t := NewTable()
		t.SetColumnWidths([]float64{120, 160, 120})
		t.SetBorderCollapse(collapse)
		if header {
			h := t.AddHeaderRow()
			for c := range 3 {
				h.AddCell("Head", font.HelveticaBold, 10).SetBorders(pagingBorders(c + 4))
			}
		}
		for i := range rows {
			row(t, i)
		}
		if footer {
			f := t.AddFooterRow()
			for c := range 3 {
				f.AddCell("Foot", font.HelveticaBold, 10).SetBorders(pagingBorders(c + 2))
			}
		}
		return t
	}
	plain := func(t *Table, i int) {
		r := t.AddRow()
		r.AddCell(text(i), font.Helvetica, 9).SetBorders(pagingBorders(i))
		r.AddCellElement(NewParagraph(text(i*3), font.Helvetica, 9)).SetBorders(pagingBorders(i + 1))
		r.AddCell(text(i+2), font.Helvetica, 9).SetBorders(pagingBorders(i + 2))
	}
	groups := func(t *Table, i int) {
		r := t.AddRow()
		switch i % 5 {
		case 0:
			r.AddCellElement(NewParagraph("span "+text(i*5), font.Helvetica, 9)).SetRowspan(3).SetBorders(pagingBorders(i))
		case 1, 2:
		case 3:
			r.AddCell(text(i), font.Helvetica, 9).SetColspan(2).SetBorders(pagingBorders(i))
			r.AddCell("x", font.Helvetica, 9).SetBorders(pagingBorders(i + 1))
			return
		default:
			r.AddCell(text(i), font.Helvetica, 9).SetBorders(pagingBorders(i + 2))
		}
		r.AddCell(text(i+1), font.Helvetica, 9).SetBorders(pagingBorders(i + 1))
		r.AddCell(text(i+2), font.Helvetica, 9).SetBorders(pagingBorders(i))
	}
	return map[string]*Table{
		// An oversize rowspan group split mid-span, holding a second
		// spanning cell that starts below the group's first row: only
		// the first row caps its span, so that cell is drawn whole.
		"collapse oversize group with two spans": build(90, true, false, false, func(t *Table, i int) {
			r := t.AddRow()
			switch {
			case i == 0:
				r.AddCell("tall", font.Helvetica, 9).SetRowspan(40).SetBorders(pagingBorders(i))
				r.AddCell(text(i), font.Helvetica, 9).SetBorders(pagingBorders(i + 1))
			case i == 1:
				r.AddCell("inner", font.Helvetica, 9).SetRowspan(15).SetBorders(pagingBorders(i + 1))
			case i < 16:
			case i < 40:
				r.AddCell(text(i), font.Helvetica, 9).SetBorders(pagingBorders(i))
			default:
				r.AddCell(text(i), font.Helvetica, 9).SetBorders(pagingBorders(i))
				r.AddCell(text(i+1), font.Helvetica, 9).SetBorders(pagingBorders(i + 1))
			}
			r.AddCell(text(i+2), font.Helvetica, 9).SetBorders(pagingBorders(i + 2))
		}),
		"collapse header footer":  build(90, true, true, true, plain),
		"collapse rowspan groups": build(100, true, true, true, groups),
		"collapse no footer":      build(100, true, true, false, groups),
		"spacing no header":       build(90, false, false, true, plain),
		"body rowspan into footer": build(90, true, true, true, func(t *Table, i int) {
			plain(t, i)
			if i == 89 {
				t.rows[len(t.rows)-1].cells[0].SetRowspan(2)
			}
		}),
	}
}

// TestTableMasterMatchesRebuiltGrid checks the shared master grid against
// the grid a continuation table used to rebuild for itself on every page:
// the same rows, heights, rowspan groups and span geometry, and, on the
// page grid built for each split, the same resolved borders on every row
// the page draws.
func TestTableMasterMatchesRebuiltGrid(t *testing.T) {
	for name, tbl := range pagingTestTables() {
		t.Run(name, func(t *testing.T) {
			var el Element = tbl
			pages := 0
			for el != nil {
				tb := el.(*Table)
				plan := tb.PlanLayout(LayoutArea{Width: 400, Height: 250})
				if plan.Status == LayoutNothing {
					t.Fatalf("page %d placed nothing", pages)
				}
				comparePageWithRebuild(t, tb, plan, pages)
				pages++
				el = plan.Overflow
			}
			if pages < 4 {
				t.Fatalf("table spans %d pages, want a long multi-page table", pages)
			}
		})
	}
}

func comparePageWithRebuild(t *testing.T, tb *Table, plan LayoutPlan, page int) {
	t.Helper()
	colWidths := tb.resolveColWidths(400)
	m, start := tb.pagingMaster(400, colWidths)
	if m == nil {
		t.Fatalf("page %d: table has no master grid", page)
	}
	full := tb.buildGrid(colWidths)
	starts := spanGroupStarts(full)
	view := masterPageRows{t: tb, m: m, start: start}
	if view.len() != len(full) {
		t.Fatalf("page %d: master view has %d rows, rebuilt grid %d", page, view.len(), len(full))
	}
	for i := range full {
		got, want := view.row(i), &full[i]
		if got.height != want.height || got.isHeader != want.isHeader || got.isFooter != want.isFooter {
			t.Fatalf("page %d row %d: got height %v header %v footer %v, want %v %v %v",
				page, i, got.height, got.isHeader, got.isFooter, want.height, want.isHeader, want.isFooter)
		}
		if view.groupStart(i) != starts[i] {
			t.Fatalf("page %d row %d: group start %d, want %d", page, i, view.groupStart(i), starts[i])
		}
		if len(got.cells) != len(want.cells) {
			t.Fatalf("page %d row %d: %d cells, want %d", page, i, len(got.cells), len(want.cells))
		}
		for c := range want.cells {
			g, w := got.cells[c], want.cells[c]
			if g.cell != w.cell || g.col != w.col || g.spanWidth != w.spanWidth ||
				g.spanHeight != w.spanHeight || g.colSpanCount != w.colSpanCount || g.rowSpanCount != w.rowSpanCount {
				t.Fatalf("page %d row %d cell %d: geometry differs from the rebuilt grid", page, i, c)
			}
		}
	}

	headers, footers := 0, 0
	for headers < len(full) && full[headers].isHeader {
		headers++
	}
	for footers < len(full)-headers && full[len(full)-1-footers].isFooter {
		footers++
	}
	bodyEnd := len(full) - footers
	rowBlocks := len(plan.Blocks[0].Children)
	splitIdx := rowBlocks - footers // header rows plus placed body rows
	pageGrid, footerIdx := m.pageGridFor(tb, start, splitIdx, bodyEnd, footers > 0)
	if !tb.borderCollapse {
		return
	}
	resolveBorders(full, len(colWidths))
	resolveBorders(pageGrid, len(colWidths))
	drawn := func(i, pi int) {
		for c := range full[i].cells {
			g, w := pageGrid[pi].cells[c], full[i].cells[c]
			if !reflect.DeepEqual(g.resolved, w.resolved) || !reflect.DeepEqual(g.segTop, w.segTop) ||
				!reflect.DeepEqual(g.segBottom, w.segBottom) || !reflect.DeepEqual(g.segLeft, w.segLeft) ||
				!reflect.DeepEqual(g.segRight, w.segRight) {
				t.Fatalf("page %d row %d cell %d: resolved borders differ from the rebuilt grid", page, i, c)
			}
		}
	}
	for i := range splitIdx {
		drawn(i, i)
	}
	for i := bodyEnd; i < len(full); i++ {
		drawn(i, footerIdx(i))
	}
}

// TestTableContinuationSharesMaster checks that the continuation tables
// of a long table view the original's master grid from successive body
// rows, instead of building and measuring a grid of their own.
func TestTableContinuationSharesMaster(t *testing.T) {
	tbl := pagingTestTables()["collapse rowspan groups"]
	var el Element = tbl
	var paging *tablePaging
	lastStart := -1
	for el != nil {
		tb := el.(*Table)
		plan := tb.PlanLayout(LayoutArea{Width: 400, Height: 250})
		if paging == nil {
			paging = tb.paging
		}
		if tb.paging != paging || len(paging.masters) != 1 {
			t.Fatal("continuation built its own master grid")
		}
		if tb.masterStart <= lastStart {
			t.Fatalf("masterStart %d did not advance past %d", tb.masterStart, lastStart)
		}
		lastStart = tb.masterStart
		el = plan.Overflow
	}
}

// TestTableMasterCarriedOccupancyFallsBack covers a colspan that skips a
// column held by a rowspan from above: buildGridGeometry then carries the
// column's occupancy one row past the rowspan, which rebuilding a
// continuation's grid drops at its first row. A shared master would keep
// it and shift that row's cells on a later page, so the table must be
// paginated from rebuilt grids, as before.
func TestTableMasterCarriedOccupancyFallsBack(t *testing.T) {
	tbl := NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100})
	for range 14 {
		r := tbl.AddRow()
		r.AddCell("a", font.Helvetica, 10)
		r.AddCell("b", font.Helvetica, 10).SetRowspan(2)
		r.AddCell("c", font.Helvetica, 10)
		r = tbl.AddRow()
		r.AddCell("d", font.Helvetica, 10).SetColspan(2)
		r.AddCell("e", font.Helvetica, 10)
		r = tbl.AddRow()
		r.AddCell("x", font.Helvetica, 10)
		r.AddCell("y", font.Helvetica, 10)
		r.AddCell("z", font.Helvetica, 10)
	}
	colWidths := tbl.resolveColWidths(300)
	if m, _ := tbl.pagingMaster(300, colWidths); m != nil {
		t.Fatal("table carrying occupancy into a rowspan group got a shared master grid")
	}
}
