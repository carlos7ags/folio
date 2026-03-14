// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestAreaBreakForcesNewPage(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Page one content", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Page two content", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}

	// Both pages should have content.
	p1 := string(pages[0].Stream.Bytes())
	p2 := string(pages[1].Stream.Bytes())
	if p1 == "" {
		t.Error("page 1 should have content")
	}
	if p2 == "" {
		t.Error("page 2 should have content")
	}
}

func TestAreaBreakAtStart(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Should be on page two", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
}

func TestAreaBreakMultiple(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("One", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Two", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Three", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(pages))
	}
}

func TestAreaBreakLayout(t *testing.T) {
	ab := NewAreaBreak()
	lines := ab.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !lines[0].areaBreak {
		t.Error("expected areaBreak flag to be set")
	}
	if lines[0].Height != 0 {
		t.Error("AreaBreak line should have zero height")
	}
}
