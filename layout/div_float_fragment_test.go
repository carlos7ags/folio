// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

// atomicElement is an unsplittable box that honours the available height the
// way ImageElement does: LayoutFull when it fits, LayoutNothing otherwise.
// fakeElement (bookmark_test.go) always reports LayoutFull, which cannot
// exercise the "does not fit" paths below.
type atomicElement struct {
	width, height float64
}

func (a *atomicElement) PlanLayout(area LayoutArea) LayoutPlan {
	if a.height > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: a.height,
		Blocks:   []PlacedBlock{{Width: a.width, Height: a.height}},
	}
}

// floatRecord mirrors the reported markup: a container whose first child is a
// floated 15pt icon, followed by a 21pt header row and a 40pt table.
func floatRecord() *Div {
	return NewDiv().
		Add(NewFloat(FloatLeft, &atomicElement{width: 15, height: 15})).
		Add(&atomicElement{width: 100, height: 21}).
		Add(&atomicElement{width: 100, height: 40})
}

// countFloatBlocks counts the placed blocks carrying float metadata.
func countFloatBlocks(blocks []PlacedBlock) int {
	n := 0
	for i := range blocks {
		if blocks[i].floatInfo != nil {
			n++
		}
		n += countFloatBlocks(blocks[i].Children)
	}
	return n
}

// TestDivFloatThatDoesNotFitMovesWithItsContent is the regression test for a
// float dropped at a page break: with less room than the float's own height,
// the float's plan was LayoutNothing and the float branch skipped it — never
// placed, never carried into the overflow container — so the icon vanished
// while the header and table continued on the next page. The container must
// instead report LayoutNothing so the renderer relocates it whole.
func TestDivFloatThatDoesNotFitMovesWithItsContent(t *testing.T) {
	plan := floatRecord().PlanLayout(LayoutArea{Width: 200, Height: 10})
	if plan.Status != LayoutNothing {
		t.Fatalf("expected LayoutNothing (nothing placed), got status %d", plan.Status)
	}
	if got := countFloatBlocks(plan.Blocks); got != 0 {
		t.Errorf("placed %d float blocks in 10pt, want 0", got)
	}
}

// TestDivFloatNotStrandedWhenFirstInFlowChildMoves covers the room between
// the float's height and the header's: the float fits but the header does
// not. Leaving the float alone at the page bottom strands the icon away from
// the content it was placed beside, and the header was previously placed
// anyway (LayoutFull with Consumed > remaining), overrunning the page.
func TestDivFloatNotStrandedWhenFirstInFlowChildMoves(t *testing.T) {
	plan := floatRecord().PlanLayout(LayoutArea{Width: 200, Height: 18})
	if plan.Status != LayoutNothing {
		t.Fatalf("expected LayoutNothing (float moves with its header), got status %d", plan.Status)
	}
	if got := countFloatBlocks(plan.Blocks); got != 0 {
		t.Errorf("placed %d float blocks in 18pt, want 0", got)
	}
}

// TestDivFloatStaysWithPlacedContent: when in-flow content was placed beside
// the float on this page, the float stays and only the remainder overflows.
func TestDivFloatStaysWithPlacedContent(t *testing.T) {
	plan := floatRecord().PlanLayout(LayoutArea{Width: 200, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial (header fits, table does not), got status %d", plan.Status)
	}
	if got := countFloatBlocks(plan.Blocks); got != 1 {
		t.Errorf("placed %d float blocks, want 1", got)
	}
	if got := countLeafBlocks(plan.Blocks); got != 2 {
		t.Errorf("placed %d leaf blocks, want 2 (icon + header)", got)
	}
	ov, ok := plan.Overflow.(*Div)
	if !ok {
		t.Fatalf("overflow is %T, want *Div", plan.Overflow)
	}
	if n := len(ov.Children()); n != 1 {
		t.Fatalf("overflow carries %d children, want 1 (the table)", n)
	}
	if _, isFloat := ov.Children()[0].(*Float); isFloat {
		t.Error("overflow must not carry the float that was already placed")
	}
}

// TestDivFloatRecordFitsWhole guards the float-aware happy path.
func TestDivFloatRecordFitsWhole(t *testing.T) {
	plan := floatRecord().PlanLayout(LayoutArea{Width: 200, Height: 100})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got status %d", plan.Status)
	}
	if got := countFloatBlocks(plan.Blocks); got != 1 {
		t.Errorf("placed %d float blocks, want 1", got)
	}
	if got := countLeafBlocks(plan.Blocks); got != 3 {
		t.Errorf("placed %d leaf blocks, want 3", got)
	}
}

// TestDivFloatSurvivesRelocation simulates the renderer: the container is
// first offered the sliver left on a page, then re-planned on a fresh page.
// Exactly one float must be drawn across the two attempts.
func TestDivFloatSurvivesRelocation(t *testing.T) {
	for _, sliver := range []float64{10, 18} {
		var elem Element = floatRecord()
		first := elem.PlanLayout(LayoutArea{Width: 200, Height: sliver})
		floats := countFloatBlocks(first.Blocks)
		switch first.Status {
		case LayoutNothing:
			// relocated whole: re-plan the same element
		case LayoutPartial:
			elem = first.Overflow
		default:
			t.Fatalf("sliver %.0f: unexpected status %d", sliver, first.Status)
		}
		second := elem.PlanLayout(LayoutArea{Width: 200, Height: 500})
		if second.Status != LayoutFull {
			t.Fatalf("sliver %.0f: fresh page should hold the record whole, got status %d", sliver, second.Status)
		}
		floats += countFloatBlocks(second.Blocks)
		if floats != 1 {
			t.Errorf("sliver %.0f: %d float blocks drawn across the page chain, want exactly 1", sliver, floats)
		}
		if got := countLeafBlocks(second.Blocks); got != 3 {
			t.Errorf("sliver %.0f: fresh page placed %d leaf blocks, want 3 (icon, header, table)", sliver, got)
		}
	}
}

// TestDivFloatInDefiniteHeightBoxIsNotDeferred: a box with a definite height
// contains (or clips) its content rather than fragmenting it. A float taller
// than the box must be placed overflowing it, as a browser does, not moved —
// with the in-flow content after it — into an overflow fragment, which left
// the box empty and drew its label on the next page.
func TestDivFloatInDefiniteHeightBoxIsNotDeferred(t *testing.T) {
	d := NewDiv().SetHeightUnit(Pt(20)).
		Add(NewFloat(FloatLeft, &atomicElement{width: 15, height: 24})).
		Add(&atomicElement{width: 100, height: 12})
	plan := d.PlanLayout(LayoutArea{Width: 200, Height: 500})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull (definite-height box contains its content), got status %d", plan.Status)
	}
	if got := countFloatBlocks(plan.Blocks); got != 1 {
		t.Errorf("placed %d float blocks, want 1 (overflowing the box)", got)
	}
	if got := countLeafBlocks(plan.Blocks); got != 2 {
		t.Errorf("placed %d leaf blocks, want 2 (icon + label)", got)
	}
}

// splitElement is a stack of fixed-height lines that splits between lines,
// like a paragraph: it places the lines that fit and overflows the rest.
type splitElement struct {
	lines int
	lineH float64
}

func (s *splitElement) PlanLayout(area LayoutArea) LayoutPlan {
	fit := s.lines
	if area.Height > 0 {
		if n := int(area.Height / s.lineH); n < fit {
			fit = n
		}
	}
	if fit == 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	blocks := make([]PlacedBlock, fit)
	for i := range blocks {
		blocks[i] = PlacedBlock{Y: float64(i) * s.lineH, Width: 50, Height: s.lineH}
	}
	plan := LayoutPlan{Status: LayoutFull, Consumed: float64(fit) * s.lineH, Blocks: blocks}
	if fit < s.lines {
		plan.Status = LayoutPartial
		plan.Overflow = &splitElement{lines: s.lines - fit, lineH: s.lineH}
	}
	return plan
}

// TestDivPartialFloatKeepsItsTail: a floated block taller than the space left
// places the lines that fit, and the rest must continue as a float on the next
// page. The container ignored the float's overflow, so when the in-flow
// content beside it fitted the container reported LayoutFull and the float's
// remaining lines were drawn on no page.
func TestDivPartialFloatKeepsItsTail(t *testing.T) {
	const lines = 10
	var elem Element = NewDiv().
		Add(NewFloat(FloatLeft, &splitElement{lines: lines, lineH: 10})).
		Add(&atomicElement{width: 100, height: 10})

	first := elem.PlanLayout(LayoutArea{Width: 200, Height: 30})
	if first.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial (the float's tail continues), got status %d", first.Status)
	}
	ov, ok := first.Overflow.(*Div)
	if !ok || len(ov.Children()) == 0 {
		t.Fatalf("overflow = %T, want a *Div carrying the float's tail", first.Overflow)
	}
	if _, isFloat := ov.Children()[0].(*Float); !isFloat {
		t.Errorf("overflow's first child is %T, want *Float (the tail stays floated)", ov.Children()[0])
	}

	got := countLeafBlocks(first.Blocks)
	elem = first.Overflow
	for page := 0; page < 20; page++ {
		plan := elem.PlanLayout(LayoutArea{Width: 200, Height: 30})
		got += countLeafBlocks(plan.Blocks)
		if plan.Status != LayoutPartial {
			elem = nil
			break
		}
		elem = plan.Overflow
	}
	if elem != nil {
		t.Fatal("fragmentation did not terminate")
	}
	if want := lines + 1; got != want {
		t.Errorf("placed %d leaf blocks across the page chain, want %d (every float line + the in-flow box)", got, want)
	}
}
