// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

// TestAnchorRecordsNameOnFirstBlock verifies the wrapper tags the first
// placed block with the fragment id so render_plans can surface it as a
// named-destination target.
func TestAnchorRecordsNameOnFirstBlock(t *testing.T) {
	inner := &fakeElement{width: 100, height: 10}
	a := NewAnchor(inner, "section-two")
	plan := a.PlanLayout(LayoutArea{Width: 100, Height: 100})
	if got := plan.Blocks[0].AnchorName; got != "section-two" {
		t.Errorf("AnchorName = %q, want %q", got, "section-two")
	}
}

// TestAnchorOverflowNotRetagged verifies that when the inner element
// splits across pages, the continuation is NOT re-tagged with the anchor
// name — the destination must point at the element's start, once.
func TestAnchorOverflowNotRetagged(t *testing.T) {
	inner := &fakeElement{width: 100, height: 10, split: true}
	a := NewAnchor(inner, "sec")
	plan := a.PlanLayout(LayoutArea{Width: 100, Height: 100})

	if plan.Status != LayoutPartial || plan.Overflow == nil {
		t.Fatalf("setup: expected LayoutPartial with Overflow, got status=%v overflow=%v",
			plan.Status, plan.Overflow)
	}
	if plan.Blocks[0].AnchorName != "sec" {
		t.Errorf("page 1 AnchorName = %q, want %q", plan.Blocks[0].AnchorName, "sec")
	}

	contPlan := plan.Overflow.PlanLayout(LayoutArea{Width: 100, Height: 100})
	if contPlan.Blocks[0].AnchorName != "" {
		t.Errorf("continuation AnchorName = %q, want empty (must not duplicate the destination)",
			contPlan.Blocks[0].AnchorName)
	}
}

// TestAnchorMeasurableConditional verifies the wrapper satisfies
// Measurable iff its inner does, so wrapping does not collapse widths in
// flex / table-cell shrink-to-fit callers.
func TestAnchorMeasurableConditional(t *testing.T) {
	nonMeasurable := &fakeElement{width: 100, height: 10}
	if _, ok := NewAnchor(nonMeasurable, "x").(Measurable); ok {
		t.Error("Anchor over non-Measurable inner should not satisfy Measurable")
	}

	m := &measurableFake{
		fakeElement: fakeElement{width: 100, height: 10},
		min:         42,
		max:         84,
	}
	mw, ok := NewAnchor(m, "x").(Measurable)
	if !ok {
		t.Fatal("Anchor over Measurable inner should satisfy Measurable")
	}
	if got := mw.MinWidth(); got != 42 {
		t.Errorf("MinWidth = %v, want 42 (must delegate to inner)", got)
	}
	if got := mw.MaxWidth(); got != 84 {
		t.Errorf("MaxWidth = %v, want 84 (must delegate to inner)", got)
	}
}

// TestAnchorEmptyPlanPassthrough is a defensive check: an inner element
// that returns no blocks must not panic and must propagate unchanged.
func TestAnchorEmptyPlanPassthrough(t *testing.T) {
	empty := elementFunc(func(LayoutArea) LayoutPlan {
		return LayoutPlan{Status: LayoutNothing}
	})
	plan := NewAnchor(empty, "x").PlanLayout(LayoutArea{Width: 100, Height: 100})
	if len(plan.Blocks) != 0 || plan.Status != LayoutNothing {
		t.Errorf("empty inner plan: got %+v, want empty Blocks with LayoutNothing", plan)
	}
}

// TestAnchorEmptyNameIsNoOp verifies an empty name leaves the first block
// untagged (nothing to register).
func TestAnchorEmptyNameIsNoOp(t *testing.T) {
	inner := &fakeElement{width: 100, height: 10}
	plan := NewAnchor(inner, "").PlanLayout(LayoutArea{Width: 100, Height: 100})
	if plan.Blocks[0].AnchorName != "" {
		t.Errorf("AnchorName = %q, want empty for a no-op wrap", plan.Blocks[0].AnchorName)
	}
}
