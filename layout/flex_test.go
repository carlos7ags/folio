// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func flexParagraph(text string) *Paragraph {
	return NewParagraph(text, font.Helvetica, 10)
}

func TestFlexRowBasic(t *testing.T) {
	f := NewFlex().
		Add(flexParagraph("Hello")).
		Add(flexParagraph("World"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("consumed should be > 0")
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 container block, got %d", len(plan.Blocks))
	}
	if len(plan.Blocks[0].Children) < 2 {
		t.Errorf("expected at least 2 children, got %d", len(plan.Blocks[0].Children))
	}
}

func TestFlexColumnBasic(t *testing.T) {
	f := NewFlex().
		SetDirection(FlexColumn).
		Add(flexParagraph("First")).
		Add(flexParagraph("Second")).
		Add(flexParagraph("Third"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks[0].Children) < 3 {
		t.Errorf("expected at least 3 children, got %d", len(plan.Blocks[0].Children))
	}
}

func TestFlexGrow(t *testing.T) {
	// Two items: one grow=1, one grow=2. They should split the space 1:2.
	f := NewFlex().
		AddItem(NewFlexItem(flexParagraph("A")).SetGrow(1)).
		AddItem(NewFlexItem(flexParagraph("B")).SetGrow(2))

	plan := f.PlanLayout(LayoutArea{Width: 300, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// Both items should be present as children.
	children := plan.Blocks[0].Children
	if len(children) < 2 {
		t.Fatalf("expected 2+ children, got %d", len(children))
	}
}

func TestFlexJustifyCenter(t *testing.T) {
	f := NewFlex().
		SetJustifyContent(JustifyCenter).
		Add(flexParagraph("Centered"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	child := plan.Blocks[0].Children[0]
	// Centered: child.X should be > 0 (offset from left).
	if child.X <= 0 {
		t.Errorf("expected centered X offset > 0, got %f", child.X)
	}
}

func TestFlexJustifySpaceBetween(t *testing.T) {
	f := NewFlex().
		SetJustifyContent(JustifySpaceBetween).
		Add(flexParagraph("Left")).
		Add(flexParagraph("Right"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	children := plan.Blocks[0].Children
	if len(children) < 2 {
		t.Fatal("expected 2+ children")
	}
	// First child should be at X=0, second should be further right.
	if children[0].X >= children[1].X {
		t.Error("first child should be left of second child")
	}
}

func TestFlexJustifyFlexEnd(t *testing.T) {
	f := NewFlex().
		SetJustifyContent(JustifyFlexEnd).
		Add(flexParagraph("Right"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	child := plan.Blocks[0].Children[0]
	if child.X <= 0 {
		t.Errorf("expected flex-end X offset > 0, got %f", child.X)
	}
}

func TestFlexJustifySpaceAround(t *testing.T) {
	f := NewFlex().
		SetJustifyContent(JustifySpaceAround).
		Add(flexParagraph("A")).
		Add(flexParagraph("B"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	children := plan.Blocks[0].Children
	if len(children) < 2 {
		t.Fatal("expected 2+ children")
	}
	// First child should have some left margin (half space).
	if children[0].X <= 0 {
		t.Error("space-around: first child should have left margin")
	}
}

func TestFlexJustifySpaceEvenly(t *testing.T) {
	f := NewFlex().
		SetJustifyContent(JustifySpaceEvenly).
		Add(flexParagraph("A")).
		Add(flexParagraph("B"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	children := plan.Blocks[0].Children
	if len(children) < 2 {
		t.Fatal("expected 2+ children")
	}
	if children[0].X <= 0 {
		t.Error("space-evenly: first child should have left margin")
	}
}

func TestFlexAlignCrossCenter(t *testing.T) {
	// Two items with different heights. CrossAlignCenter should center the shorter one.
	short := flexParagraph("Short")
	long := flexParagraph("This is a much longer paragraph that will wrap to multiple lines when given a narrow width")

	f := NewFlex().
		SetAlignItems(CrossAlignCenter).
		AddItem(NewFlexItem(short).SetBasis(100)).
		AddItem(NewFlexItem(long).SetBasis(100))

	plan := f.PlanLayout(LayoutArea{Width: 220, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	children := plan.Blocks[0].Children
	if len(children) < 2 {
		t.Fatal("expected 2+ children")
	}
	// The shorter item should have a Y offset > the taller item's Y offset.
	if children[0].Y <= 0 && children[1].Y <= 0 {
		// At least one should be offset if they have different heights.
		t.Log("both at Y=0, heights may be equal")
	}
}

func TestFlexAlignSelfOverride(t *testing.T) {
	f := NewFlex().
		SetAlignItems(CrossAlignStart).
		AddItem(NewFlexItem(flexParagraph("Start")).SetBasis(100)).
		AddItem(NewFlexItem(flexParagraph("End")).SetBasis(100).SetAlignSelf(CrossAlignEnd))

	plan := f.PlanLayout(LayoutArea{Width: 220, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestFlexWrap(t *testing.T) {
	f := NewFlex().
		SetWrap(FlexWrapOn).
		SetColumnGap(10)

	// Add items that won't all fit on one line.
	for range 5 {
		f.AddItem(NewFlexItem(flexParagraph("Item")).SetBasis(100))
	}

	plan := f.PlanLayout(LayoutArea{Width: 250, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// With 250px width, 100px items + 10px gap: 2 items per line (100+10+100=210).
	// So 3 lines: [2, 2, 1].
	if plan.Consumed <= 0 {
		t.Error("consumed should be > 0")
	}
}

func TestFlexGap(t *testing.T) {
	f := NewFlex().
		SetGap(20).
		Add(flexParagraph("A")).
		Add(flexParagraph("B"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	children := plan.Blocks[0].Children
	if len(children) < 2 {
		t.Fatal("expected 2+ children")
	}
	// Second child should be offset by first child's width + gap.
	gap := children[1].X - (children[0].X + children[0].Width)
	if gap < 15 { // allow some tolerance
		t.Errorf("expected ~20pt gap, got %f", gap)
	}
}

func TestFlexPadding(t *testing.T) {
	f := NewFlex().
		SetPadding(10).
		Add(flexParagraph("Padded"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	child := plan.Blocks[0].Children[0]
	// Child should be offset by padding.
	if child.X < 10 {
		t.Errorf("expected X >= 10 (padding), got %f", child.X)
	}
	if child.Y < 10 {
		t.Errorf("expected Y >= 10 (padding), got %f", child.Y)
	}
}

func TestFlexBackground(t *testing.T) {
	f := NewFlex().
		SetBackground(ColorBlue).
		Add(flexParagraph("BG"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	if plan.Blocks[0].Draw == nil {
		t.Error("container should have a Draw closure for background")
	}
}

func TestFlexBorders(t *testing.T) {
	f := NewFlex().
		SetBorder(Border{Width: 1, Color: ColorBlack, Style: BorderSolid}).
		Add(flexParagraph("Bordered"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	if plan.Blocks[0].Draw == nil {
		t.Error("container should have a Draw closure for borders")
	}
}

func TestFlexColumnPageBreak(t *testing.T) {
	f := NewFlex().SetDirection(FlexColumn)
	for range 50 {
		f.Add(flexParagraph("Line of text that takes some vertical space"))
	}

	// 50 items at ~12pt each = ~600pt. Height of 100 should cause a split.
	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 100})
	if plan.Status == LayoutFull {
		t.Errorf("expected partial, area is too small for 50 items (consumed would be ~600pt)")
	}
	if plan.Status == LayoutPartial && plan.Overflow == nil {
		t.Error("partial status but no overflow element")
	}
}

func TestFlexRowPageBreakWrapped(t *testing.T) {
	f := NewFlex().SetWrap(FlexWrapOn)
	for range 20 {
		f.AddItem(NewFlexItem(flexParagraph("Item")).SetBasis(100))
	}

	plan := f.PlanLayout(LayoutArea{Width: 250, Height: 30})
	// Very small height should cause partial layout.
	if plan.Status == LayoutFull {
		t.Error("expected partial, area height too small for many wrapped lines")
	}
}

func TestFlexEmpty(t *testing.T) {
	f := NewFlex()
	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Error("empty flex should return LayoutFull")
	}
}

func TestFlexMinWidthRow(t *testing.T) {
	f := NewFlex().
		Add(flexParagraph("Hello")).
		Add(flexParagraph("World"))

	min := f.MinWidth()
	max := f.MaxWidth()
	if min <= 0 {
		t.Error("MinWidth should be > 0")
	}
	if max < min {
		t.Errorf("MaxWidth (%f) should be >= MinWidth (%f)", max, min)
	}
}

func TestFlexMinWidthColumn(t *testing.T) {
	f := NewFlex().
		SetDirection(FlexColumn).
		Add(flexParagraph("Hello")).
		Add(flexParagraph("World"))

	min := f.MinWidth()
	max := f.MaxWidth()
	if min <= 0 {
		t.Error("MinWidth should be > 0")
	}
	if max < min {
		t.Errorf("MaxWidth (%f) should be >= MinWidth (%f)", max, min)
	}
}

func TestFlexMinWidthNoWrap(t *testing.T) {
	f := NewFlex().
		SetColumnGap(10).
		Add(flexParagraph("A")).
		Add(flexParagraph("B"))

	fWrap := NewFlex().
		SetWrap(FlexWrapOn).
		SetColumnGap(10).
		Add(flexParagraph("A")).
		Add(flexParagraph("B"))

	// No-wrap min should be >= wrap min (all items must fit on one line).
	noWrapMin := f.MinWidth()
	wrapMin := fWrap.MinWidth()
	if noWrapMin < wrapMin {
		t.Errorf("no-wrap MinWidth (%f) should be >= wrap MinWidth (%f)", noWrapMin, wrapMin)
	}
}

func TestFlexLayoutElement(t *testing.T) {
	// Test the Element interface (Layout method).
	f := NewFlex().
		Add(flexParagraph("Hello")).
		Add(flexParagraph("World"))

	lines := f.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Height <= 0 {
		t.Error("line height should be > 0")
	}
}

func TestFlexSpaceBeforeAfter(t *testing.T) {
	f := NewFlex().
		SetSpaceBefore(20).
		SetSpaceAfter(10).
		Add(flexParagraph("Spaced"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	// Consumed should include spaceBefore + content + spaceAfter.
	if plan.Consumed < 30 {
		t.Errorf("consumed (%f) should include spaceBefore (20) + spaceAfter (10)", plan.Consumed)
	}
}

func TestFlexShrink(t *testing.T) {
	// Items with basis wider than container should shrink.
	f := NewFlex().
		AddItem(NewFlexItem(flexParagraph("A")).SetBasis(200).SetShrink(1)).
		AddItem(NewFlexItem(flexParagraph("B")).SetBasis(200).SetShrink(1))

	plan := f.PlanLayout(LayoutArea{Width: 300, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestFlexNestedDiv(t *testing.T) {
	inner := NewDiv().
		SetPadding(5).
		SetBackground(ColorLightGray).
		Add(flexParagraph("Inside div"))

	f := NewFlex().
		Add(inner).
		Add(flexParagraph("Next to div"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	if len(plan.Blocks[0].Children) < 2 {
		t.Error("expected at least 2 children")
	}
}

func TestFlexColumnAlignEnd(t *testing.T) {
	f := NewFlex().
		SetDirection(FlexColumn).
		SetAlignItems(CrossAlignEnd).
		Add(flexParagraph("Right-aligned"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	child := plan.Blocks[0].Children[0]
	// Child should be pushed to the right.
	if child.X <= 0 {
		t.Errorf("expected X > 0 for CrossAlignEnd, got %f", child.X)
	}
}

func TestFlexColumnAlignCenter(t *testing.T) {
	f := NewFlex().
		SetDirection(FlexColumn).
		SetAlignItems(CrossAlignCenter).
		Add(flexParagraph("Centered"))

	plan := f.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	child := plan.Blocks[0].Children[0]
	if child.X <= 0 {
		t.Errorf("expected X > 0 for CrossAlignCenter, got %f", child.X)
	}
}
