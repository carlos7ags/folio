// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// countLeafBlocks walks a block tree and counts the leaf blocks (those with
// no children) — for a paragraph these are the individual text lines, so the
// count is a proxy for how much content actually survived layout.
func countLeafBlocks(blocks []PlacedBlock) int {
	n := 0
	for i := range blocks {
		if len(blocks[i].Children) == 0 {
			n++
			continue
		}
		n += countLeafBlocks(blocks[i].Children)
	}
	return n
}

// tallParagraph builds a paragraph that wraps into many lines at the given
// width, so it is taller than a single page.
func tallParagraph() *Paragraph {
	return NewParagraph(strings.Repeat("word ", 200), font.Helvetica, 10)
}

// TestFlexRowSingleLineFragments verifies that a single non-wrapping flex
// line taller than the available height is fragmented across the page
// boundary, not laid out whole with the overflow clipped and silently
// dropped. Before the fix planRow returned LayoutFull here (no fittedLineCount
// guard for the first line), so the tail was lost.
func TestFlexRowSingleLineFragments(t *testing.T) {
	f := NewFlex().
		AddItem(NewFlexItem(tallParagraph()).SetBasis(200)).
		AddItem(NewFlexItem(flexParagraph("Short column")).SetBasis(200))

	plan := f.PlanLayout(LayoutArea{Width: 420, Height: 60})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial (tall column must continue on the next page), got status %d", plan.Status)
	}
	if plan.Overflow == nil {
		t.Fatal("partial layout must carry an overflow element with the remaining content")
	}
	if plan.Consumed > 60+0.01 {
		t.Errorf("fitted portion consumed %.2f, must not exceed the 60pt area", plan.Consumed)
	}

	cont := plan.Overflow.PlanLayout(LayoutArea{Width: 420, Height: 4000})
	if countLeafBlocks(cont.Blocks) == 0 {
		t.Error("overflow produced no content — the remaining lines were dropped")
	}
}

// TestFlexRowFragmentPreservesAllContent verifies that fragmenting a flex
// line across pages loses no content: the total number of leaf blocks across
// the page chain equals the number produced when the same flex is laid out
// with unlimited height.
func TestFlexRowFragmentPreservesAllContent(t *testing.T) {
	build := func() *Flex {
		return NewFlex().
			AddItem(NewFlexItem(tallParagraph()).SetBasis(200)).
			AddItem(NewFlexItem(flexParagraph("Short column")).SetBasis(200))
	}

	full := build().PlanLayout(LayoutArea{Width: 420, Height: 100000})
	wantLeaves := countLeafBlocks(full.Blocks)

	var gotLeaves int
	var elem Element = build()
	for page := 0; page < 100; page++ {
		plan := elem.PlanLayout(LayoutArea{Width: 420, Height: 80})
		gotLeaves += countLeafBlocks(plan.Blocks)
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
		t.Fatal("fragmentation did not terminate within 100 pages (possible infinite overflow loop)")
	}
	if gotLeaves != wantLeaves {
		t.Errorf("fragmented layout has %d leaf blocks, unlimited-height layout has %d — content lost or duplicated",
			gotLeaves, wantLeaves)
	}
}

// TestFlexRowAtomicTallLineStillPlaced guards the fallback path: when the
// first/only line is taller than the page but nothing in it can be split, it
// must still be placed (LayoutFull) rather than looping forever trying to
// defer an unsplittable line.
func TestFlexRowAtomicTallLineStillPlaced(t *testing.T) {
	// fakeElement with split=false is atomic — it reports a fixed height and
	// never yields an overflow, so it cannot be fragmented vertically.
	atomic := &fakeElement{width: 300, height: 400}
	f := NewFlex().AddItem(NewFlexItem(atomic).SetBasis(300))

	plan := f.PlanLayout(LayoutArea{Width: 320, Height: 100})
	if plan.Status != LayoutFull {
		t.Fatalf("an unsplittable line taller than the page must be placed whole (LayoutFull), got status %d", plan.Status)
	}
	if plan.Overflow != nil {
		t.Error("unsplittable line must not produce overflow (would loop forever)")
	}
}

// TestFlexWrappedLaterLineDeferredWhole verifies that a wrapped flex line
// which does not fit in the space left is deferred whole to the next page
// rather than fragmented in place — otherwise a splittable later line gets
// left straddling the page bottom, drawing past the margin. Whole-line
// deferral must take precedence over in-place fragmentation for later lines;
// the deferred line fragments on the next page, where it starts at the top.
func TestFlexWrappedLaterLineDeferredWhole(t *testing.T) {
	// Two items whose basis (400 each in a 420 area) forces a wrap into two
	// lines: line 1 = short (fits), line 2 = tall paragraph.
	f := NewFlex().SetWrap(FlexWrapOn).
		AddItem(NewFlexItem(flexParagraph("Short")).SetBasis(400)).
		AddItem(NewFlexItem(tallParagraph()).SetBasis(400))

	const areaH = 16.0
	plan := f.PlanLayout(LayoutArea{Width: 420, Height: areaH})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial (line 2 deferred), got status %d", plan.Status)
	}
	if plan.Consumed > areaH+0.01 {
		t.Errorf("deferred later line straddles the page: consumed %.2f exceeds area %.1f", plan.Consumed, areaH)
	}
	if plan.Overflow == nil {
		t.Fatal("deferral must carry the un-placed line as overflow")
	}
}

// TestFlexConstrainedHeightDoesNotFragment verifies that a flex with a
// definite height — its own explicit height, or one imposed by a parent that
// constrains its cross-axis — contains/clips its overflowing content instead
// of fragmenting it across pages, mirroring the Div fit-guard carve-out.
func TestFlexConstrainedHeightDoesNotFragment(t *testing.T) {
	build := func() *Flex {
		return NewFlex().AddItem(NewFlexItem(tallParagraph()).SetBasis(200))
	}

	// Control: an auto-height flex fragments.
	auto := build().PlanLayout(LayoutArea{Width: 220, Height: 80})
	if auto.Status != LayoutPartial {
		t.Fatalf("auto-height flex should fragment, got status %d", auto.Status)
	}

	// Explicit height: must contain/clip, not fragment.
	fixed := build()
	fixed.ForceHeight(Pt(80))
	fp := fixed.PlanLayout(LayoutArea{Width: 220, Height: 400})
	if fp.Status != LayoutFull || fp.Overflow != nil {
		t.Errorf("explicit-height flex must not fragment, got status %d overflow=%v", fp.Status, fp.Overflow != nil)
	}

	// Definite cross-size imposed by a constraining parent: also must not fragment.
	dc := build().SetDefiniteCrossSize(true)
	dp := dc.PlanLayout(LayoutArea{Width: 220, Height: 80})
	if dp.Status != LayoutFull || dp.Overflow != nil {
		t.Errorf("definite-cross-size flex must not fragment, got status %d overflow=%v", dp.Status, dp.Overflow != nil)
	}
}
