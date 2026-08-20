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

// unitStack models a flex column made of unbreakable units — stacked tables,
// table row groups, fixed-height boxes. It places whole units only: it reports
// LayoutNothing when not even the first unit fits the space left, LayoutPartial
// (with the rest as overflow) when some fit, and LayoutFull when all do. That
// LayoutNothing is the case a real column hits at a page boundary, e.g. a table
// whose header and first body row have to move to the next page as a group.
type unitStack struct {
	units      int
	unitHeight float64
}

func (u *unitStack) PlanLayout(area LayoutArea) LayoutPlan {
	fit := int((area.Height + 0.01) / u.unitHeight)
	if fit <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	if fit > u.units {
		fit = u.units
	}
	blocks := make([]PlacedBlock, 0, fit)
	for i := 0; i < fit; i++ {
		blocks = append(blocks, PlacedBlock{
			Y:      float64(i) * u.unitHeight,
			Width:  area.Width,
			Height: u.unitHeight,
		})
	}
	plan := LayoutPlan{
		Status:   LayoutFull,
		Consumed: float64(fit) * u.unitHeight,
		Blocks:   blocks,
	}
	if fit < u.units {
		plan.Status = LayoutPartial
		plan.Overflow = &unitStack{units: u.units - fit, unitHeight: u.unitHeight}
	}
	return plan
}

// rowWrapper builds the shape that loses content: a flex row used as a row
// wrapper — a short fixed-width marker plus a tall column of unbreakable
// units. align-self start on both items keeps the cross-axis stretch pass out
// of it, matching a row wrapper that pins its items to the top.
func rowWrapper(units int, unitHeight float64) *Flex {
	start := CrossAlignStart
	return NewFlex().
		AddItem(NewFlexItem(&fakeElement{width: 20, height: 12}).SetBasis(20).SetAlignSelf(start)).
		AddItem(NewFlexItem(&unitStack{units: units, unitHeight: unitHeight}).SetBasis(300).SetAlignSelf(start))
}

// TestFlexRowColumnPlacingNothingIsNotDropped is the regression test for the
// content-loss half of flex row fragmentation: when a column reports
// LayoutNothing (it could not place even its first unbreakable unit in the
// space left on the page) it hands back no blocks AND no overflow — its content
// lives on only in the element itself. planRow used to ignore that status, lay
// the line out with the other columns' blocks and report LayoutFull, so the
// whole column was silently dropped: a well-formed PDF, every page complete
// looking, the tail content simply gone (and the document SHORTER, because the
// dropped content never asked for pages of its own).
//
// The line must instead report LayoutNothing so the renderer relocates it to a
// fresh page where the column can make progress.
func TestFlexRowColumnPlacingNothingIsNotDropped(t *testing.T) {
	f := rowWrapper(6, 40)

	// 30pt left on the page — room for the 12pt marker, but not for the
	// column's first 40pt unit.
	plan := f.PlanLayout(LayoutArea{Width: 340, Height: 30})

	if plan.Status != LayoutNothing {
		t.Errorf("status = %d, want LayoutNothing (%d): a line whose column placed nothing must move to the next page, not be laid out with that column dropped",
			plan.Status, LayoutNothing)
	}
	if n := countLeafBlocks(plan.Blocks); n != 0 {
		t.Errorf("plan carries %d leaf block(s) — a deferred line must place nothing here", n)
	}
}

// TestFlexRowAtomicColumnPreservesAllContent walks the whole page chain the way
// the renderer does — including its LayoutNothing handling (relocate to a fresh
// page, and at the top of a page force-place with an unbounded height) — and
// asserts on surviving CONTENT rather than page count. Page count is a
// misleading signal here: dropping the column makes the document shorter, so
// the bug looks like an improvement.
func TestFlexRowAtomicColumnPreservesAllContent(t *testing.T) {
	const (
		pageHeight = 100.0 // usable height per page
		remaining  = 30.0  // space left on the page the row starts on
	)

	wantLeaves := countLeafBlocks(rowWrapper(6, 40).PlanLayout(LayoutArea{Width: 340, Height: 100000}).Blocks)
	if wantLeaves == 0 {
		t.Fatal("unlimited-height layout produced no blocks — fixture is broken")
	}

	var elem Element = rowWrapper(6, 40)
	avail := remaining
	gotLeaves := 0
	atPageTop := false
	for page := 0; page < 100 && elem != nil; page++ {
		plan := elem.PlanLayout(LayoutArea{Width: 340, Height: avail})
		if plan.Status == LayoutNothing {
			if atPageTop {
				// Renderer's page-top fallback: force-place with an
				// effectively unbounded height so pagination cannot loop.
				plan = elem.PlanLayout(LayoutArea{Width: 340, Height: 1e9})
			} else {
				avail, atPageTop = pageHeight, true
				continue
			}
		}
		gotLeaves += countLeafBlocks(plan.Blocks)
		if plan.Status != LayoutPartial {
			elem = nil
			break
		}
		if plan.Overflow == nil {
			t.Fatal("partial layout without overflow — pagination cannot continue")
		}
		elem = plan.Overflow
		avail, atPageTop = pageHeight, true
	}
	if elem != nil {
		t.Fatal("fragmentation did not terminate within 100 pages (possible pagination loop)")
	}
	if gotLeaves != wantLeaves {
		t.Errorf("fragmented layout placed %d leaf blocks, unlimited-height layout places %d — content lost or duplicated",
			gotLeaves, wantLeaves)
	}
}

// TestFlexWrappedLaterLineWithNothingDeferredWhole covers the same fault on a
// later line of a wrapped container: line 1 fits, line 2's column cannot place
// even its first unit. The fitted line must be kept and line 2 deferred whole
// as overflow — never laid out here with its column dropped.
func TestFlexWrappedLaterLineWithNothingDeferredWhole(t *testing.T) {
	start := CrossAlignStart
	f := NewFlex().SetWrap(FlexWrapOn).
		AddItem(NewFlexItem(flexParagraph("Line one")).SetBasis(400).SetAlignSelf(start)).
		AddItem(NewFlexItem(&unitStack{units: 4, unitHeight: 40}).SetBasis(400).SetAlignSelf(start))

	const areaH = 20.0
	plan := f.PlanLayout(LayoutArea{Width: 420, Height: areaH})

	if plan.Status != LayoutPartial {
		t.Fatalf("status = %d, want LayoutPartial: line 1 fits and line 2 must be deferred", plan.Status)
	}
	if plan.Overflow == nil {
		t.Fatal("deferral must carry the un-placed line as overflow")
	}
	if plan.Consumed > areaH+0.01 {
		t.Errorf("fitted portion consumed %.2f, must not exceed the %.1fpt area", plan.Consumed, areaH)
	}
	cont := plan.Overflow.PlanLayout(LayoutArea{Width: 420, Height: 4000})
	if countLeafBlocks(cont.Blocks) == 0 {
		t.Error("overflow produced no content — the deferred line's column was dropped")
	}
}
