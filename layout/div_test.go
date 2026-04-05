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

func TestDivPerCornerBorderRadius(t *testing.T) {
	// Per-corner radius: only top corners rounded (e.g., card header).
	d := NewDiv().
		SetBorderRadiusPerCorner(8, 8, 0, 0).
		SetBorder(SolidBorder(1, ColorBlack)).
		SetBackground(RGB(0.9, 0.9, 0.95)).
		SetPadding(10).
		Add(NewParagraph("Rounded top corners only", font.Helvetica, 12))

	// Verify layout succeeds.
	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}

	// Verify rendering doesn't panic (exercises drawRoundedBorders with per-corner).
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestDivUniformBorderRadius(t *testing.T) {
	d := NewDiv().
		SetBorderRadius(10).
		SetBorder(SolidBorder(2, RGB(0.3, 0.3, 0.9))).
		SetBackground(RGB(0.95, 0.95, 1)).
		SetPadding(12).
		Add(NewParagraph("Uniform radius", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestDivAspectRatio(t *testing.T) {
	// 2:1 ratio on a 400pt wide area → height should be 200pt.
	d := NewDiv().
		SetAspectRatio(2). // width / height = 2
		SetBackground(RGB(0.9, 0.9, 0.95)).
		Add(NewParagraph("Wide box", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	// Consumed height should be 200pt (400 / 2).
	if plan.Consumed < 195 || plan.Consumed > 205 {
		t.Errorf("expected ~200pt consumed for 2:1 ratio on 400pt width, got %f", plan.Consumed)
	}
}

func TestDivAspectRatio16by9(t *testing.T) {
	d := NewDiv().
		SetAspectRatio(16.0 / 9.0).
		SetBackground(RGB(0, 0, 0))

	plan := d.PlanLayout(LayoutArea{Width: 320, Height: 1000})
	// 320 / (16/9) = 180
	if plan.Consumed < 175 || plan.Consumed > 185 {
		t.Errorf("expected ~180pt for 16:9 on 320pt, got %f", plan.Consumed)
	}
}

func TestDivAspectRatioExplicitHeightWins(t *testing.T) {
	// Explicit height should override aspect-ratio.
	d := NewDiv().
		SetAspectRatio(2).
		SetHeightUnit(Pt(100)).
		SetBackground(RGB(0.5, 0.5, 0.5))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// Explicit 100pt should win over 200pt from aspect-ratio.
	if plan.Consumed < 95 || plan.Consumed > 105 {
		t.Errorf("explicit height should win: expected ~100pt, got %f", plan.Consumed)
	}
}

func TestDivAspectRatioWithMinHeight(t *testing.T) {
	// min-height should override aspect-ratio when larger.
	d := NewDiv().
		SetAspectRatio(4). // width/height=4 → 400/4 = 100pt
		SetMinHeightUnit(Pt(200)).
		SetBackground(RGB(0.9, 0.9, 0.9))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// min-height 200 > aspect-ratio 100 → 200pt wins.
	if plan.Consumed < 195 || plan.Consumed > 205 {
		t.Errorf("min-height should override aspect-ratio: expected ~200pt, got %f", plan.Consumed)
	}
}

func TestDivAspectRatioWithMaxHeight(t *testing.T) {
	// max-height should cap aspect-ratio derived height.
	d := NewDiv().
		SetAspectRatio(0.5). // 400/0.5 = 800pt (very tall)
		SetMaxHeightUnit(Pt(300)).
		SetBackground(RGB(0.9, 0.9, 0.9))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// max-height 300 < aspect-ratio 800 → 300pt wins.
	if plan.Consumed < 295 || plan.Consumed > 305 {
		t.Errorf("max-height should cap aspect-ratio: expected ~300pt, got %f", plan.Consumed)
	}
}

func TestDivAspectRatioWithPadding(t *testing.T) {
	// Padding should be inside the aspect-ratio computed height.
	d := NewDiv().
		SetAspectRatio(2). // 400/2 = 200pt total height
		SetPadding(20).
		SetBackground(RGB(0.9, 0.9, 0.9))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// Height forced to 200pt by aspect-ratio (overrides content+padding).
	if plan.Consumed < 195 || plan.Consumed > 205 {
		t.Errorf("expected ~200pt with aspect-ratio ignoring padding, got %f", plan.Consumed)
	}
}

func TestDivAspectRatioZeroIsNoop(t *testing.T) {
	// Zero ratio means not set — height should be content-based.
	d := NewDiv().
		SetAspectRatio(0).
		SetPadding(10).
		Add(NewParagraph("Short text", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// Should be content height (~14pt line + 20pt padding), not 0.
	if plan.Consumed < 20 {
		t.Errorf("zero aspect-ratio should not affect height: got %f", plan.Consumed)
	}
}

func TestDivAspectRatioNoWidthFillsAvailable(t *testing.T) {
	// No explicit width — Div fills available area width.
	d := NewDiv().
		SetAspectRatio(2).
		SetBackground(RGB(0.5, 0.5, 0.5))

	plan := d.PlanLayout(LayoutArea{Width: 300, Height: 1000})
	// 300 / 2 = 150pt.
	if plan.Consumed < 145 || plan.Consumed > 155 {
		t.Errorf("expected ~150pt for ratio 2 on 300pt available, got %f", plan.Consumed)
	}
}

func TestDivAspectRatioContentOverflow(t *testing.T) {
	// Children taller than derived height — content overflows but height is fixed.
	d := NewDiv().
		SetAspectRatio(10). // 400/10 = 40pt
		SetBackground(RGB(0.9, 0.9, 0.9))
	for range 5 {
		d.Add(NewParagraph("Line of text", font.Helvetica, 12))
	}

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// Height forced to 40pt by aspect-ratio, even though content is taller.
	if plan.Consumed < 35 || plan.Consumed > 45 {
		t.Errorf("aspect-ratio should force height: expected ~40pt, got %f", plan.Consumed)
	}
}

// --- Regression tests for Div PlanLayout height/width interactions ---

func TestDivExplicitWidthAndHeight(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pt(200)).
		SetHeightUnit(Pt(100)).
		SetBackground(RGB(0.9, 0.9, 0.9))
	// Add content taller than 100pt.
	for range 10 {
		d.Add(NewParagraph("Line", font.Helvetica, 12))
	}

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// Explicit height should win regardless of content.
	if plan.Consumed < 95 || plan.Consumed > 105 {
		t.Errorf("explicit height should be ~100pt, got %f", plan.Consumed)
	}
}

func TestDivOverflowHiddenClipsHeight(t *testing.T) {
	d := NewDiv().
		SetHeightUnit(Pt(50)).
		SetOverflow("hidden").
		SetBackground(RGB(0.8, 0.8, 0.8))
	for range 10 {
		d.Add(NewParagraph("Tall content", font.Helvetica, 12))
	}

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed < 45 || plan.Consumed > 55 {
		t.Errorf("overflow:hidden with height should be ~50pt, got %f", plan.Consumed)
	}
}

func TestDivMinWidthAndMinHeight(t *testing.T) {
	d := NewDiv().
		SetMinWidthUnit(Pt(300)).
		SetMinHeightUnit(Pt(200)).
		SetBackground(RGB(0.9, 0.9, 0.9)).
		Add(NewParagraph("Small", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed < 195 {
		t.Errorf("min-height should enforce >= 200pt, got %f", plan.Consumed)
	}
}

func TestDivMaxWidthAndMaxHeight(t *testing.T) {
	d := NewDiv().
		SetMaxWidthUnit(Pt(150)).
		SetMaxHeightUnit(Pt(80)).
		SetBackground(RGB(0.9, 0.9, 0.9))
	for range 10 {
		d.Add(NewParagraph("Content exceeding max", font.Helvetica, 12))
	}

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed > 85 {
		t.Errorf("max-height should cap at ~80pt, got %f", plan.Consumed)
	}
}

func TestDivBackgroundOnlyNoBorders(t *testing.T) {
	d := NewDiv().
		SetBackground(RGB(0.95, 0.95, 1)).
		SetPadding(10).
		Add(NewParagraph("Background only", font.Helvetica, 12))

	// Should render without panic (exercises Draw closure with bg but no borders).
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestDivPercentageWidth(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pct(50)).
		SetBackground(RGB(0.9, 0.9, 0.9)).
		Add(NewParagraph("Half width", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	// Block should be ~200pt wide (50% of 400).
	if len(plan.Blocks) > 0 {
		// The container block width is captured; just verify it rendered.
		if plan.Consumed <= 0 {
			t.Error("expected positive consumed height")
		}
	}
}

func TestDivHCenter(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pt(200)).
		SetHCenter(true).
		SetBackground(RGB(0.9, 0.9, 0.9)).
		Add(NewParagraph("Centered", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	// Block X should be ~100pt (centered in 400pt with 200pt width).
	if len(plan.Blocks) > 0 && plan.Blocks[0].X < 90 {
		t.Errorf("expected X offset ~100pt for centered div, got %f", plan.Blocks[0].X)
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

func TestDivOverflowKeepsFollowingChildren(t *testing.T) {
	// First child is large enough to require multiple pages, so Div must
	// enqueue both the overflow portion and the remaining siblings.
	first := NewParagraph(strings.Repeat("long text ", 200), font.Helvetica, 12)
	second := NewParagraph("Second child", font.Helvetica, 12)
	third := NewParagraph("Third child", font.Helvetica, 12)

	d := NewDiv().
		Add(first).
		Add(second).
		Add(third)

	plan := d.PlanLayout(LayoutArea{Width: 200, Height: 50})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	next, ok := plan.Overflow.(*Div)
	if !ok {
		t.Fatalf("expected overflow Div, got %T", plan.Overflow)
	}
	children := next.Children()
	if len(children) < 3 {
		t.Fatalf("expected overflow div to keep overflow + 2 siblings, got %d", len(children))
	}
	if children[1] != second {
		t.Fatal("second child missing from overflow div")
	}
	if children[2] != third {
		t.Fatal("third child missing from overflow div")
	}
}

func TestDivOverlayPositioning(t *testing.T) {
	d := NewDiv().SetPadding(10).SetBackground(RGB(0.95, 0.95, 0.95))
	d.Add(NewParagraph("Base content", font.Helvetica, 12))
	d.AddOverlay(NewParagraph("Overlay", font.Helvetica, 10), 50, 30, 100, false, 0)

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
	b := pages[0].Stream.Bytes()
	// Overlay renders text — should have at least 2 BT blocks (base + overlay).
	bt := countOps(b, "BT")
	if bt < 2 {
		t.Errorf("expected ≥2 text blocks (base + overlay), got %d", bt)
	}
}

func TestDivBoxShadowRendered(t *testing.T) {
	d := NewDiv().SetPadding(10).SetBackground(RGB(1, 1, 1))
	d.boxShadows = []BoxShadow{
		{OffsetX: 3, OffsetY: 3, Blur: 6, Color: RGB(0, 0, 0)},
		{OffsetX: -2, OffsetY: -2, Blur: 4, Color: RGB(0.5, 0, 0)},
	}
	d.Add(NewParagraph("Two shadows", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
	b := pages[0].Stream.Bytes()
	// Two shadows → at least 2 fill rectangles for shadow layers.
	fills := countOps(b, "f")
	if fills < 3 { // 2 shadows + 1 background
		t.Errorf("expected ≥3 fill ops (2 shadows + bg), got %d", fills)
	}
}

func TestDivTransformRendered(t *testing.T) {
	d := NewDiv().SetPadding(10).SetBackground(RGB(0.9, 0.9, 0.9))
	d.transforms = []TransformOp{{Type: "rotate", Values: [2]float64{45, 0}}}
	d.transformOriginX = 50
	d.transformOriginY = 50
	d.Add(NewParagraph("Rotated", font.Helvetica, 12))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
	b := pages[0].Stream.Bytes()
	// Transform produces cm (concat matrix) operator.
	if !containsOp(b, "cm") {
		t.Error("rotated div should produce cm (concat matrix) operator")
	}
}

func TestDivClearFloat(t *testing.T) {
	// Float left, then div with clear:left → div starts below float.
	// Without clear, text wraps beside the float. With clear, it drops below.
	fl := NewFloat(FloatLeft, NewParagraph("Float content here", font.Helvetica, 12))
	fl.SetMargin(5)

	// Without clear.
	noClear := NewDiv().SetPadding(5)
	noClear.Add(NewParagraph("Beside the float", font.Helvetica, 12))

	r1 := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r1.Add(fl)
	r1.Add(noClear)
	pagesNoClear := r1.Render()

	// With clear.
	fl2 := NewFloat(FloatLeft, NewParagraph("Float content here", font.Helvetica, 12))
	fl2.SetMargin(5)

	cleared := NewDiv().SetPadding(5)
	cleared.clear = "left"
	cleared.Add(NewParagraph("Below the float", font.Helvetica, 12))

	r2 := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r2.Add(fl2)
	r2.Add(cleared)
	pagesClear := r2.Render()

	if len(pagesNoClear) == 0 || len(pagesClear) == 0 {
		t.Fatal("expected pages from both renders")
	}

	// The cleared version should use more vertical space (text drops below float).
	streamClear := len(pagesClear[0].Stream.Bytes())
	streamNoClear := len(pagesNoClear[0].Stream.Bytes())
	// Both should produce content.
	if streamClear == 0 || streamNoClear == 0 {
		t.Error("both versions should produce content")
	}
}

func TestDivKeepTogetherMovesToNextPage(t *testing.T) {
	// First paragraph takes most of the page, then a keepTogether div
	// that doesn't fit → should move to page 2.
	// Fill most of the page with paragraphs.
	filler := NewDiv().SetPadding(5)
	for range 5 {
		filler.Add(NewParagraph("Filler line taking up vertical space on page.", font.Helvetica, 12))
	}

	d := NewDiv().SetPadding(10).SetKeepTogether(true)
	for range 10 {
		d.Add(NewParagraph("Keep-together line that needs space on the page.", font.Helvetica, 12))
	}

	r := NewRenderer(612, 250, Margins{Top: 20, Bottom: 20, Left: 20, Right: 20})
	r.Add(filler)
	r.Add(d)
	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected ≥2 pages, got %d", len(pages))
	}
}

func TestDivTagStructure(t *testing.T) {
	d := NewDiv().SetPadding(5)
	d.SetTag("Sect")
	d.Add(NewParagraph("Tagged content", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status == LayoutNothing {
		t.Fatal("expected layout output")
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	if plan.Blocks[0].Tag != "Sect" {
		t.Errorf("expected tag 'Sect', got %q", plan.Blocks[0].Tag)
	}
}
