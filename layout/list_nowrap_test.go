// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestListNoWrapFromRuns pins that an item whose runs all carry NoWrap stays
// on one overflowing line, that MinWidth reports the full item width (there
// is no soft wrap opportunity to shrink to), and that the decision is per
// item: a sibling item without NoWrap runs still wraps.
func TestListNoWrapFromRuns(t *testing.T) {
	const text = "alpha beta gamma delta epsilon"
	wrapping := NewList(font.Helvetica, 12).AddItem(text)
	if n := len(wrapping.Layout(60)); n < 2 {
		t.Fatalf("control: wrapping list should take several lines at 60pt, got %d", n)
	}
	l := NewList(font.Helvetica, 12).AddItemRuns([]TextRun{{
		Text: text, Font: font.Helvetica, FontSize: 12, NoWrap: true,
	}})
	if n := len(l.Layout(60)); n != 1 {
		t.Errorf("item with nowrap runs should stay on one line, got %d", n)
	}
	if l.MinWidth() < l.MaxWidth()-0.01 {
		t.Errorf("nowrap MinWidth %.2f should equal MaxWidth %.2f", l.MinWidth(), l.MaxWidth())
	}
	l.AddItemRuns([]TextRun{{Text: text, Font: font.Helvetica, FontSize: 12}})
	if n := len(l.Layout(60)); n < 3 {
		t.Errorf("wrapping sibling item should wrap next to a nowrap item, got %d lines", n)
	}
}

// TestParagraphMaxWidthForcedBreaks pins that max-content width is the widest
// line between forced breaks (preserved newlines), not the sum of all lines —
// otherwise a nowrap/pre paragraph's MinWidth (= MaxWidth) would be inflated
// by every line of a multi-line pre block.
func TestParagraphMaxWidthForcedBreaks(t *testing.T) {
	single := NewParagraph("alpha beta", font.Helvetica, 12).MaxWidth()
	multi := NewParagraph("alpha beta\nalpha", font.Helvetica, 12)
	if got := multi.MaxWidth(); got < single-0.01 || got > single+0.01 {
		t.Errorf("MaxWidth = %.2f, want widest line %.2f", got, single)
	}
	multi.SetNoWrap(true)
	if got := multi.MinWidth(); got < single-0.01 || got > single+0.01 {
		t.Errorf("nowrap MinWidth = %.2f, want widest line %.2f", got, single)
	}
}
