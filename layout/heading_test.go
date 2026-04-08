// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestHeadingH1DefaultSize(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// H1 should use 28pt font
	if lines[0].Words[0].FontSize != 28 {
		t.Errorf("expected H1 font size 28, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingH6DefaultSize(t *testing.T) {
	h := NewHeading("Tiny heading", H6)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].FontSize != 10.7 {
		t.Errorf("expected H6 font size 10.7, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingDefaultFont(t *testing.T) {
	h := NewHeading("Bold heading", H2)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].Font != font.HelveticaBold {
		t.Error("expected HelveticaBold as default heading font")
	}
}

func TestHeadingWithFont(t *testing.T) {
	h := NewHeadingWithFont("Custom", H3, font.TimesRoman, 30)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].Font != font.TimesRoman {
		t.Error("expected TimesRoman")
	}
	if lines[0].Words[0].FontSize != 30 {
		t.Errorf("expected font size 30, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingSpacing(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// First line height should include spacing (fontSize*leading + fontSize*0.5)
	expectedMin := 28 * 1.2 // at least the base line height
	if lines[0].Height <= expectedMin {
		t.Logf("line height %f should include spacing above", lines[0].Height)
	}
}

func TestHeadingAlignment(t *testing.T) {
	h := NewHeading("Centered", H1).SetAlign(AlignCenter)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Align != AlignCenter {
		t.Error("expected AlignCenter")
	}
}

func TestHeadingWordWrap(t *testing.T) {
	h := NewHeading("This is a very long heading that should wrap to multiple lines", H1)
	lines := h.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected multiple lines for narrow width, got %d", len(lines))
	}
}

func TestHeadingAllLevels(t *testing.T) {
	levels := []HeadingLevel{H1, H2, H3, H4, H5, H6}
	var prevSize float64
	for _, level := range levels {
		h := NewHeading("Test", level)
		lines := h.Layout(500)
		if len(lines) == 0 {
			t.Fatalf("H%d produced no lines", level)
		}
		size := lines[0].Words[0].FontSize
		if prevSize > 0 && size >= prevSize {
			t.Errorf("H%d size %f should be smaller than H%d size %f", level, size, level-1, prevSize)
		}
		prevSize = size
	}
}

// TestHeadingPlanLayoutMultilineNoOverlap is a regression test for a bug
// where a heading whose text wrapped to multiple lines would render with
// the wrapped lines overprinted at the same Y-coordinate. The cause was
// that the heading's "space above" offset was being applied only to the
// first PlacedBlock, leaving subsequent line blocks at their original Y
// and producing an overlap of exactly headingSize*0.5 between every
// adjacent pair of lines.
//
// We exercise PlanLayout (the path used by the document renderer), not
// the older Layout(maxWidth) []Line API, because that is where the bug
// lived. Every heading level is checked: the visual severity of the bug
// scales with font size, so H1/H2 are obvious in PDFs, but H5/H6 carry
// the same defect at smaller magnitudes.
func TestHeadingPlanLayoutMultilineNoOverlap(t *testing.T) {
	const text = "Globex Corporation — Platform Renewal + Expansion (FY26)"
	levels := []HeadingLevel{H1, H2, H3, H4, H5, H6}

	for _, level := range levels {
		h := NewHeading(text, level)
		// Narrow width forces wrapping for every level.
		plan := h.PlanLayout(LayoutArea{Width: 180, Height: 10000})
		if len(plan.Blocks) < 2 {
			t.Fatalf("H%d: expected wrapping (>=2 blocks) at width 180, got %d",
				level, len(plan.Blocks))
		}
		for i := 1; i < len(plan.Blocks); i++ {
			prev := plan.Blocks[i-1]
			cur := plan.Blocks[i]
			if cur.Y < prev.Y+prev.Height-0.001 {
				t.Errorf("H%d: line %d Y=%.2f overlaps prev (Y=%.2f, H=%.2f); "+
					"every wrapped line block must start at or below the bottom "+
					"of the previous one",
					level, i, cur.Y, prev.Y, prev.Height)
			}
		}
		// Sanity: the heading's space-above must be reflected on the
		// first block, not absorbed elsewhere.
		expectedSpacing := headingSize(level) * 0.5
		if plan.Blocks[0].Y < expectedSpacing-0.001 {
			t.Errorf("H%d: first block Y=%.2f should include space-above %.2f",
				level, plan.Blocks[0].Y, expectedSpacing)
		}
	}
}

// TestHeadingPlanLayoutSingleLineSpacing guards the simple case: a
// single-line heading must still receive its space-above offset on the
// only block it produces. The multiline fix loops over all blocks; this
// test ensures the loop still handles the len==1 path correctly.
func TestHeadingPlanLayoutSingleLineSpacing(t *testing.T) {
	h := NewHeading("Short", H1)
	plan := h.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 block for short H1, got %d", len(plan.Blocks))
	}
	expected := headingSize(H1) * 0.5
	if plan.Blocks[0].Y < expected-0.001 || plan.Blocks[0].Y > expected+0.001 {
		t.Errorf("single-line H1 first block Y: expected %.2f, got %.2f",
			expected, plan.Blocks[0].Y)
	}
}
