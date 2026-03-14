// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestColumnsBasic(t *testing.T) {
	cols := NewColumns(2)
	cols.Add(0, NewParagraph("Left column text", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Right column text", font.Helvetica, 12))

	lines := cols.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}
	if lines[0].columnsRef == nil {
		t.Fatal("expected columnsRef on line")
	}
	if len(lines[0].columnsRef.colLines) != 2 {
		t.Errorf("expected 2 column entries, got %d", len(lines[0].columnsRef.colLines))
	}
}

func TestColumnsEqualWidths(t *testing.T) {
	cols := NewColumns(3).SetGap(10)
	cols.Add(0, NewParagraph("A", font.Helvetica, 12))
	cols.Add(1, NewParagraph("B", font.Helvetica, 12))
	cols.Add(2, NewParagraph("C", font.Helvetica, 12))

	lines := cols.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}

	// Total gap = 2 * 10 = 20. Content width = 380. Each col = 380/3 ≈ 126.67
	cls := lines[0].columnsRef.colLines
	expectedWidth := (400.0 - 20.0) / 3.0
	for i, cl := range cls {
		if abs(cl.width-expectedWidth) > 0.1 {
			t.Errorf("col %d: expected width ~%.1f, got %.1f", i, expectedWidth, cl.width)
		}
	}
}

func TestColumnsXOffsets(t *testing.T) {
	cols := NewColumns(2).SetGap(20)
	cols.Add(0, NewParagraph("Left", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Right", font.Helvetica, 12))

	lines := cols.Layout(400)
	cls := lines[0].columnsRef.colLines

	// Col 0 should start at x=0
	if cls[0].xOffset != 0 {
		t.Errorf("col 0 xOffset: expected 0, got %f", cls[0].xOffset)
	}
	// Col 1 should start at colWidth + gap
	expectedX := (400.0-20.0)/2.0 + 20.0
	if abs(cls[1].xOffset-expectedX) > 0.1 {
		t.Errorf("col 1 xOffset: expected %.1f, got %.1f", expectedX, cls[1].xOffset)
	}
}

func TestColumnsCustomWidths(t *testing.T) {
	cols := NewColumns(2).SetGap(10).SetWidths([]float64{0.3, 0.7})
	cols.Add(0, NewParagraph("Narrow", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Wide", font.Helvetica, 12))

	lines := cols.Layout(400)
	cls := lines[0].columnsRef.colLines

	contentWidth := 400.0 - 10.0
	if abs(cls[0].width-contentWidth*0.3) > 0.1 {
		t.Errorf("col 0 width: expected %.1f, got %.1f", contentWidth*0.3, cls[0].width)
	}
	if abs(cls[1].width-contentWidth*0.7) > 0.1 {
		t.Errorf("col 1 width: expected %.1f, got %.1f", contentWidth*0.7, cls[1].width)
	}
}

func TestColumnsUnevenContent(t *testing.T) {
	cols := NewColumns(2)
	// Left column has more lines than right.
	cols.Add(0, NewParagraph("Short text on the left side that wraps to multiple lines when narrow", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Short", font.Helvetica, 12))

	lines := cols.Layout(300)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// Last line should have nil for the right column.
	last := lines[len(lines)-1].columnsRef.colLines
	if last[1].line != nil {
		t.Error("right column should be nil in last row (exhausted)")
	}
}

func TestColumnsEmpty(t *testing.T) {
	cols := NewColumns(2)
	lines := cols.Layout(400)
	if len(lines) != 0 {
		t.Errorf("empty columns should produce 0 lines, got %d", len(lines))
	}
}

func TestColumnsRendererBasic(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	cols := NewColumns(2)
	cols.Add(0, NewParagraph("Left column", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Right column", font.Helvetica, 12))

	r.Add(cols)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Fonts) == 0 {
		t.Error("expected at least 1 font registered")
	}
}

func TestColumnsImplementsElement(t *testing.T) {
	var _ Element = NewColumns(2)
}

func TestColumnsZeroPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewColumns(0) should panic")
		}
	}()
	NewColumns(0)
}

func TestColumnsNegativeGapClamped(t *testing.T) {
	cols := NewColumns(2).SetGap(-10)
	if cols.gap != 0 {
		t.Errorf("negative gap should be clamped to 0, got %f", cols.gap)
	}
}

func TestColumnsWithTable(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	cols := NewColumns(2)
	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell("A", font.Helvetica, 10)
	row.AddCell("B", font.Helvetica, 10)
	cols.Add(0, tbl)
	cols.Add(1, NewParagraph("Right side", font.Helvetica, 12))

	r.Add(cols)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	stream := string(pages[0].Stream.Bytes())
	if !stringContains(stream, "Right") {
		t.Error("stream should contain text from right column")
	}
}

func TestColumnsAlignCenter(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	cols := NewColumns(2)
	p := NewParagraph("Hi", font.Helvetica, 12).SetAlign(AlignCenter)
	cols.Add(0, p)
	cols.Add(1, NewParagraph("Right", font.Helvetica, 12))

	r.Add(cols)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// Verify it renders without error. Exact positioning validated by
	// the alignment fix in emitLineAny.
	if pages[0].Stream == nil {
		t.Error("expected non-nil stream")
	}
}

func TestColumnsWithList(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	cols := NewColumns(2)
	l := NewList(font.Helvetica, 12).AddItem("Item A").AddItem("Item B")
	cols.Add(0, l)
	cols.Add(1, NewParagraph("Right", font.Helvetica, 12))

	r.Add(cols)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestColumnsSpaceBeforeAfter(t *testing.T) {
	cols := NewColumns(2)
	// Left column paragraph with SpaceBefore=10.
	pLeft := NewParagraph("Left", font.Helvetica, 12).SetSpaceBefore(10).SetSpaceAfter(6)
	// Right column paragraph with SpaceBefore=8.
	pRight := NewParagraph("Right", font.Helvetica, 12).SetSpaceBefore(8).SetSpaceAfter(4)

	cols.Add(0, pLeft)
	cols.Add(1, pRight)

	lines := cols.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}

	// The combined row should inherit the max SpaceBefore from the columns.
	if lines[0].SpaceBefore != 10 {
		t.Errorf("expected SpaceBefore=10, got %f", lines[0].SpaceBefore)
	}

	// The last line should inherit the max SpaceAfterV from the columns.
	last := lines[len(lines)-1]
	if last.SpaceAfterV != 6 {
		t.Errorf("expected SpaceAfterV=6, got %f", last.SpaceAfterV)
	}

	// Row height should include spacing contributions.
	// Each line's Height should be at least the content height + spacing.
	if lines[0].Height < 12*1.2+10+6 {
		t.Errorf("expected row height to include spacing, got %f", lines[0].Height)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
