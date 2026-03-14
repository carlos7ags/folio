// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestParagraphMinWidth(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	minW := p.MinWidth()
	maxW := p.MaxWidth()

	// MinWidth should be the width of the longer word.
	helloW := font.Helvetica.MeasureString("Hello", 12)
	worldW := font.Helvetica.MeasureString("World", 12)
	expected := max(helloW, worldW)

	if math.Abs(minW-expected) > 0.1 {
		t.Errorf("MinWidth = %.2f, want %.2f (longest word)", minW, expected)
	}

	// MaxWidth should be approximately the full line width.
	// Allow some tolerance for inter-run spacing.
	fullW := font.Helvetica.MeasureString("Hello World", 12)

	if math.Abs(maxW-fullW) > 5 {
		t.Errorf("MaxWidth = %.2f, want ~%.2f (full line)", maxW, fullW)
	}

	// MinWidth <= MaxWidth always.
	if minW > maxW {
		t.Errorf("MinWidth (%.2f) > MaxWidth (%.2f)", minW, maxW)
	}
}

func TestParagraphMinWidthLongWord(t *testing.T) {
	p := NewParagraph("Supercalifragilisticexpialidocious short", font.Helvetica, 12)
	minW := p.MinWidth()
	longW := font.Helvetica.MeasureString("Supercalifragilisticexpialidocious", 12)

	if math.Abs(minW-longW) > 0.1 {
		t.Errorf("MinWidth = %.2f, want %.2f (longest word)", minW, longW)
	}
}

func TestHeadingMeasurable(t *testing.T) {
	h := NewHeading("Chapter One", H1)
	minW := h.MinWidth()
	maxW := h.MaxWidth()

	if minW <= 0 {
		t.Error("Heading MinWidth should be positive")
	}
	if maxW < minW {
		t.Errorf("MaxWidth (%.2f) < MinWidth (%.2f)", maxW, minW)
	}
}

func TestImageMeasurable(t *testing.T) {
	// Can't easily create a real image in a unit test, so test the explicit size case.
	ie := &ImageElement{width: 100, height: 50}
	if ie.MinWidth() != 100 {
		t.Errorf("Image MinWidth = %.1f, want 100", ie.MinWidth())
	}
	if ie.MaxWidth() != 100 {
		t.Errorf("Image MaxWidth = %.1f, want 100", ie.MaxWidth())
	}
}

func TestDivMeasurable(t *testing.T) {
	d := NewDiv().
		SetPadding(10).
		Add(NewParagraph("Hello World", font.Helvetica, 12))

	minW := d.MinWidth()
	maxW := d.MaxWidth()

	// Should include padding.
	paraMinW := NewParagraph("Hello World", font.Helvetica, 12).MinWidth()
	expectedMinW := paraMinW + 20 // left + right padding

	if math.Abs(minW-expectedMinW) > 0.1 {
		t.Errorf("Div MinWidth = %.2f, want %.2f", minW, expectedMinW)
	}
	if maxW < minW {
		t.Errorf("Div MaxWidth (%.2f) < MinWidth (%.2f)", maxW, minW)
	}
}

func TestListMeasurable(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("Short").
		AddItem("Longer item text")

	minW := l.MinWidth()
	maxW := l.MaxWidth()

	if minW <= 0 {
		t.Error("List MinWidth should be positive")
	}
	if maxW < minW {
		t.Errorf("List MaxWidth (%.2f) < MinWidth (%.2f)", maxW, minW)
	}
	// Should include indent.
	if minW < l.indent {
		t.Errorf("List MinWidth (%.2f) should be >= indent (%.2f)", minW, l.indent)
	}
}

func TestTableAutoColumnWidths(t *testing.T) {
	tbl := NewTable().SetAutoColumnWidths()

	h := tbl.AddRow()
	h.AddCell("Name", font.HelveticaBold, 10)
	h.AddCell("Description", font.HelveticaBold, 10)
	h.AddCell("$", font.HelveticaBold, 10)

	r1 := tbl.AddRow()
	r1.AddCell("Widget", font.Helvetica, 10)
	r1.AddCell("A very long description that should make this column wider than the others", font.Helvetica, 10)
	r1.AddCell("99", font.Helvetica, 10)

	r2 := tbl.AddRow()
	r2.AddCell("Gadget", font.Helvetica, 10)
	r2.AddCell("Short desc", font.Helvetica, 10)
	r2.AddCell("1234", font.Helvetica, 10)

	lines := tbl.Layout(468)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (rows), got %d", len(lines))
	}

	// The description column should be wider than the price column.
	ref := lines[0].tableRow
	if ref.colWidths[1] <= ref.colWidths[2] {
		t.Errorf("description col (%.1f) should be wider than price col (%.1f)",
			ref.colWidths[1], ref.colWidths[2])
	}
}

func TestTableAutoColumnWidthsNarrowContent(t *testing.T) {
	tbl := NewTable().SetAutoColumnWidths()

	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)
	r.AddCell("C", font.Helvetica, 10)

	lines := tbl.Layout(468)
	ref := lines[0].tableRow

	// When all content is equally narrow, columns should be roughly equal.
	for i := 1; i < len(ref.colWidths); i++ {
		ratio := ref.colWidths[i] / ref.colWidths[0]
		if ratio < 0.5 || ratio > 2.0 {
			t.Errorf("column %d width %.1f is too different from column 0 width %.1f",
				i, ref.colWidths[i], ref.colWidths[0])
		}
	}
}

func TestTableAutoColumnWidthsRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tbl := NewTable().SetAutoColumnWidths()
	row := tbl.AddRow()
	row.AddCell("Short", font.Helvetica, 10)
	row.AddCell(strings.Repeat("Long content ", 10), font.Helvetica, 10)
	r.Add(tbl)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text content")
	}
}

func TestTableAutoColumnWidthsExplicitOverride(t *testing.T) {
	tbl := NewTable().SetAutoColumnWidths()

	// SetAutoColumnWidths, then override with explicit — explicit should win.
	tbl.SetColumnWidths([]float64{100, 200, 168})

	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)
	r.AddCell("C", font.Helvetica, 10)

	lines := tbl.Layout(468)
	ref := lines[0].tableRow

	if ref.colWidths[0] != 100 {
		t.Errorf("col 0 = %.1f, want 100 (explicit override)", ref.colWidths[0])
	}
}
