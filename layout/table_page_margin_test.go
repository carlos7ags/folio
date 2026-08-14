// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// pageContentMinY scans a page content stream for the lowest y-coordinate
// touched by text (Td) and vector (re/m/l) operators — how far the page's
// content reaches toward the bottom, in absolute PDF page coordinates (origin
// at the page's bottom-left). Returns found=false for an empty page.
func pageContentMinY(stream string) (minY float64, found bool) {
	minY = 1e18
	consider := func(y float64) {
		if y < minY {
			minY = y
		}
		found = true
	}
	for _, line := range strings.Split(stream, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch f[len(f)-1] {
		case "Td", "m", "l":
			var y float64
			if _, err := fmt.Sscanf(f[len(f)-2], "%g", &y); err == nil {
				consider(y)
			}
		case "re": // x y w h re — y is the rect's bottom edge
			if len(f) >= 5 {
				var y float64
				if _, err := fmt.Sscanf(f[len(f)-4], "%g", &y); err == nil {
					consider(y)
				}
			}
		}
	}
	return
}

// TestTableRowDoesNotOverflowBottomMargin is the regression test for a table
// header + first body row being crammed into the bottom margin when the table
// begins near the bottom of a page. The row-placement path (Table.PlanLayout)
// placed the header rows and the first body row unconditionally — the split
// check was gated on `i > headerRowCount`, so it never fired for the first
// body row — so a table given only a little room at the page bottom emitted
// its header + first row past pageHeight-bottomMargin instead of moving them
// together to the next page. A spacer Div sized to leave a gap smaller than
// header+row reproduces the exact placement.
func TestTableRowDoesNotOverflowBottomMargin(t *testing.T) {
	const (
		pageW, pageH = 595.28, 841.89
		top, bottom  = 28.35, 28.35 // 10mm A4 margins
		tol          = 2.0          // text baselines sit a few pt above the glyph bottom
		usable       = pageH - top - bottom
	)

	// Leave only ~18pt at the bottom of page 0 — less than the table's
	// header + first row (~28pt) — so the header+row must break to page 2.
	for _, gap := range []float64{18.0, 40.0} {
		t.Run(fmt.Sprintf("gap=%.0f", gap), func(t *testing.T) {
			r := NewRenderer(pageW, pageH, Margins{Top: top, Right: 28.35, Bottom: bottom, Left: 28.35})
			spacer := NewDiv()
			spacer.SetHeightUnit(Pt(usable - gap))
			spacer.Add(NewParagraph("spacer", font.Helvetica, 10))
			r.Add(spacer)

			tbl := NewTable().SetAutoColumnWidths()
			tbl.AddHeaderRow().AddCell("Proceso section header", font.HelveticaBold, 10)
			for i := 0; i < 15; i++ {
				tbl.AddRow().AddCell(fmt.Sprintf("Ciudad: BOGOTA row %d", i), font.Helvetica, 10)
			}
			r.Add(tbl)

			pages := r.Render()
			if len(pages) < 2 {
				t.Fatalf("expected the table to span multiple pages, got %d", len(pages))
			}
			for i, pg := range pages {
				minY, ok := pageContentMinY(string(pg.Stream.Bytes()))
				if !ok {
					continue
				}
				if minY < bottom-tol {
					t.Errorf("page %d: content reaches y=%.1f, below the bottom margin (%.1f) — a table row/header overran the margin", i, minY, bottom)
				}
			}
		})
	}
}

// TestTableHeaderNotStrandedWithoutFirstRow pins the orphan guard at the unit
// level: when the available height fits the header but not the header plus its
// first body row, the table defers the whole table (LayoutNothing) rather than
// emitting the header alone at the page bottom (which would both strand the
// header and, since the overflow table re-adds the header, duplicate it).
func TestTableHeaderNotStrandedWithoutFirstRow(t *testing.T) {
	build := func() *Table {
		tbl := NewTable().SetAutoColumnWidths()
		tbl.AddHeaderRow().AddCell("Header", font.HelveticaBold, 10)
		for i := 0; i < 10; i++ {
			tbl.AddRow().AddCell(fmt.Sprintf("row %d", i), font.Helvetica, 10)
		}
		return tbl
	}

	// Measure header height and header+first-row height.
	headerOnly := build().PlanLayout(LayoutArea{Width: 300, Height: 1e6})
	_ = headerOnly // full layout; used only to confirm the table lays out at all
	if headerOnly.Status == LayoutNothing {
		t.Fatal("table failed to lay out at unlimited height")
	}

	// A height that fits the header (~13pt) but not header+first row (~26pt):
	// the table must defer entirely, placing nothing, not strand the header.
	plan := build().PlanLayout(LayoutArea{Width: 300, Height: 18})
	if plan.Status != LayoutNothing {
		t.Fatalf("expected LayoutNothing (defer header+row together), got status %d with %d blocks", plan.Status, len(plan.Blocks))
	}

	// A height that fits header+first row must place them (not defer).
	fits := build().PlanLayout(LayoutArea{Width: 300, Height: 40})
	if fits.Status == LayoutNothing {
		t.Error("header + first row fit in the height but the table deferred anyway")
	}
}
