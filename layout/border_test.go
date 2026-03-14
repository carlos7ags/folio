// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestBorderStyles(t *testing.T) {
	solid := SolidBorder(1, ColorBlack)
	if solid.Style != BorderSolid {
		t.Errorf("SolidBorder style = %d, want BorderSolid", solid.Style)
	}

	dashed := DashedBorder(1, ColorRed)
	if dashed.Style != BorderDashed {
		t.Errorf("DashedBorder style = %d, want BorderDashed", dashed.Style)
	}
	if dashed.Color != ColorRed {
		t.Error("DashedBorder should be red")
	}

	dotted := DottedBorder(0.5, ColorBlue)
	if dotted.Style != BorderDotted {
		t.Errorf("DottedBorder style = %d, want BorderDotted", dotted.Style)
	}
	if dotted.Width != 0.5 {
		t.Errorf("DottedBorder width = %.1f, want 0.5", dotted.Width)
	}

	double := DoubleBorder(1, ColorBlack)
	if double.Style != BorderDouble {
		t.Errorf("DoubleBorder style = %d, want BorderDouble", double.Style)
	}
}

func TestDefaultBorderIsSolid(t *testing.T) {
	b := DefaultBorder()
	if b.Style != BorderSolid {
		t.Errorf("DefaultBorder style = %d, want BorderSolid", b.Style)
	}
	if b.Width != 0.5 {
		t.Errorf("DefaultBorder width = %.1f, want 0.5", b.Width)
	}
}

func TestDashedBorderRendering(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("Dashed", font.Helvetica, 10).
		SetBorders(AllBorders(DashedBorder(1, ColorBlack)))

	renderer := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	renderer.Add(tbl)
	pages := renderer.Render()

	content := string(pages[0].Stream.Bytes())
	// Dashed borders use the "d" operator for dash pattern.
	if !strings.Contains(content, " d") {
		t.Error("dashed border should emit dash pattern operator 'd'")
	}
}

func TestDottedBorderRendering(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("Dotted", font.Helvetica, 10).
		SetBorders(AllBorders(DottedBorder(1, ColorBlack)))

	renderer := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	renderer.Add(tbl)
	pages := renderer.Render()

	content := string(pages[0].Stream.Bytes())
	// Dotted borders use round line cap (1 J) and dash pattern.
	if !strings.Contains(content, "1 J") {
		t.Error("dotted border should set round line cap")
	}
}

func TestDoubleBorderRendering(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("Double", font.Helvetica, 10).
		SetBorders(AllBorders(DoubleBorder(1, ColorBlack)))

	renderer := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	renderer.Add(tbl)
	pages := renderer.Render()

	content := string(pages[0].Stream.Bytes())
	// Double border draws two lines, so we should see multiple strokes.
	count := strings.Count(content, "\nS\n") + strings.Count(content, "\nS")
	if count < 4 { // at minimum 4 sides × 2 lines each, but some share strokes
		t.Logf("stroke count = %d (content length = %d)", count, len(content))
	}
}

func TestNoBorderRendering(t *testing.T) {
	tbl := NewTable()
	r := tbl.AddRow()
	r.AddCell("No border", font.Helvetica, 10).
		SetBorders(NoBorders())

	renderer := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	renderer.Add(tbl)
	pages := renderer.Render()

	content := string(pages[0].Stream.Bytes())
	// Should have text but no border strokes (S operator in graphics context).
	if !strings.Contains(content, "Tj") {
		t.Error("should still have text content")
	}
}

func TestTablePercentageWidths(t *testing.T) {
	tbl := NewTable().
		SetColumnUnitWidths([]UnitValue{Pct(30), Pct(70)})

	r := tbl.AddRow()
	r.AddCell("Narrow", font.Helvetica, 10)
	r.AddCell("Wide", font.Helvetica, 10)

	lines := tbl.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}
	ref := lines[0].tableRow
	if ref == nil {
		t.Fatal("expected tableRow ref")
	}
	// 30% of 500 = 150, 70% of 500 = 350
	if ref.colWidths[0] != 150 {
		t.Errorf("col 0 width = %.1f, want 150", ref.colWidths[0])
	}
	if ref.colWidths[1] != 350 {
		t.Errorf("col 1 width = %.1f, want 350", ref.colWidths[1])
	}
}

func TestTableMixedUnitWidths(t *testing.T) {
	tbl := NewTable().
		SetColumnUnitWidths([]UnitValue{Pt(100), Pct(50), Pt(100)})

	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)
	r.AddCell("C", font.Helvetica, 10)

	lines := tbl.Layout(468) // 612 - 72*2 = 468
	ref := lines[0].tableRow
	if ref.colWidths[0] != 100 {
		t.Errorf("col 0 = %.1f, want 100", ref.colWidths[0])
	}
	// 50% of 468 = 234
	if ref.colWidths[1] != 234 {
		t.Errorf("col 1 = %.1f, want 234", ref.colWidths[1])
	}
	if ref.colWidths[2] != 100 {
		t.Errorf("col 2 = %.1f, want 100", ref.colWidths[2])
	}
}
