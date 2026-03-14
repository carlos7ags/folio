// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestDivBasic(t *testing.T) {
	d := NewDiv().
		Add(NewParagraph("Hello", font.Helvetica, 12)).
		Add(NewParagraph("World", font.Helvetica, 12))

	lines := d.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("Div should produce 1 synthetic line, got %d", len(lines))
	}
	if lines[0].divRef == nil {
		t.Fatal("expected divRef on line")
	}
	if lines[0].Height <= 0 {
		t.Error("Div should have positive height")
	}
}

func TestDivWithPadding(t *testing.T) {
	d := NewDiv().
		SetPadding(10).
		Add(NewParagraph("Padded content", font.Helvetica, 12))

	lines := d.Layout(400)
	ref := lines[0].divRef

	// Inner width should be reduced by left+right padding.
	expectedInner := 400.0 - 10 - 10
	if ref.innerWidth != expectedInner {
		t.Errorf("innerWidth = %.1f, want %.1f", ref.innerWidth, expectedInner)
	}

	// Total height should include top+bottom padding.
	if ref.totalHeight <= ref.contentHeight {
		t.Error("totalHeight should be > contentHeight due to padding")
	}
}

func TestDivWithBordersAndBackground(t *testing.T) {
	d := NewDiv().
		SetBorder(SolidBorder(1, ColorRed)).
		SetBackground(ColorLightGray).
		SetPadding(8).
		Add(NewParagraph("Boxed content", font.Helvetica, 12))

	lines := d.Layout(400)
	ref := lines[0].divRef

	if ref.div.background == nil {
		t.Error("expected background color")
	}
	if ref.div.borders.Top.Width != 1 {
		t.Error("expected top border width 1")
	}
	if ref.div.borders.Top.Color != ColorRed {
		t.Error("expected red border")
	}
}

func TestDivRendersContent(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	d := NewDiv().
		SetPadding(10).
		SetBorder(DefaultBorder()).
		SetBackground(ColorLightGray).
		Add(NewParagraph("Inside the div", font.Helvetica, 12))

	r.Add(d)
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	content := string(pages[0].Stream.Bytes())
	// Should have background fill (rg), border (RG + stroke), and text (Tj).
	if !strings.Contains(content, "rg") {
		t.Error("expected fill color operator for background")
	}
	if !strings.Contains(content, "Tj") {
		t.Error("expected text operator")
	}
}

func TestDivSpacing(t *testing.T) {
	d := NewDiv().
		SetSpaceBefore(20).
		SetSpaceAfter(15).
		Add(NewParagraph("Spaced", font.Helvetica, 12))

	lines := d.Layout(400)
	if lines[0].SpaceBefore != 20 {
		t.Errorf("SpaceBefore = %.1f, want 20", lines[0].SpaceBefore)
	}
	if lines[0].SpaceAfterV != 15 {
		t.Errorf("SpaceAfterV = %.1f, want 15", lines[0].SpaceAfterV)
	}
}

func TestDivNested(t *testing.T) {
	inner := NewDiv().
		SetPadding(5).
		SetBorder(DashedBorder(0.5, ColorBlue)).
		Add(NewParagraph("Inner content", font.Helvetica, 10))

	outer := NewDiv().
		SetPadding(10).
		SetBorder(SolidBorder(1, ColorBlack)).
		Add(NewParagraph("Outer before", font.Helvetica, 12)).
		Add(inner).
		Add(NewParagraph("Outer after", font.Helvetica, 12))

	lines := outer.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("outer Div should produce 1 line, got %d", len(lines))
	}
	if lines[0].Height <= 0 {
		t.Error("nested Div should have positive height")
	}
}

func TestDivEmpty(t *testing.T) {
	d := NewDiv().SetPadding(10)
	lines := d.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Should still have padding height.
	if lines[0].Height != 20 { // top + bottom padding
		t.Errorf("empty Div height = %.1f, want 20 (padding only)", lines[0].Height)
	}
}

func TestDivNegativePadding(t *testing.T) {
	// Negative padding values should not cause a panic.
	d := NewDiv().
		SetPaddingAll(Padding{Top: -5, Right: -5, Bottom: -5, Left: -5}).
		Add(NewParagraph("Hello", font.Helvetica, 12))

	// Should not panic.
	lines := d.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestDivZeroWidth(t *testing.T) {
	// Layout with zero width should not panic.
	d := NewDiv().
		Add(NewParagraph("Hello", font.Helvetica, 12))

	// Should not panic.
	plan := d.PlanLayout(LayoutArea{Width: 0, Height: 500})
	// Either LayoutFull or LayoutNothing is fine; just don't crash.
	_ = plan
}

func TestDivNegativeMargins(t *testing.T) {
	// SpaceBefore/SpaceAfter can be negative (like negative margins).
	// Should not cause a crash.
	d := NewDiv().
		SetSpaceBefore(-10).
		SetSpaceAfter(-5).
		Add(NewParagraph("Content", font.Helvetica, 12))

	lines := d.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}
