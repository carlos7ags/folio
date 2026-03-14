// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestTabbedLineBasic(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 300, Align: TabAlignRight},
	).SetSegments("Chapter 1", "15")

	lines := tl.Layout(468)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0].Words) < 2 {
		t.Fatalf("expected at least 2 words, got %d", len(lines[0].Words))
	}
}

func TestTabbedLineLeftAlign(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 200, Align: TabAlignLeft},
	).SetSegments("Label", "Value")

	lines := tl.Layout(468)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// The "Value" word should start at or near position 200.
	// Compute the x position by summing widths + spaces of preceding words.
	x := 0.0
	for i, w := range lines[0].Words {
		if w.Text == "Value" {
			if math.Abs(x-200) > 5 {
				t.Errorf("Value at x=%.1f, expected ~200", x)
			}
			break
		}
		x += w.Width
		if i < len(lines[0].Words)-1 {
			x += w.SpaceAfter
		}
	}
}

func TestTabbedLineRightAlign(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 400, Align: TabAlignRight},
	).SetSegments("Item", "99.95")

	lines := tl.Layout(468)
	// The right-aligned text "99.95" should end at position 400.
	// So its start x = 400 - width("99.95").
	measurer := font.Helvetica
	numWidth := measurer.MeasureString("99.95", 12)

	x := 0.0
	for i, w := range lines[0].Words {
		if w.Text == "99.95" {
			expectedX := 400 - numWidth
			if math.Abs(x-expectedX) > 5 {
				t.Errorf("99.95 at x=%.1f, expected ~%.1f (right-aligned to 400)", x, expectedX)
			}
			break
		}
		x += w.Width
		if i < len(lines[0].Words)-1 {
			x += w.SpaceAfter
		}
	}
}

func TestTabbedLineCenterAlign(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 234, Align: TabAlignCenter},
	).SetSegments("Left", "Center")

	lines := tl.Layout(468)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	measurer := font.Helvetica
	centerWidth := measurer.MeasureString("Center", 12)

	x := 0.0
	for i, w := range lines[0].Words {
		if w.Text == "Center" {
			expectedX := 234 - centerWidth/2
			if math.Abs(x-expectedX) > 5 {
				t.Errorf("Center at x=%.1f, expected ~%.1f", x, expectedX)
			}
			break
		}
		x += w.Width
		if i < len(lines[0].Words)-1 {
			x += w.SpaceAfter
		}
	}
}

func TestTabbedLineWithLeader(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 400, Align: TabAlignRight, Leader: '.'},
	).SetSegments("Chapter 1", "15")

	lines := tl.Layout(468)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Should have words containing dot leaders.
	hasLeader := false
	for _, w := range lines[0].Words {
		if strings.Contains(w.Text, ".") && len(w.Text) > 2 {
			hasLeader = true
			break
		}
	}
	if !hasLeader {
		t.Error("expected dot leader characters between segments")
	}
}

func TestTabbedLineMultipleStops(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 10,
		TabStop{Position: 150, Align: TabAlignLeft},
		TabStop{Position: 350, Align: TabAlignRight},
	).SetSegments("Product", "Widget A", "$48,000")

	lines := tl.Layout(468)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Should have all three segments as words.
	texts := make([]string, 0)
	for _, w := range lines[0].Words {
		texts = append(texts, w.Text)
	}
	joined := strings.Join(texts, " ")
	if !strings.Contains(joined, "Product") {
		t.Error("missing Product")
	}
	if !strings.Contains(joined, "Widget") {
		t.Error("missing Widget")
	}
	if !strings.Contains(joined, "$48,000") {
		t.Error("missing $48,000")
	}
}

func TestTabbedLineRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 400, Align: TabAlignRight, Leader: '.'},
	).SetSegments("Introduction", "1")

	r.Add(tl)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text operators")
	}
}

func TestTabbedLineEmpty(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12)
	lines := tl.Layout(468)
	if len(lines) != 0 {
		t.Errorf("empty TabbedLine should produce 0 lines, got %d", len(lines))
	}
}

func TestTabbedLineSingleSegment(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 300, Align: TabAlignRight},
	).SetSegments("Just text, no tab")

	lines := tl.Layout(468)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestTabbedLineColor(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 300, Align: TabAlignRight},
	).SetSegments("Red text", "Blue text").SetColor(ColorRed)

	lines := tl.Layout(468)
	for _, w := range lines[0].Words {
		if w.Color != ColorRed {
			t.Errorf("word %q color = %+v, want ColorRed", w.Text, w.Color)
		}
	}
}

func TestTabbedLineTOC(t *testing.T) {
	// Typical table of contents usage.
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	entries := []struct{ title, page string }{
		{"Introduction", "1"},
		{"Getting Started", "5"},
		{"Advanced Topics", "23"},
		{"Conclusion", "45"},
	}

	for _, e := range entries {
		tl := NewTabbedLine(font.Helvetica, 12,
			TabStop{Position: 430, Align: TabAlignRight, Leader: '.'},
		).SetSegments(e.title, e.page)
		r.Add(tl)
	}

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Fonts) != 1 {
		t.Errorf("expected 1 font, got %d", len(pages[0].Fonts))
	}
}
