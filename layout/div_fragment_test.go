// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

// TestDivFragmentsAtomicChildren verifies that a Div whose unsplittable
// (fixed-height) children collectively overflow the available height breaks
// between children and pushes the remainder to overflow, instead of placing
// every child and letting the tail spill past the page to be clipped. A
// LayoutFull child taller than the space left previously fell straight through
// with no fit check, so a container of fixed-height boxes silently lost
// content once its total height crossed a page boundary.
func TestDivFragmentsAtomicChildren(t *testing.T) {
	d := NewDiv()
	for range 5 {
		d.Add(&fakeElement{width: 100, height: 60}) // atomic 60pt boxes
	}

	// 150pt of space fits exactly two 60pt boxes (120pt); the third would
	// cross the boundary and must move to the next page.
	plan := d.PlanLayout(LayoutArea{Width: 200, Height: 150})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial (atomic children overflow the area), got status %d", plan.Status)
	}
	if plan.Overflow == nil {
		t.Fatal("partial layout must carry overflow with the un-placed boxes")
	}
	if got := countLeafBlocks(plan.Blocks); got != 2 {
		t.Errorf("first page placed %d boxes, want 2 (must not overflow the 150pt area)", got)
	}
}

// TestDivFragmentPreservesAllAtomicChildren walks the overflow chain and
// confirms every fixed-height child survives across the pages, with none
// dropped or duplicated.
func TestDivFragmentPreservesAllAtomicChildren(t *testing.T) {
	const n = 7
	build := func() *Div {
		d := NewDiv()
		for range n {
			d.Add(&fakeElement{width: 100, height: 60})
		}
		return d
	}

	var got int
	var elem Element = build()
	for page := 0; page < 100; page++ {
		plan := elem.PlanLayout(LayoutArea{Width: 200, Height: 150})
		got += countLeafBlocks(plan.Blocks)
		if plan.Status != LayoutPartial {
			elem = nil
			break
		}
		if plan.Overflow == nil {
			t.Fatal("partial layout without overflow — pagination cannot continue")
		}
		elem = plan.Overflow
	}
	if elem != nil {
		t.Fatal("fragmentation did not terminate within 100 pages")
	}
	if got != n {
		t.Errorf("placed %d boxes across the page chain, want %d — content lost or duplicated", got, n)
	}
}

// TestDivSingleOversizedChildStillPlaced guards the fittedInFlow > 0 clause:
// a lone child taller than the whole area must still be placed (clipping) so
// pagination cannot loop forever trying to defer a child that never fits.
func TestDivSingleOversizedChildStillPlaced(t *testing.T) {
	d := NewDiv().Add(&fakeElement{width: 100, height: 400})
	plan := d.PlanLayout(LayoutArea{Width: 200, Height: 150})
	if plan.Status != LayoutFull {
		t.Fatalf("a single oversized child must be placed whole (LayoutFull), got status %d", plan.Status)
	}
	if plan.Overflow != nil {
		t.Error("single oversized child must not produce overflow (would loop forever)")
	}
}
