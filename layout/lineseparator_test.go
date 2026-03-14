// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestLineSeparatorBasic(t *testing.T) {
	ls := NewLineSeparator()
	lines := ls.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].separatorRef == nil {
		t.Fatal("expected separatorRef")
	}
	if lines[0].Width != 400 {
		t.Errorf("width = %.1f, want 400", lines[0].Width)
	}
}

func TestLineSeparatorFraction(t *testing.T) {
	ls := NewLineSeparator().SetFraction(0.5).SetAlign(AlignCenter)
	lines := ls.Layout(400)
	if lines[0].Width != 200 {
		t.Errorf("width = %.1f, want 200 (50%% of 400)", lines[0].Width)
	}
	if lines[0].Align != AlignCenter {
		t.Error("expected center alignment")
	}
}

func TestLineSeparatorSpacing(t *testing.T) {
	ls := NewLineSeparator().SetSpaceBefore(10).SetSpaceAfter(5)
	lines := ls.Layout(400)
	if lines[0].SpaceBefore != 10 {
		t.Errorf("SpaceBefore = %.1f, want 10", lines[0].SpaceBefore)
	}
	if lines[0].SpaceAfterV != 5 {
		t.Errorf("SpaceAfterV = %.1f, want 5", lines[0].SpaceAfterV)
	}
}

func TestLineSeparatorRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Before", font.Helvetica, 12))
	r.Add(NewLineSeparator().SetWidth(1).SetColor(ColorRed))
	r.Add(NewParagraph("After", font.Helvetica, 12))

	pages := r.Render()
	content := string(pages[0].Stream.Bytes())
	// Should have the separator's stroke (S operator).
	if !strings.Contains(content, "S") {
		t.Error("expected stroke operator for separator")
	}
	// Should have red stroke color.
	if !strings.Contains(content, "1 0 0 RG") {
		t.Error("expected red stroke color")
	}
}

func TestLineSeparatorDashed(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewLineSeparator().SetStyle(BorderDashed))

	pages := r.Render()
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, " d") {
		t.Error("dashed separator should emit dash pattern operator")
	}
}
