// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// countOps counts occurrences of a PDF operator in a content stream.
// An operator is a standalone token at the end of an operand sequence.
func countOps(stream []byte, op string) int {
	s := string(stream)
	count := 0
	// Simple: count lines ending with the operator or containing " op\n"
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " "+op) || line == op {
			count++
		}
	}
	return count
}

func containsOp(stream []byte, op string) bool {
	return countOps(stream, op) > 0
}

// --- Stage 1: Draw function rendering tests ---

func TestDrawWavyLine(t *testing.T) {
	s := content.NewStream()
	drawWavyLine(s, 10, 50, 100, 1.5)
	b := s.Bytes()

	if len(b) == 0 {
		t.Fatal("wavy line produced empty stream")
	}

	// Wavy line uses zigzag LineTo segments alternating up/down.
	// For a 100pt line with amplitude 1.5 (step=6), expect ~16 segments.
	lines := countOps(b, "l")
	if lines < 10 {
		t.Errorf("expected ≥10 line segments for 100pt wavy line, got %d", lines)
	}

	// Must start with moveto (m) and end with stroke (S).
	if !containsOp(b, "m") {
		t.Error("wavy line missing moveto operator")
	}
	if !containsOp(b, "S") {
		t.Error("wavy line missing stroke operator")
	}

	// Wider line should have more segments.
	s2 := content.NewStream()
	drawWavyLine(s2, 0, 0, 200, 1.5)
	lines2 := countOps(s2.Bytes(), "l")
	if lines2 <= lines {
		t.Errorf("200pt wavy line should have more segments than 100pt: %d vs %d", lines2, lines)
	}
}

func TestDrawBoxShadow(t *testing.T) {
	// Render a Div with box-shadow through the full pipeline.
	d := NewDiv().
		SetBackground(RGB(1, 1, 1)).
		SetPadding(10).
		SetBorder(SolidBorder(1, ColorBlack))
	d.boxShadows = []BoxShadow{{
		OffsetX: 4, OffsetY: 4, Blur: 8, Spread: 0,
		Color: RGB(0, 0, 0),
	}}
	d.Add(NewParagraph("Shadow test", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Box shadow uses save/restore (q/Q) scope.
	saves := countOps(b, "q")
	restores := countOps(b, "Q")
	if saves != restores {
		t.Errorf("unbalanced save/restore: %d q vs %d Q", saves, restores)
	}
	// Shadow involves a fill operation (f).
	if !containsOp(b, "f") {
		t.Error("box shadow should produce fill operator")
	}
}

func TestDrawTextShadow(t *testing.T) {
	// Paragraph with text shadow.
	p := NewStyledParagraph(
		NewRun("Shadow text", font.Helvetica, 12),
	)
	p.runs[0].TextShadow = &TextShadow{
		OffsetX: 2, OffsetY: 2, Blur: 0,
		Color: RGB(0.5, 0.5, 0.5),
	}

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(p)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Text shadow draws text twice: once for shadow, once for actual.
	// Each text draw uses BT/ET (begin/end text).
	textBlocks := countOps(b, "BT")
	if textBlocks < 2 {
		t.Errorf("expected ≥2 text blocks (shadow + actual), got %d", textBlocks)
	}
}

func TestDrawOutline(t *testing.T) {
	d := NewDiv().
		SetBackground(RGB(1, 1, 1)).
		SetPadding(10)
	d.outlineWidth = 2
	d.outlineStyle = "solid"
	d.outlineColor = RGB(1, 0, 0)
	d.outlineOffset = 4
	d.Add(NewParagraph("Outline test", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// Outline draws a rectangle stroke (re + S).
	if !containsOp(b, "re") {
		t.Error("outline should produce rectangle operator")
	}
	if !containsOp(b, "S") {
		t.Error("outline should produce stroke operator")
	}
	// Outline uses RG (stroke color).
	if !containsOp(b, "RG") {
		t.Error("outline should set stroke color (RG)")
	}
}

func TestDrawColumnRules(t *testing.T) {
	cols := NewColumns(3)
	cols.SetGap(20)
	cols.SetColumnRule(ColumnRule{Width: 1, Color: RGB(0.5, 0.5, 0.5), Style: "solid"})
	cols.Add(0, NewParagraph("Col 1 text here", font.Helvetica, 10))
	cols.Add(1, NewParagraph("Col 2 text here", font.Helvetica, 10))
	cols.Add(2, NewParagraph("Col 3 text here", font.Helvetica, 10))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(cols)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	// 3 columns → 2 column rules (vertical lines between columns).
	// Each rule is a moveto + lineto (m + l) pair.
	moveOps := countOps(b, "m")
	lineOps := countOps(b, "l")
	if moveOps < 2 {
		t.Errorf("expected ≥2 moveto operators for 2 column rules, got %d", moveOps)
	}
	if lineOps < 2 {
		t.Errorf("expected ≥2 lineto operators for 2 column rules, got %d", lineOps)
	}
}

func TestDrawSaveRestoreBalance(t *testing.T) {
	// Complex element: Div with background, border, shadow, outline.
	// Verify all q/Q are balanced.
	d := NewDiv().
		SetBackground(RGB(0.9, 0.9, 0.9)).
		SetBorder(SolidBorder(1, ColorBlack)).
		SetOpacity(0.8).
		SetPadding(10)
	d.outlineWidth = 1
	d.outlineStyle = "solid"
	d.outlineColor = ColorBlack
	d.boxShadows = []BoxShadow{{OffsetX: 2, OffsetY: 2, Blur: 4, Color: RGB(0, 0, 0)}}
	d.Add(NewParagraph("Complex", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}

	b := pages[0].Stream.Bytes()
	saves := countOps(b, "q")
	restores := countOps(b, "Q")
	if saves != restores {
		t.Errorf("UNBALANCED save/restore: %d q vs %d Q — this will corrupt the graphics state", saves, restores)
	}
}
